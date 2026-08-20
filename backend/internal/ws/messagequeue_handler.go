package ws

import (
	"context"

	"ChatRoom/internal/messagequeue"
)

var _ messagequeue.Handler = (*Hub)(nil)

// Handle 处理消费者交付的聊天消息，并复用 Hub 现有持久化与投递逻辑。
func (h *Hub) Handle(ctx context.Context, queued messagequeue.ChatMessage) error {
	if err := queued.Validate(); err != nil {
		return err
	}

	message := &Message{
		MsgID:       queued.MsgID,
		Type:        MsgTypeChat,
		FromID:      queued.FromID,
		FromName:    queued.FromName,
		ToID:        queued.ToID,
		ToType:      queued.ToType,
		ContentType: queued.ContentType,
		Content:     queued.Content,
		Timestamp:   queued.Timestamp,
	}

	if message.ToType == ToTypeGroup {
		return h.processGroupMessage(ctx, message)
	}
	return h.processPrivateMessage(ctx, message)
}
