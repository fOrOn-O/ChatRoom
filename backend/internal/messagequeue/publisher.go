package messagequeue

import "context"

// Publisher 将聊天消息提交到具体的消息队列实现。
// 只有消息被队列接受后才返回 nil，并且实现必须响应传入的 Context。
type Publisher interface {
	Publish(ctx context.Context, message ChatMessage) error
}
