package redisstream

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"ChatRoom/internal/messagequeue"

	"github.com/redis/go-redis/v9"
)

func TestPublisherAddsValidatedMessageToConfiguredStream(t *testing.T) {
	writer := &recordingStreamWriter{
		result: redis.NewStringResult("1787241600000-0", nil),
	}
	publisher, err := NewPublisher(writer, "chatroom:stream:chat", 100000)
	if err != nil {
		t.Fatalf("创建 Redis Streams 发布器失败: %v", err)
	}
	want := validChatMessage()

	if err := publisher.Publish(context.Background(), want); err != nil {
		t.Fatalf("发布聊天消息失败: %v", err)
	}
	if writer.calls != 1 {
		t.Fatalf("XADD 调用次数 = %d，期望 1", writer.calls)
	}
	if writer.args.Stream != "chatroom:stream:chat" {
		t.Fatalf("Stream 键 = %q，期望 %q", writer.args.Stream, "chatroom:stream:chat")
	}
	if writer.args.MaxLen != 100000 || !writer.args.Approx {
		t.Fatalf("Stream 长度配置 = MaxLen %d, Approx %t，期望 100000, true", writer.args.MaxLen, writer.args.Approx)
	}

	values, ok := writer.args.Values.(map[string]interface{})
	if !ok {
		t.Fatalf("XADD Values 类型 = %T，期望 map[string]interface{}", writer.args.Values)
	}
	if values["msg_id"] != want.MsgID {
		t.Fatalf("XADD msg_id = %v，期望 %q", values["msg_id"], want.MsgID)
	}
	payload, ok := values["payload"].(string)
	if !ok {
		t.Fatalf("XADD payload 类型 = %T，期望 string", values["payload"])
	}
	got, err := messagequeue.DecodeChatMessage([]byte(payload))
	if err != nil {
		t.Fatalf("解码 XADD payload 失败: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("XADD payload = %+v，期望 %+v", got, want)
	}
}

func TestPublisherRejectsInvalidMessageBeforeWritingRedis(t *testing.T) {
	writer := &recordingStreamWriter{
		result: redis.NewStringResult("1787241600000-0", nil),
	}
	publisher, err := NewPublisher(writer, "chatroom:stream:chat", 100000)
	if err != nil {
		t.Fatalf("创建 Redis Streams 发布器失败: %v", err)
	}
	message := validChatMessage()
	message.MsgID = ""

	err = publisher.Publish(context.Background(), message)
	if !errors.Is(err, messagequeue.ErrInvalidChatMessage) {
		t.Fatalf("发布无效消息时错误 = %v，期望 ErrInvalidChatMessage", err)
	}
	if writer.calls != 0 {
		t.Fatalf("无效消息触发 XADD %d 次，期望 0", writer.calls)
	}
}

func TestPublisherPreservesRedisWriteError(t *testing.T) {
	wantErr := errors.New("redis unavailable")
	writer := &recordingStreamWriter{
		result: redis.NewStringResult("", wantErr),
	}
	publisher, err := NewPublisher(writer, "chatroom:stream:chat", 100000)
	if err != nil {
		t.Fatalf("创建 Redis Streams 发布器失败: %v", err)
	}

	err = publisher.Publish(context.Background(), validChatMessage())
	if !errors.Is(err, wantErr) {
		t.Fatalf("发布错误 = %v，期望保留 Redis 错误 %v", err, wantErr)
	}
}

func TestPublisherStopsBeforeWritingWhenContextIsCanceled(t *testing.T) {
	writer := &recordingStreamWriter{
		result: redis.NewStringResult("1787241600000-0", nil),
	}
	publisher, err := NewPublisher(writer, "chatroom:stream:chat", 100000)
	if err != nil {
		t.Fatalf("创建 Redis Streams 发布器失败: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = publisher.Publish(ctx, validChatMessage())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Context 已取消时错误 = %v，期望 context.Canceled", err)
	}
	if writer.calls != 0 {
		t.Fatalf("Context 已取消时触发 XADD %d 次，期望 0", writer.calls)
	}
}

func TestNewPublisherRejectsInvalidConfiguration(t *testing.T) {
	writer := &recordingStreamWriter{
		result: redis.NewStringResult("1787241600000-0", nil),
	}
	tests := []struct {
		name      string
		writer    StreamWriter
		streamKey string
		maxLength int64
	}{
		{name: "Redis 客户端缺失", writer: nil, streamKey: "chatroom:stream:chat", maxLength: 100000},
		{name: "Stream 键缺失", writer: writer, streamKey: " ", maxLength: 100000},
		{name: "最大长度无效", writer: writer, streamKey: "chatroom:stream:chat", maxLength: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewPublisher(test.writer, test.streamKey, test.maxLength)
			if !errors.Is(err, ErrInvalidPublisherConfig) {
				t.Fatalf("NewPublisher() 错误 = %v，期望 ErrInvalidPublisherConfig", err)
			}
		})
	}
}

type recordingStreamWriter struct {
	args   *redis.XAddArgs
	result *redis.StringCmd
	calls  int
}

func (w *recordingStreamWriter) XAdd(_ context.Context, args *redis.XAddArgs) *redis.StringCmd {
	w.calls++
	w.args = args
	return w.result
}

func validChatMessage() messagequeue.ChatMessage {
	return messagequeue.ChatMessage{
		Version:     messagequeue.ChatMessageVersion,
		MsgID:       "59ffb838-6a50-4d9c-94e7-12cdd75269a1",
		FromID:      7,
		FromName:    "alice",
		ToID:        8,
		ToType:      messagequeue.ToTypeUser,
		ContentType: messagequeue.ContentTypeText,
		Content:     "你好",
		Timestamp:   1787241600,
	}
}
