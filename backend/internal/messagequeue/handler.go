package messagequeue

import "context"

// Handler 处理从消息队列取出的聊天消息。
type Handler interface {
	Handle(ctx context.Context, message ChatMessage) error
}
