package ws

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

var (
	// ErrHubClosed 表示 Hub 已进入关闭状态，不再接受新事件。
	ErrHubClosed = errors.New("websocket hub is closed")
)

type registerRequest struct {
	client *Client
	done   chan struct{}
}

// Hub 是 WebSocket 连接管理与队列消息投递中心。
type Hub struct {
	// ========== 数据库连接 ==========
	db                     *gorm.DB
	authorizeGroupMessage  groupMessageAuthorizer
	resolveGroupRecipients groupRecipientResolver

	// ========== 注册/注销 Channel ==========
	register   chan registerRequest // 客户端注册
	unregister chan *Client         // 客户端注销

	// ========== 状态管理 ==========
	clients map[uint]*Client // 用户ID -> 客户端

	// ========== 并发控制 ==========
	mu sync.RWMutex

	// ========== 生命周期控制 ==========
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// NewHub 创建新的 Hub 实例
func NewHub(db *gorm.DB) *Hub {
	ctx, cancel := context.WithCancel(context.Background())

	return &Hub{
		db:                     db,
		authorizeGroupMessage:  newGroupMessageAuthorizer(db),
		resolveGroupRecipients: newGroupRecipientResolver(db),
		register:               make(chan registerRequest, 256),
		unregister:             make(chan *Client, 256),
		clients:                make(map[uint]*Client),
		ctx:                    ctx,
		cancel:                 cancel,
		done:                   make(chan struct{}),
	}
}

// Run 启动 Hub 的主循环，处理连接生命周期事件。
func (h *Hub) Run() {
	log.Println("Hub 已启动")
	defer log.Println("Hub 已关闭")

	for {
		select {
		// ========== 客户端注册 ==========
		case request := <-h.register:
			h.handleRegister(request.client)
			close(request.done)

		// ========== 客户端注销 ==========
		case client := <-h.unregister:
			h.handleUnregister(client)

		// ========== 优雅关闭 ==========
		case <-h.ctx.Done():
			h.cleanup()
			return
		}
	}
}

// handleRegister 处理客户端注册
func (h *Hub) handleRegister(client *Client) {
	h.mu.Lock()

	old, wasOnline := h.clients[client.UserID]
	h.clients[client.UserID] = client

	onlineCount := len(h.clients)
	h.mu.Unlock()

	if old != nil && old != client {
		log.Printf("用户 %d 建立新连接，关闭旧连接", client.UserID)
		old.CloseWithReason(CloseCodeConnectionReplaced, "connection replaced")
	}

	if !wasOnline {
		log.Printf("用户 %d 上线, 当前在线: %d", client.UserID, onlineCount)
		go h.broadcastOnlineStatus(client.UserID, true)
	}
}

// handleUnregister 处理客户端注销
func (h *Hub) handleUnregister(client *Client) {
	h.disconnectClient(client, websocket.CloseNormalClosure, "")
}

func (h *Hub) disconnectClient(client *Client, code int, reason string) bool {
	h.mu.Lock()
	removed := false
	if current, ok := h.clients[client.UserID]; ok && current == client {
		delete(h.clients, client.UserID)
		removed = true
	}
	onlineCount := len(h.clients)
	h.mu.Unlock()

	client.CloseWithReason(code, reason)

	if removed {
		log.Printf("用户 %d 下线, 当前在线: %d", client.UserID, onlineCount)
		go h.broadcastOnlineStatus(client.UserID, false)
	}

	return removed
}

func (h *Hub) processPrivateMessage(ctx context.Context, msg *Message) error {
	// 保存消息到数据库
	persistenceResult, err := h.saveMessage(ctx, msg)
	if err != nil {
		h.sendToUser(msg.FromID, newMessagePersistenceErrorMessage(msg.MsgID, err))
		return err
	}
	if persistenceResult == messagePersistenceDuplicate {
		h.sendAck(msg.FromID, msg.MsgID, "sent")
		return nil
	}

	h.mu.RLock()
	target, ok := h.clients[msg.ToID]
	h.mu.RUnlock()

	if ok {
		if !target.TrySend(msg) {
			h.disconnectClient(target, websocket.CloseTryAgainLater, "client too slow")
		}
	}

	// ACK 表示消息已经可靠写入历史记录，接收者离线时同样确认。
	h.sendAck(msg.FromID, msg.MsgID, "sent")
	return nil
}

func (h *Hub) processGroupMessage(ctx context.Context, msg *Message) error {
	recipientIDs, err := h.resolveGroupRecipients(ctx, msg.ToID)
	if err != nil {
		h.sendToUser(msg.FromID, newGroupRecipientResolutionErrorMessage(msg.MsgID))
		return err
	}

	// 接收成员解析成功后再保存，避免失败消息进入历史记录。
	persistenceResult, err := h.saveMessage(ctx, msg)
	if err != nil {
		h.sendToUser(msg.FromID, newMessagePersistenceErrorMessage(msg.MsgID, err))
		return err
	}
	if persistenceResult == messagePersistenceDuplicate {
		h.sendAck(msg.FromID, msg.MsgID, "sent")
		return nil
	}

	h.mu.RLock()
	recipients := make([]*Client, 0, len(recipientIDs))
	for _, userID := range recipientIDs {
		if userID == msg.FromID {
			continue
		}
		if client, ok := h.clients[userID]; ok {
			recipients = append(recipients, client)
		}
	}
	h.mu.RUnlock()

	for _, client := range recipients {
		if !client.TrySend(msg) {
			h.disconnectClient(client, websocket.CloseTryAgainLater, "client too slow")
		}
	}

	// 发送确认给发送者
	h.sendAck(msg.FromID, msg.MsgID, "sent")
	return nil
}

func (h *Hub) sendToUser(userID uint, msg *Message) {
	h.mu.RLock()
	client, ok := h.clients[userID]
	h.mu.RUnlock()

	if ok && !client.TrySend(msg) {
		h.disconnectClient(client, websocket.CloseTryAgainLater, "client too slow")
	}
}

// broadcastOnlineStatus 广播用户在线状态
func (h *Hub) broadcastOnlineStatus(userID uint, online bool) {
	// 修复 #5: 使用 Data 字段而非 Raw，确保 JSON 序列化正确
	msg := &Message{
		Type: "online_status",
		Data: map[string]interface{}{
			"user_id": userID,
			"online":  online,
		},
	}

	h.mu.RLock()
	recipients := make([]*Client, 0, len(h.clients))
	for _, client := range h.clients {
		if client.UserID != userID {
			recipients = append(recipients, client)
		}
	}
	h.mu.RUnlock()

	for _, client := range recipients {
		if !client.TrySend(msg) {
			h.disconnectClient(client, websocket.CloseTryAgainLater, "client too slow")
		}
	}
}

// sendAck 发送消息确认
func (h *Hub) sendAck(userID uint, msgID string, status string) {
	h.sendToUser(userID, &Message{
		Type: "chat_ack",
		Data: map[string]interface{}{
			"msg_id": msgID,
			"status": status,
		},
	})
}

// cleanup 清理资源
func (h *Hub) cleanup() {
	h.mu.Lock()
	clients := make([]*Client, 0, len(h.clients))
	for _, client := range h.clients {
		clients = append(clients, client)
	}

	h.clients = make(map[uint]*Client)
	h.mu.Unlock()

	for _, client := range clients {
		client.CloseWithReason(websocket.CloseGoingAway, "server shutting down")
	}

	close(h.done)
}

// Register 注册客户端（供外部调用）
func (h *Hub) Register(client *Client) error {
	select {
	case <-h.ctx.Done():
		return ErrHubClosed
	default:
	}

	request := registerRequest{
		client: client,
		done:   make(chan struct{}),
	}

	select {
	case h.register <- request:
	case <-h.ctx.Done():
		return ErrHubClosed
	}

	select {
	case <-request.done:
		return nil
	case <-h.ctx.Done():
		return ErrHubClosed
	}
}

// Unregister 提交客户端清理请求，并确保 Hub 开始关闭后不会永久阻塞。
func (h *Hub) Unregister(client *Client) {
	select {
	case h.unregister <- client:
	case <-h.ctx.Done():
	}
}

// Shutdown 停止 Hub、关闭所有活跃的 WebSocket 连接，并等待清理完成或上下文超时。
func (h *Hub) Shutdown(ctx context.Context) error {
	h.cancel()
	select {
	case <-h.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GetOnlineCount 获取在线用户数量
func (h *Hub) GetOnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// IsUserOnline 检查用户是否在线
func (h *Hub) IsUserOnline(userID uint) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[userID]
	return ok
}

// GetUserClient 获取用户客户端
func (h *Hub) GetUserClient(userID uint) *Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.clients[userID]
}
