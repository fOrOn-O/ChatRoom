package ws

import (
	"context"
	"errors"
	"fmt"
)

var errMessagePersistenceUnavailable = errors.New("message persistence unavailable")

// saveMessage 在 Hub 确认或投递消息前将消息持久化。
func (h *Hub) saveMessage(ctx context.Context, msg *Message) error {
	if h.db == nil {
		return errMessagePersistenceUnavailable
	}

	result := h.db.WithContext(ctx).Exec(
		"INSERT INTO messages (msg_id, from_user_id, to_id, to_type, content_type, content, created_at) VALUES (?, ?, ?, ?, ?, ?, NOW())",
		msg.MsgID,
		msg.FromID,
		msg.ToID,
		msg.ToType,
		msg.ContentType,
		msg.Content,
	)
	if result.Error != nil {
		return fmt.Errorf("%w: %v", errMessagePersistenceUnavailable, result.Error)
	}

	return nil
}

func newMessagePersistenceErrorMessage(msgID string) *Message {
	return &Message{
		Type: MsgTypeError,
		Data: map[string]interface{}{
			"code":    errorCodeInternal,
			"message": "消息保存失败，请稍后重试",
			"msg_id":  msgID,
		},
	}
}
