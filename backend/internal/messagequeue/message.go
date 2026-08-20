package messagequeue

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	// ChatMessageVersion 是当前聊天消息队列载荷的协议版本。
	ChatMessageVersion = 1

	// ToTypeUser 和 ToTypeGroup 是支持的消息接收类型。
	ToTypeUser  = "user"
	ToTypeGroup = "group"

	// ContentTypeText、ContentTypeImage 和 ContentTypeFile 是支持的消息内容类型。
	ContentTypeText  = "text"
	ContentTypeImage = "image"
	ContentTypeFile  = "file"
)

var (
	// ErrInvalidChatMessage 表示聊天消息不符合队列契约。
	ErrInvalidChatMessage = errors.New("聊天消息无效")
	// ErrUnsupportedChatMessageVersion 表示消费者无法处理该消息协议版本。
	ErrUnsupportedChatMessageVersion = fmt.Errorf("%w: 消息协议版本不受支持", ErrInvalidChatMessage)
)

// ChatMessage 是进入消息队列的聊天消息契约。
type ChatMessage struct {
	Version     int    `json:"version"`
	MsgID       string `json:"msg_id"`
	FromID      uint   `json:"from_id"`
	FromName    string `json:"from_name"`
	ToID        uint   `json:"to_id"`
	ToType      string `json:"to_type"`
	ContentType string `json:"content_type"`
	Content     string `json:"content"`
	Timestamp   int64  `json:"timestamp"`
}

// Validate 检查聊天消息是否满足队列契约。
func (m ChatMessage) Validate() error {
	if m.Version != ChatMessageVersion {
		return fmt.Errorf("%w: %d", ErrUnsupportedChatMessageVersion, m.Version)
	}
	if strings.TrimSpace(m.MsgID) == "" {
		return fmt.Errorf("%w: 消息编号不能为空", ErrInvalidChatMessage)
	}
	if m.FromID == 0 {
		return fmt.Errorf("%w: 发送者编号不能为空", ErrInvalidChatMessage)
	}
	if strings.TrimSpace(m.FromName) == "" {
		return fmt.Errorf("%w: 发送者名称不能为空", ErrInvalidChatMessage)
	}
	if m.ToID == 0 {
		return fmt.Errorf("%w: 接收目标编号不能为空", ErrInvalidChatMessage)
	}
	if m.ToType != ToTypeUser && m.ToType != ToTypeGroup {
		return fmt.Errorf("%w: 不支持的接收类型 %q", ErrInvalidChatMessage, m.ToType)
	}
	if m.ContentType != ContentTypeText && m.ContentType != ContentTypeImage && m.ContentType != ContentTypeFile {
		return fmt.Errorf("%w: 不支持的内容类型 %q", ErrInvalidChatMessage, m.ContentType)
	}
	if m.Content == "" {
		return fmt.Errorf("%w: 消息内容不能为空", ErrInvalidChatMessage)
	}
	if m.Timestamp <= 0 {
		return fmt.Errorf("%w: 消息时间戳必须大于零", ErrInvalidChatMessage)
	}
	return nil
}

// EncodeChatMessage 将聊天消息编码为与具体消息队列无关的 JSON 载荷。
func EncodeChatMessage(message ChatMessage) ([]byte, error) {
	if err := message.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(message)
}

// DecodeChatMessage 从 JSON 载荷中还原聊天消息。
func DecodeChatMessage(payload []byte) (ChatMessage, error) {
	var message ChatMessage
	if err := json.Unmarshal(payload, &message); err != nil {
		return ChatMessage{}, fmt.Errorf("%w: JSON 解析失败: %v", ErrInvalidChatMessage, err)
	}
	if err := message.Validate(); err != nil {
		return ChatMessage{}, err
	}
	return message, nil
}
