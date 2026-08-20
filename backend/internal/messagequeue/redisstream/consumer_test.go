package redisstream

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"ChatRoom/internal/messagequeue"

	"github.com/redis/go-redis/v9"
)

func TestConsumerCreatesGroupProcessesAndAcknowledgesMessage(t *testing.T) {
	want := validChatMessage()
	payload, err := messagequeue.EncodeChatMessage(want)
	if err != nil {
		t.Fatalf("编码测试消息失败: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	client := &recordingConsumerClient{
		groupResult: redis.NewStatusResult("OK", nil),
		readResult: redis.NewXStreamSliceCmdResult([]redis.XStream{{
			Stream: "chatroom:stream:chat",
			Messages: []redis.XMessage{{
				ID: "1787241600000-0",
				Values: map[string]interface{}{
					"msg_id":  want.MsgID,
					"payload": string(payload),
				},
			}},
		}}, nil),
		ackResult: redis.NewIntResult(1, nil),
		onAck:     cancel,
	}
	handler := &recordingHandler{}
	consumer, err := NewConsumer(client, handler, ConsumerOptions{
		StreamKey:    "chatroom:stream:chat",
		Group:        "chatroom-message-workers",
		ConsumerName: "server-1",
		BatchSize:    32,
		BlockTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("创建 Redis Streams 消费者失败: %v", err)
	}

	if err := consumer.Run(ctx); err != nil {
		t.Fatalf("运行 Redis Streams 消费者失败: %v", err)
	}
	if client.groupStream != "chatroom:stream:chat" || client.groupName != "chatroom-message-workers" || client.groupStart != "0" {
		t.Fatalf("消费者组创建参数 = stream %q, group %q, start %q", client.groupStream, client.groupName, client.groupStart)
	}
	if client.readArgs == nil {
		t.Fatal("未调用 XREADGROUP")
	}
	if client.readArgs.Group != "chatroom-message-workers" || client.readArgs.Consumer != "server-1" {
		t.Fatalf("读取参数 = group %q, consumer %q", client.readArgs.Group, client.readArgs.Consumer)
	}
	if !reflect.DeepEqual(client.readArgs.Streams, []string{"chatroom:stream:chat", ">"}) {
		t.Fatalf("读取 Streams = %v，期望 [chatroom:stream:chat >]", client.readArgs.Streams)
	}
	if client.readArgs.Count != 32 || client.readArgs.Block != time.Second || client.readArgs.NoAck {
		t.Fatalf("读取配置 = Count %d, Block %v, NoAck %t", client.readArgs.Count, client.readArgs.Block, client.readArgs.NoAck)
	}
	if len(handler.messages) != 1 || !reflect.DeepEqual(handler.messages[0], want) {
		t.Fatalf("处理消息 = %+v，期望 %+v", handler.messages, want)
	}
	if client.ackStream != "chatroom:stream:chat" || client.ackGroup != "chatroom-message-workers" || !reflect.DeepEqual(client.ackIDs, []string{"1787241600000-0"}) {
		t.Fatalf("确认参数 = stream %q, group %q, ids %v", client.ackStream, client.ackGroup, client.ackIDs)
	}
}

func TestConsumerContinuesWhenGroupAlreadyExists(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &recordingConsumerClient{
		groupResult: redis.NewStatusResult("", errors.New("BUSYGROUP Consumer Group name already exists")),
		readResult:  redis.NewXStreamSliceCmdResult(nil, context.Canceled),
		onRead:      cancel,
	}
	consumer, err := NewConsumer(client, &recordingHandler{}, ConsumerOptions{
		StreamKey:    "chatroom:stream:chat",
		Group:        "chatroom-message-workers",
		ConsumerName: "server-1",
		BatchSize:    32,
		BlockTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("创建 Redis Streams 消费者失败: %v", err)
	}

	if err := consumer.Run(ctx); err != nil {
		t.Fatalf("消费者组已存在时 Run() 错误 = %v", err)
	}
	if client.readArgs == nil {
		t.Fatal("消费者组已存在时未继续调用 XREADGROUP")
	}
}

func TestNewConsumerRejectsInvalidConfiguration(t *testing.T) {
	client := &recordingConsumerClient{}
	handler := &recordingHandler{}
	tests := []struct {
		name    string
		client  ConsumerClient
		handler messagequeue.Handler
		options ConsumerOptions
	}{
		{name: "Redis 客户端缺失", client: nil, handler: handler, options: validConsumerOptions()},
		{name: "消息处理器缺失", client: client, handler: nil, options: validConsumerOptions()},
		{name: "Stream 键缺失", client: client, handler: handler, options: changeConsumerOptions(func(options *ConsumerOptions) { options.StreamKey = " " })},
		{name: "消费者组缺失", client: client, handler: handler, options: changeConsumerOptions(func(options *ConsumerOptions) { options.Group = "" })},
		{name: "消费者名称缺失", client: client, handler: handler, options: changeConsumerOptions(func(options *ConsumerOptions) { options.ConsumerName = "" })},
		{name: "批量读取数量无效", client: client, handler: handler, options: changeConsumerOptions(func(options *ConsumerOptions) { options.BatchSize = 0 })},
		{name: "阻塞读取时间无效", client: client, handler: handler, options: changeConsumerOptions(func(options *ConsumerOptions) { options.BlockTimeout = 0 })},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewConsumer(test.client, test.handler, test.options)
			if !errors.Is(err, ErrInvalidConsumerConfig) {
				t.Fatalf("NewConsumer() 错误 = %v，期望 ErrInvalidConsumerConfig", err)
			}
		})
	}
}

func TestConsumerLeavesInvalidPayloadPending(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &recordingConsumerClient{
		groupResult: redis.NewStatusResult("OK", nil),
		readResult: redis.NewXStreamSliceCmdResult([]redis.XStream{{
			Stream: "chatroom:stream:chat",
			Messages: []redis.XMessage{{
				ID:     "1787241600000-0",
				Values: map[string]interface{}{"payload": "not-json"},
			}},
		}}, nil),
		ackResult: redis.NewIntResult(1, nil),
		onRead:    cancel,
	}
	handler := &recordingHandler{}
	consumer, err := NewConsumer(client, handler, validConsumerOptions())
	if err != nil {
		t.Fatalf("创建 Redis Streams 消费者失败: %v", err)
	}

	if err := consumer.Run(ctx); err != nil {
		t.Fatalf("载荷损坏时 Run() 错误 = %v", err)
	}
	if len(handler.messages) != 0 {
		t.Fatalf("损坏载荷触发处理器 %d 次，期望 0", len(handler.messages))
	}
	if len(client.ackIDs) != 0 {
		t.Fatalf("损坏载荷被确认: %v", client.ackIDs)
	}
}

func TestConsumerLeavesHandlerFailurePending(t *testing.T) {
	want := validChatMessage()
	payload, err := messagequeue.EncodeChatMessage(want)
	if err != nil {
		t.Fatalf("编码测试消息失败: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	client := &recordingConsumerClient{
		groupResult: redis.NewStatusResult("OK", nil),
		readResult: redis.NewXStreamSliceCmdResult([]redis.XStream{{
			Stream: "chatroom:stream:chat",
			Messages: []redis.XMessage{{
				ID:     "1787241600000-0",
				Values: map[string]interface{}{"payload": string(payload)},
			}},
		}}, nil),
		ackResult: redis.NewIntResult(1, nil),
	}
	handler := &recordingHandler{
		err:      errors.New("database unavailable"),
		onHandle: cancel,
	}
	consumer, err := NewConsumer(client, handler, validConsumerOptions())
	if err != nil {
		t.Fatalf("创建 Redis Streams 消费者失败: %v", err)
	}

	if err := consumer.Run(ctx); err != nil {
		t.Fatalf("业务处理失败时 Run() 错误 = %v", err)
	}
	if len(handler.messages) != 1 || !reflect.DeepEqual(handler.messages[0], want) {
		t.Fatalf("处理消息 = %+v，期望 %+v", handler.messages, want)
	}
	if len(client.ackIDs) != 0 {
		t.Fatalf("处理失败消息被确认: %v", client.ackIDs)
	}
}

func TestConsumerReturnsAcknowledgementFailure(t *testing.T) {
	want := validChatMessage()
	payload, err := messagequeue.EncodeChatMessage(want)
	if err != nil {
		t.Fatalf("编码测试消息失败: %v", err)
	}
	wantErr := errors.New("redis unavailable")
	client := &recordingConsumerClient{
		groupResult: redis.NewStatusResult("OK", nil),
		readResult: redis.NewXStreamSliceCmdResult([]redis.XStream{{
			Stream: "chatroom:stream:chat",
			Messages: []redis.XMessage{{
				ID:     "1787241600000-0",
				Values: map[string]interface{}{"payload": string(payload)},
			}},
		}}, nil),
		ackResult: redis.NewIntResult(0, wantErr),
	}
	handler := &recordingHandler{}
	consumer, err := NewConsumer(client, handler, validConsumerOptions())
	if err != nil {
		t.Fatalf("创建 Redis Streams 消费者失败: %v", err)
	}

	err = consumer.Run(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("确认失败时 Run() 错误 = %v，期望保留 %v", err, wantErr)
	}
	if len(handler.messages) != 1 {
		t.Fatalf("确认失败前处理消息 %d 次，期望 1", len(handler.messages))
	}
	if !reflect.DeepEqual(client.ackIDs, []string{"1787241600000-0"}) {
		t.Fatalf("确认消息编号 = %v，期望 [1787241600000-0]", client.ackIDs)
	}
}

func TestConsumerStopsCleanlyWhenAcknowledgementIsCanceled(t *testing.T) {
	want := validChatMessage()
	payload, err := messagequeue.EncodeChatMessage(want)
	if err != nil {
		t.Fatalf("编码测试消息失败: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	client := &recordingConsumerClient{
		groupResult: redis.NewStatusResult("OK", nil),
		readResult: redis.NewXStreamSliceCmdResult([]redis.XStream{{
			Stream: "chatroom:stream:chat",
			Messages: []redis.XMessage{{
				ID:     "1787241600000-0",
				Values: map[string]interface{}{"payload": string(payload)},
			}},
		}}, nil),
		ackResult: redis.NewIntResult(0, context.Canceled),
		onAck:     cancel,
	}
	consumer, err := NewConsumer(client, &recordingHandler{}, validConsumerOptions())
	if err != nil {
		t.Fatalf("创建 Redis Streams 消费者失败: %v", err)
	}

	if err := consumer.Run(ctx); err != nil {
		t.Fatalf("关闭期间确认被取消时 Run() 错误 = %v", err)
	}
}

func TestConsumerReturnsReadFailure(t *testing.T) {
	wantErr := errors.New("redis unavailable")
	client := &recordingConsumerClient{
		groupResult: redis.NewStatusResult("OK", nil),
		readResult:  redis.NewXStreamSliceCmdResult(nil, wantErr),
	}
	consumer, err := NewConsumer(client, &recordingHandler{}, validConsumerOptions())
	if err != nil {
		t.Fatalf("创建 Redis Streams 消费者失败: %v", err)
	}

	err = consumer.Run(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("读取失败时 Run() 错误 = %v，期望保留 %v", err, wantErr)
	}
}

func TestConsumerReturnsGroupCreationFailure(t *testing.T) {
	wantErr := errors.New("permission denied")
	client := &recordingConsumerClient{
		groupResult: redis.NewStatusResult("", wantErr),
	}
	consumer, err := NewConsumer(client, &recordingHandler{}, validConsumerOptions())
	if err != nil {
		t.Fatalf("创建 Redis Streams 消费者失败: %v", err)
	}

	err = consumer.Run(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("建组失败时 Run() 错误 = %v，期望保留 %v", err, wantErr)
	}
	if client.readArgs != nil {
		t.Fatal("消费者组创建失败后仍调用了 XREADGROUP")
	}
}

func validConsumerOptions() ConsumerOptions {
	return ConsumerOptions{
		StreamKey:    "chatroom:stream:chat",
		Group:        "chatroom-message-workers",
		ConsumerName: "server-1",
		BatchSize:    32,
		BlockTimeout: time.Second,
	}
}

func changeConsumerOptions(change func(*ConsumerOptions)) ConsumerOptions {
	options := validConsumerOptions()
	change(&options)
	return options
}

type recordingHandler struct {
	messages []messagequeue.ChatMessage
	err      error
	onHandle func()
}

func (h *recordingHandler) Handle(_ context.Context, message messagequeue.ChatMessage) error {
	h.messages = append(h.messages, message)
	if h.onHandle != nil {
		h.onHandle()
	}
	return h.err
}

type recordingConsumerClient struct {
	groupResult *redis.StatusCmd
	readResult  *redis.XStreamSliceCmd
	ackResult   *redis.IntCmd
	onRead      func()
	onAck       func()

	groupStream string
	groupName   string
	groupStart  string
	readArgs    *redis.XReadGroupArgs
	ackStream   string
	ackGroup    string
	ackIDs      []string
}

func (c *recordingConsumerClient) XGroupCreateMkStream(_ context.Context, stream, group, start string) *redis.StatusCmd {
	c.groupStream = stream
	c.groupName = group
	c.groupStart = start
	return c.groupResult
}

func (c *recordingConsumerClient) XReadGroup(_ context.Context, args *redis.XReadGroupArgs) *redis.XStreamSliceCmd {
	c.readArgs = args
	if c.onRead != nil {
		c.onRead()
	}
	return c.readResult
}

func (c *recordingConsumerClient) XAck(_ context.Context, stream, group string, ids ...string) *redis.IntCmd {
	c.ackStream = stream
	c.ackGroup = group
	c.ackIDs = append([]string(nil), ids...)
	if c.onAck != nil {
		c.onAck()
	}
	return c.ackResult
}
