package ws

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// 心跳相关
	pongWait   = 60 * time.Second    // 等待 Pong 消息的超时时间
	pingPeriod = (pongWait * 9) / 10 // 发送 Ping 的周期
	writeWait  = 10 * time.Second    // 写操作的超时时间

	// 消息限制
	maxMessageSize = 4096 // 最大消息大小
	sendBufferSize = 256  // 发送缓冲区大小

	// CloseCodeConnectionReplaced 表示同一账号的新连接已经接管当前会话。
	CloseCodeConnectionReplaced = 4001
)

// Client 代表一个 WebSocket 客户端连接
type Client struct {
	// 用户信息
	UserID   uint   // 用户ID
	Username string // 用户名
	GroupIDs []uint // 加入的群组ID列表

	// WebSocket 连接
	Conn *websocket.Conn

	// 消息发送通道
	Send chan *Message

	// 生命周期控制
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once

	// 元数据
	ConnectedAt time.Time
	LastPingAt  time.Time
}

// NewClient 创建新的客户端
func NewClient(conn *websocket.Conn, userID uint, username string, groupIDs []uint) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		UserID:      userID,
		Username:    username,
		GroupIDs:    groupIDs,
		Conn:        conn,
		Send:        make(chan *Message, sendBufferSize),
		ctx:         ctx,
		cancel:      cancel,
		ConnectedAt: time.Now(),
		LastPingAt:  time.Now(),
	}
}

// ReadPump 从 WebSocket 连接读取消息
// 运行在独立的 Goroutine 中
func (c *Client) ReadPump(hub *Hub) {
	defer func() {
		c.Close()
		hub.Unregister(c)
	}()

	// 设置读取限制
	c.Conn.SetReadLimit(maxMessageSize)

	// 设置 Pong 处理器
	c.Conn.SetPongHandler(func(string) error {
		c.LastPingAt = time.Now()
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// 设置初始读取超时
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))

	for {
		select {
		case <-c.ctx.Done():
			// Context 被取消，退出
			return
		default:
			_, message, err := c.Conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					log.Printf("WebSocket 读取错误: %v", err)
				}
				return
			}

			// 处理接收到的消息
			c.handleMessage(hub, message)
		}
	}
}

// WritePump 向 WebSocket 连接写入消息
// 运行在独立的 Goroutine 中
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			// 设置写超时
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))

			if !ok {
				// Channel 已关闭，发送关闭消息
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// 修复 #4: 逐条发送消息，不批量拼接
			data, err := json.Marshal(message)
			if err != nil {
				log.Printf("消息序列化失败: %v", err)
				continue
			}

			if err := c.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}

		case <-ticker.C:
			// 发送心跳 Ping
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.ctx.Done():
			// Context 取消，退出
			return
		}
	}
}

// handleMessage 处理接收到的消息
func (c *Client) handleMessage(hub *Hub, data []byte) {
	var msg struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("消息解析失败: %v", err)
		return
	}

	switch msg.Type {
	case "chat":
		c.handleChatMessage(hub, msg.Data)
	default:
		log.Printf("未知消息类型: %s", msg.Type)
	}
}

// handleChatMessage 处理聊天消息
func (c *Client) handleChatMessage(hub *Hub, data json.RawMessage) {
	var chatMsg struct {
		MsgID       string `json:"msg_id"`
		ToID        uint   `json:"to_id"`
		ToType      string `json:"to_type"`      // "user" 或 "group"
		ContentType string `json:"content_type"` // "text", "image", "file"
		Content     string `json:"content"`
	}

	if err := json.Unmarshal(data, &chatMsg); err != nil {
		log.Printf("聊天消息解析失败: %v", err)
		return
	}

	msg := &Message{
		MsgID:       chatMsg.MsgID,
		Type:        "chat",
		FromID:      c.UserID,
		FromName:    c.Username,
		ToID:        chatMsg.ToID,
		ToType:      chatMsg.ToType,
		ContentType: chatMsg.ContentType,
		Content:     chatMsg.Content,
		Timestamp:   time.Now().Unix(),
	}

	switch chatMsg.ToType {
	case "user":
		if err := hub.SendPrivate(msg); err != nil {
			log.Printf("私聊消息进入处理队列失败: msg_id=%s err=%v", msg.MsgID, err)
			c.TrySend(newMessageEnqueueErrorMessage(msg.MsgID))
		}
	case "group":
		if err := hub.authorizeGroupMessage(c.ctx, c.UserID, chatMsg.ToID); err != nil {
			log.Printf("群消息权限校验失败: user_id=%d group_id=%d err=%v", c.UserID, chatMsg.ToID, err)
			c.TrySend(newGroupAuthorizationErrorMessage(chatMsg.MsgID, err))
			return
		}
		if err := hub.SendGroup(msg); err != nil {
			log.Printf("群聊消息进入处理队列失败: msg_id=%s err=%v", msg.MsgID, err)
			c.TrySend(newMessageEnqueueErrorMessage(msg.MsgID))
		}
	default:
		log.Printf("未知的接收类型: %s", chatMsg.ToType)
	}
}

// SendMessage 发送消息给客户端
func (c *Client) SendMessage(msg *Message) bool {
	return c.TrySend(msg)
}

// TrySend 仅在客户端仍处于活动状态且发送缓冲区有容量时将消息入队。
func (c *Client) TrySend(msg *Message) bool {
	select {
	case <-c.ctx.Done():
		return false
	default:
	}

	select {
	case <-c.ctx.Done():
		return false
	case c.Send <- msg:
		return true
	default:
		return false
	}
}

// Close 关闭客户端连接
func (c *Client) Close() {
	c.CloseWithReason(websocket.CloseNormalClosure, "")
}

// CloseWithReason 仅关闭客户端一次，并在条件允许时向对端发送关闭原因。
func (c *Client) CloseWithReason(code int, reason string) {
	c.closeOnce.Do(func() {
		if c.Conn != nil {
			_ = c.Conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(code, reason),
				time.Now().Add(writeWait),
			)
		}
		c.cancel()
		if c.Conn != nil {
			_ = c.Conn.Close()
		}
	})
}
