package ws

import (
	"context"
	"errors"
	"testing"
	"time"

	"ChatRoom/internal/messagequeue"
)

func TestPrivateMessagePublishesWithoutAcknowledgingEarly(t *testing.T) {
	hub := NewHub(nil)
	publisher := &recordingMessagePublisher{messages: make(chan messagequeue.ChatMessage, 1)}
	peer, server := websocketPairForAuthorization(t)
	client := NewClient(server, 7, "alice")
	go client.WritePump()
	go client.ReadPump(hub, publisher, nil)

	const msgID = "private-publish-success"
	if err := peer.WriteJSON(map[string]any{
		"type": MsgTypeChat,
		"data": map[string]any{
			"msg_id":       msgID,
			"to_id":        8,
			"to_type":      ToTypeUser,
			"content_type": ContentTypeText,
			"content":      "publish once",
		},
	}); err != nil {
		t.Fatalf("写入私聊消息失败: %v", err)
	}

	select {
	case published := <-publisher.messages:
		if published.Version != messagequeue.ChatMessageVersion ||
			published.MsgID != msgID || published.FromID != 7 || published.FromName != "alice" ||
			published.ToID != 8 || published.ToType != messagequeue.ToTypeUser ||
			published.ContentType != messagequeue.ContentTypeText || published.Content != "publish once" ||
			published.Timestamp <= 0 {
			t.Fatalf("发布的聊天消息不完整: %+v", published)
		}
	case <-time.After(time.Second):
		t.Fatal("私聊消息未发布到消息队列")
	}

	if err := peer.SetReadDeadline(time.Now().Add(150 * time.Millisecond)); err != nil {
		t.Fatalf("设置读取超时失败: %v", err)
	}
	_, _, err := peer.ReadMessage()
	var netErr interface{ Timeout() bool }
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("消费者处理前不应收到 ACK 或其他消息: %v", err)
	}
}

type recordingMessagePublisher struct {
	messages chan messagequeue.ChatMessage
	err      error
}

type handlingMessagePublisher struct {
	handler messagequeue.Handler
}

func (p handlingMessagePublisher) Publish(ctx context.Context, message messagequeue.ChatMessage) error {
	return p.handler.Handle(ctx, message)
}

func (p *recordingMessagePublisher) Publish(ctx context.Context, message messagequeue.ChatMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if p.err != nil {
		return p.err
	}
	p.messages <- message
	return nil
}
