package ws

import (
	"context"
	"errors"
	"fmt"

	drivermysql "github.com/go-sql-driver/mysql"
)

var (
	errMessagePersistenceUnavailable = errors.New("message persistence unavailable")
	errMessageIDConflict             = errors.New("message ID conflicts with an existing message")
)

const errorCodeInvalidMessage = 1001

type messagePersistenceResult uint8

const (
	messagePersistenceCreated messagePersistenceResult = iota
	messagePersistenceDuplicate
)

type persistedMessage struct {
	FromID      uint
	ToID        uint
	ToType      string
	ContentType string
	Content     string
}

// saveMessage 在 Hub 确认或投递消息前将消息持久化。
func (h *Hub) saveMessage(ctx context.Context, msg *Message) (messagePersistenceResult, error) {
	if h.db == nil {
		return messagePersistenceCreated, errMessagePersistenceUnavailable
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
	if result.Error == nil {
		return messagePersistenceCreated, nil
	}
	if !isDuplicateMessageError(result.Error) {
		return messagePersistenceCreated, fmt.Errorf("%w: %v", errMessagePersistenceUnavailable, result.Error)
	}

	existing, err := h.findPersistedMessage(ctx, msg.MsgID)
	if err != nil {
		return messagePersistenceCreated, fmt.Errorf("%w: %v", errMessagePersistenceUnavailable, err)
	}
	if !existing.matches(msg) {
		return messagePersistenceCreated, errMessageIDConflict
	}

	return messagePersistenceDuplicate, nil
}

func isDuplicateMessageError(err error) bool {
	var mysqlErr *drivermysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

func (h *Hub) findPersistedMessage(ctx context.Context, msgID string) (persistedMessage, error) {
	var existing persistedMessage
	err := h.db.WithContext(ctx).
		Raw(
			"SELECT from_user_id, to_id, to_type, content_type, content FROM messages WHERE msg_id = ? LIMIT 1",
			msgID,
		).
		Row().
		Scan(
			&existing.FromID,
			&existing.ToID,
			&existing.ToType,
			&existing.ContentType,
			&existing.Content,
		)
	return existing, err
}

func (m persistedMessage) matches(msg *Message) bool {
	return m.FromID == msg.FromID &&
		m.ToID == msg.ToID &&
		m.ToType == msg.ToType &&
		m.ContentType == msg.ContentType &&
		m.Content == msg.Content
}

func newMessagePersistenceErrorMessage(msgID string, err error) *Message {
	code := errorCodeInternal
	message := "消息保存失败，请稍后重试"
	if errors.Is(err, errMessageIDConflict) {
		code = errorCodeInvalidMessage
		message = "消息编号与已存在消息冲突，请使用新的消息编号"
	}

	return &Message{
		Type: MsgTypeError,
		Data: map[string]interface{}{
			"code":    code,
			"message": message,
			"msg_id":  msgID,
		},
	}
}
