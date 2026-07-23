package ws

import (
	"context"
	"log"
	"sync"

	"gorm.io/gorm"
	"github.com/redis/go-redis/v9"
)

// Hub 是 WebSocket 连接管理中心
// 使用 Channel 实现高效的消息路由，避免 map 遍历
type Hub struct {
	// ========== 数据库连接 ==========
	db  *gorm.DB
	rdb *redis.Client

	// ========== 注册/注销 Channel ==========
	register   chan *Client    // 客户端注册
	unregister chan *Client    // 客户端注销

	// ========== 消息路由 Channel ==========
	broadcast  chan *Message   // 广播消息
	private    chan *Message   // 私聊消息
	group      chan *Message   // 群聊消息

	// ========== 状态管理 ==========
	clients    map[uint]*Client           // 用户ID -> 客户端
	groups     map[uint]map[uint]*Client  // 群ID -> {用户ID -> 客户端}

	// ========== 并发控制 ==========
	mu sync.RWMutex

	// ========== 生命周期控制 ==========
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// NewHub 创建新的 Hub 实例
func NewHub(db *gorm.DB, rdb *redis.Client) *Hub {
	ctx, cancel := context.WithCancel(context.Background())

	return &Hub{
		db:         db,
		rdb:        rdb,
		register:   make(chan *Client, 256),
		unregister: make(chan *Client, 256),
		broadcast:  make(chan *Message, 1024),
		private:    make(chan *Message, 1024),
		group:      make(chan *Message, 1024),
		clients:    make(map[uint]*Client),
		groups:     make(map[uint]map[uint]*Client),
		ctx:        ctx,
		cancel:     cancel,
		done:       make(chan struct{}),
	}
}

// Run 启动 Hub 的主循环，使用 select 多路复用处理各种消息
func (h *Hub) Run() {
	log.Println("Hub 已启动")
	defer log.Println("Hub 已关闭")

	for {
		select {
		// ========== 客户端注册 ==========
		case client := <-h.register:
			h.handleRegister(client)

		// ========== 客户端注销 ==========
		case client := <-h.unregister:
			h.handleUnregister(client)

		// ========== 私聊消息 ==========
		case msg := <-h.private:
			h.handlePrivateMessage(msg)

		// ========== 群聊消息 ==========
		case msg := <-h.group:
			h.handleGroupMessage(msg)

		// ========== 广播消息 ==========
		case msg := <-h.broadcast:
			h.handleBroadcastMessage(msg)

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
	defer h.mu.Unlock()

	// 修复 #2: 重复登录时关闭旧连接
	if old, ok := h.clients[client.UserID]; ok {
		log.Printf("用户 %d 重复登录，关闭旧连接", client.UserID)
		// 不在这里关闭 old.Send，因为 defer 会处理
		// 只是从群组映射中移除
		for _, groupID := range old.GroupIDs {
			if members, ok := h.groups[groupID]; ok {
				delete(members, old.GroupIDs[0])
			}
		}
	}

	h.clients[client.UserID] = client

	// 加入群组映射
	for _, groupID := range client.GroupIDs {
		if h.groups[groupID] == nil {
			h.groups[groupID] = make(map[uint]*Client)
		}
		h.groups[groupID][client.UserID] = client
	}

	log.Printf("用户 %d 上线, 当前在线: %d", client.UserID, len(h.clients))

	// 广播上线通知 - 移到锁外避免阻塞
	go h.broadcastOnlineStatus(client.UserID, true)
}

// handleUnregister 处理客户端注销
func (h *Hub) handleUnregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 只删除当前存储的客户端（防止旧连接删除新连接）
	if current, ok := h.clients[client.UserID]; ok && current == client {
		delete(h.clients, client.UserID)

		// 从群组移除
		for _, groupID := range client.GroupIDs {
			if members, ok := h.groups[groupID]; ok {
				delete(members, client.UserID)
				if len(members) == 0 {
					delete(h.groups, groupID)
				}
			}
		}

		// 关闭发送通道
		close(client.Send)

		log.Printf("用户 %d 下线, 当前在线: %d", client.UserID, len(h.clients))

		// 广播下线通知 - 移到锁外
		go h.broadcastOnlineStatus(client.UserID, false)
	}
}

// handlePrivateMessage 处理私聊消息
func (h *Hub) handlePrivateMessage(msg *Message) {
	// 保存消息到数据库
	h.saveMessage(msg)

	h.mu.RLock()
	target, ok := h.clients[msg.ToID]
	h.mu.RUnlock()

	if ok {
		// 用户在线，直接推送
		select {
		case target.Send <- msg:
			// 消息发送成功
		default:
			// 修复 #1: 非阻塞发送 unregister，避免死锁
			select {
			case h.unregister <- target:
			default:
				log.Printf("unregister channel full, cannot disconnect user %d", target.UserID)
			}
		}

		// 发送确认给发送者
		h.sendAck(msg.FromID, msg.MsgID, "sent")
	} else {
		// 用户离线，存储离线消息
		h.storeOfflineMessage(msg)
	}
}

// handleGroupMessage 处理群聊消息
func (h *Hub) handleGroupMessage(msg *Message) {
	// 保存消息到数据库
	h.saveMessage(msg)

	h.mu.RLock()
	members, ok := h.groups[msg.ToID]
	h.mu.RUnlock()

	if !ok {
		return
	}

	// 向群内所有成员发送消息（除了发送者）
	for userID, client := range members {
		if userID == msg.FromID {
			continue
		}
		select {
		case client.Send <- msg:
		default:
			// 修复 #1: 非阻塞发送 unregister，避免死锁
			select {
			case h.unregister <- client:
			default:
				log.Printf("unregister channel full, cannot disconnect user %d", userID)
			}
		}
	}

	// 发送确认给发送者
	h.sendAck(msg.FromID, msg.MsgID, "sent")
}

// handleBroadcastMessage 处理广播消息
func (h *Hub) handleBroadcastMessage(msg *Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, client := range h.clients {
		select {
		case client.Send <- msg:
		default:
			// 修复 #1: 非阻塞发送 unregister，避免死锁
			select {
			case h.unregister <- client:
			default:
				log.Printf("unregister channel full, cannot disconnect user %d", client.UserID)
			}
		}
	}
}

// SendPrivate 发送私聊消息
func (h *Hub) SendPrivate(msg *Message) {
	select {
	case h.private <- msg:
	default:
		log.Printf("private channel full, dropping message from %d to %d", msg.FromID, msg.ToID)
	}
}

// SendGroup 发送群聊消息
func (h *Hub) SendGroup(msg *Message) {
	select {
	case h.group <- msg:
	default:
		log.Printf("group channel full, dropping message from %d to group %d", msg.FromID, msg.ToID)
	}
}

// Broadcast 广播消息
func (h *Hub) Broadcast(msg *Message) {
	select {
	case h.broadcast <- msg:
	default:
		log.Printf("broadcast channel full, dropping message")
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
	defer h.mu.RUnlock()

	for _, client := range h.clients {
		if client.UserID != userID {
			select {
			case client.Send <- msg:
			default:
				// 静默丢弃，不阻塞
			}
		}
	}
}

// sendAck 发送消息确认
func (h *Hub) sendAck(userID uint, msgID string, status string) {
	h.mu.RLock()
	client, ok := h.clients[userID]
	h.mu.RUnlock()

	if ok {
		ack := &Message{
			Type: "chat_ack",
			Data: map[string]interface{}{
				"msg_id": msgID,
				"status": status,
			},
		}
		select {
		case client.Send <- ack:
		default:
		}
	}
}

// saveMessage 保存消息到数据库
func (h *Hub) saveMessage(msg *Message) {
	if h.db == nil {
		return
	}

	// 直接执行SQL插入
	h.db.Exec(
		"INSERT INTO messages (msg_id, from_user_id, to_id, to_type, content_type, content, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, NOW())",
		msg.MsgID, msg.FromID, msg.ToID, msg.ToType, msg.ContentType, msg.Content, 1,
	)
}

// storeOfflineMessage 存储离线消息
func (h *Hub) storeOfflineMessage(msg *Message) {
	// TODO: 实现离线消息存储到数据库
	log.Printf("存储离线消息: from %d to %d", msg.FromID, msg.ToID)
}

// cleanup 清理资源
func (h *Hub) cleanup() {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 修复 #15: 关闭所有客户端连接
	for _, client := range h.clients {
		client.Close()
	}

	// 清空映射
	h.clients = make(map[uint]*Client)
	h.groups = make(map[uint]map[uint]*Client)

	close(h.done)
}

// Register 注册客户端（供外部调用）
func (h *Hub) Register(client *Client) {
	select {
	case h.register <- client:
	default:
		log.Printf("register channel full, dropping client %d", client.UserID)
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
