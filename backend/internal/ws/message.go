package ws

import "encoding/json"

// Message 代表 WebSocket 消息
type Message struct {
	// 消息ID
	MsgID string `json:"msg_id,omitempty"`

	// 消息类型
	Type string `json:"type"`

	// 发送者信息
	FromID   uint   `json:"from_id,omitempty"`
	FromName string `json:"from_name,omitempty"`

	// 接收者信息
	ToID   uint   `json:"to_id,omitempty"`
	ToType string `json:"to_type,omitempty"` // "user" 或 "group"

	// 消息内容
	ContentType string `json:"content_type,omitempty"` // "text", "image", "file"
	Content     string `json:"content,omitempty"`

	// 扩展数据
	Data interface{} `json:"data,omitempty"`

	// 原始数据（用于转发）
	Raw json.RawMessage `json:"-"`

	// 时间戳
	Timestamp int64 `json:"timestamp,omitempty"`
}

// 消息类型常量
const (
	MsgTypeChat         = "chat"          // 聊天消息
	MsgTypeChatAck      = "chat_ack"      // 消息确认
	MsgTypeOnlineStatus = "online_status" // 在线状态
	MsgTypeAuth         = "auth"          // 认证
	MsgTypeAuthSuccess  = "auth_success"  // 认证成功
	MsgTypeError        = "error"         // 错误
)

// 内容类型常量
const (
	ContentTypeText  = "text"  // 文本
	ContentTypeImage = "image" // 图片
	ContentTypeFile  = "file"  // 文件
)

// 接收类型常量
const (
	ToTypeUser  = "user"  // 私聊
	ToTypeGroup = "group" // 群聊
)

// Marshal 序列化消息
func (m *Message) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

// UnmarshalMessage 反序列化消息
func UnmarshalMessage(data []byte) (*Message, error) {
	var msg Message
	err := json.Unmarshal(data, &msg)
	return &msg, err
}

// NewChatMessage 创建聊天消息
func NewChatMessage(msgID string, fromID uint, fromName string, toID uint, toType, contentType, content string) *Message {
	return &Message{
		MsgID:       msgID,
		Type:        MsgTypeChat,
		FromID:      fromID,
		FromName:    fromName,
		ToID:        toID,
		ToType:      toType,
		ContentType: contentType,
		Content:     content,
		Timestamp:   0, // 由调用方设置
	}
}

// NewErrorMessage 创建错误消息
func NewErrorMessage(code int, message string) *Message {
	return &Message{
		Type: MsgTypeError,
		Data: map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
}

// NewAuthSuccessMessage 创建认证成功消息
func NewAuthSuccessMessage(userID uint, username string) *Message {
	return &Message{
		Type: MsgTypeAuthSuccess,
		Data: map[string]interface{}{
			"user_id":  userID,
			"username": username,
		},
	}
}
