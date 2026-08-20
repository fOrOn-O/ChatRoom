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
		DeadKey:      "chatroom:stream:chat:dead",
		Group:        "chatroom-message-workers",
		ConsumerName: "server-1",
		BatchSize:    32,
		BlockTimeout: time.Second,
		ClaimIdle:    30 * time.Second,
		MaxRetries:   5,
		MaxLength:    100000,
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

func TestConsumerClaimsPendingMessageAndAcknowledgesAfterSuccess(t *testing.T) {
	want := validChatMessage()
	payload, err := messagequeue.EncodeChatMessage(want)
	if err != nil {
		t.Fatalf("编码测试消息失败: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	client := &recordingConsumerClient{
		groupResult: redis.NewStatusResult("OK", nil),
		autoClaimMessages: []redis.XMessage{{
			ID:     "1787241600000-0",
			Values: map[string]interface{}{"payload": string(payload)},
		}},
		autoClaimStart: "0-0",
		pendingResult: []redis.XPendingExt{{
			ID:         "1787241600000-0",
			Consumer:   "server-1",
			RetryCount: 2,
		}},
		ackResult:  redis.NewIntResult(1, nil),
		readResult: redis.NewXStreamSliceCmdResult(nil, context.Canceled),
		onAck:      cancel,
	}
	handler := &recordingHandler{}
	consumer, err := NewConsumer(client, handler, validConsumerOptions())
	if err != nil {
		t.Fatalf("创建 Redis Streams 消费者失败: %v", err)
	}

	if err := consumer.Run(ctx); err != nil {
		t.Fatalf("处理 Pending 消息时 Run() 错误 = %v", err)
	}
	if client.autoClaimArgs == nil {
		t.Fatal("未调用 XAUTOCLAIM 接管 Pending 消息")
	}
	if len(handler.messages) != 1 || !reflect.DeepEqual(handler.messages[0], want) {
		t.Fatalf("处理消息 = %+v，期望 %+v", handler.messages, want)
	}
	if !reflect.DeepEqual(client.ackIDs, []string{"1787241600000-0"}) {
		t.Fatalf("确认消息编号 = %v，期望 [1787241600000-0]", client.ackIDs)
	}
}

func TestConsumerLeavesClaimedFailurePendingBeforeRetryLimit(t *testing.T) {
	want := validChatMessage()
	payload, err := messagequeue.EncodeChatMessage(want)
	if err != nil {
		t.Fatalf("编码测试消息失败: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	client := &recordingConsumerClient{
		groupResult: redis.NewStatusResult("OK", nil),
		autoClaimMessages: []redis.XMessage{{
			ID:     "1787241600000-0",
			Values: map[string]interface{}{"payload": string(payload)},
		}},
		autoClaimStart: "0-0",
		pendingResult: []redis.XPendingExt{{
			ID:         "1787241600000-0",
			Consumer:   "server-1",
			RetryCount: 2,
		}},
		ackResult:  redis.NewIntResult(1, nil),
		readResult: redis.NewXStreamSliceCmdResult(nil, context.Canceled),
		onPending:  cancel,
	}
	handler := &recordingHandler{
		err: errors.New("database unavailable"),
	}
	consumer, err := NewConsumer(client, handler, validConsumerOptions())
	if err != nil {
		t.Fatalf("创建 Redis Streams 消费者失败: %v", err)
	}

	if err := consumer.Run(ctx); err != nil {
		t.Fatalf("重试次数未达上限时 Run() 错误 = %v", err)
	}
	if client.pendingArgs == nil {
		t.Fatal("未查询 Pending 消息的投递次数")
	}
	if client.pendingArgs.Start != "1787241600000-0" || client.pendingArgs.End != "1787241600000-0" || client.pendingArgs.Count != 1 {
		t.Fatalf("Pending 查询参数 = %+v", client.pendingArgs)
	}
	if len(client.ackIDs) != 0 {
		t.Fatalf("未达重试上限的失败消息被确认: %v", client.ackIDs)
	}
}

func TestConsumerMovesExhaustedPendingMessageToDeadStream(t *testing.T) {
	want := validChatMessage()
	payload, err := messagequeue.EncodeChatMessage(want)
	if err != nil {
		t.Fatalf("编码测试消息失败: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	client := &recordingConsumerClient{
		groupResult: redis.NewStatusResult("OK", nil),
		autoClaimMessages: []redis.XMessage{{
			ID: "1787241600000-0",
			Values: map[string]interface{}{
				"msg_id":  want.MsgID,
				"payload": string(payload),
			},
		}},
		autoClaimStart: "0-0",
		pendingResult: []redis.XPendingExt{{
			ID:         "1787241600000-0",
			Consumer:   "server-1",
			RetryCount: 6,
		}},
		addID:      "1787241900000-0",
		ackResult:  redis.NewIntResult(1, nil),
		readResult: redis.NewXStreamSliceCmdResult(nil, context.Canceled),
		onAck:      cancel,
	}
	handler := &recordingHandler{err: errors.New("database unavailable")}
	consumer, err := NewConsumer(client, handler, validConsumerOptions())
	if err != nil {
		t.Fatalf("创建 Redis Streams 消费者失败: %v", err)
	}

	if err := consumer.Run(ctx); err != nil {
		t.Fatalf("写入死信时 Run() 错误 = %v", err)
	}
	if client.addArgs == nil {
		t.Fatal("达到重试上限后未写入死信 Stream")
	}
	if client.addArgs.Stream != "chatroom:stream:chat:dead" || client.addArgs.MaxLen != 100000 || !client.addArgs.Approx {
		t.Fatalf("死信写入参数 = %+v", client.addArgs)
	}
	values, ok := client.addArgs.Values.(map[string]interface{})
	if !ok {
		t.Fatalf("死信字段类型 = %T，期望 map[string]interface{}", client.addArgs.Values)
	}
	if values["source_stream"] != "chatroom:stream:chat" || values["source_id"] != "1787241600000-0" || values["delivery_count"] != int64(6) || values["retry_count"] != int64(5) {
		t.Fatalf("死信来源字段 = %+v", values)
	}
	if values["msg_id"] != want.MsgID || values["payload"] != string(payload) {
		t.Fatalf("死信未保留原始消息字段: %+v", values)
	}
	if !reflect.DeepEqual(client.ackIDs, []string{"1787241600000-0"}) {
		t.Fatalf("死信写入成功后确认消息编号 = %v", client.ackIDs)
	}
}

func TestConsumerDoesNotAcknowledgeWhenDeadStreamWriteFails(t *testing.T) {
	want := validChatMessage()
	payload, err := messagequeue.EncodeChatMessage(want)
	if err != nil {
		t.Fatalf("编码测试消息失败: %v", err)
	}
	wantErr := errors.New("redis unavailable")
	client := &recordingConsumerClient{
		groupResult: redis.NewStatusResult("OK", nil),
		autoClaimMessages: []redis.XMessage{{
			ID:     "1787241600000-0",
			Values: map[string]interface{}{"payload": string(payload)},
		}},
		autoClaimStart: "0-0",
		pendingResult: []redis.XPendingExt{{
			ID:         "1787241600000-0",
			Consumer:   "server-1",
			RetryCount: 6,
		}},
		addErr:    wantErr,
		ackResult: redis.NewIntResult(1, nil),
	}
	consumer, err := NewConsumer(client, &recordingHandler{err: errors.New("database unavailable")}, validConsumerOptions())
	if err != nil {
		t.Fatalf("创建 Redis Streams 消费者失败: %v", err)
	}

	err = consumer.Run(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("死信写入失败时 Run() 错误 = %v，期望保留 %v", err, wantErr)
	}
	if len(client.ackIDs) != 0 {
		t.Fatalf("死信写入失败后原消息被确认: %v", client.ackIDs)
	}
}

func TestConsumerMovesExhaustedInvalidPayloadToDeadStream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &recordingConsumerClient{
		groupResult: redis.NewStatusResult("OK", nil),
		autoClaimMessages: []redis.XMessage{{
			ID:     "1787241600000-0",
			Values: map[string]interface{}{"payload": "not-json"},
		}},
		autoClaimStart: "0-0",
		pendingResult: []redis.XPendingExt{{
			ID:         "1787241600000-0",
			Consumer:   "server-1",
			RetryCount: 6,
		}},
		addID:      "1787241900000-0",
		ackResult:  redis.NewIntResult(1, nil),
		readResult: redis.NewXStreamSliceCmdResult(nil, context.Canceled),
		onAck:      cancel,
	}
	handler := &recordingHandler{}
	consumer, err := NewConsumer(client, handler, validConsumerOptions())
	if err != nil {
		t.Fatalf("创建 Redis Streams 消费者失败: %v", err)
	}

	if err := consumer.Run(ctx); err != nil {
		t.Fatalf("无效载荷转入死信时 Run() 错误 = %v", err)
	}
	if len(handler.messages) != 0 {
		t.Fatalf("无效载荷触发处理器 %d 次，期望 0", len(handler.messages))
	}
	if client.addArgs == nil {
		t.Fatal("达到重试上限的无效载荷未写入死信 Stream")
	}
	if !reflect.DeepEqual(client.ackIDs, []string{"1787241600000-0"}) {
		t.Fatalf("无效载荷写入死信后的确认记录 = %v", client.ackIDs)
	}
}

func TestConsumerClaimsPendingMessagesPeriodically(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	client := &recordingConsumerClient{
		groupResult:    redis.NewStatusResult("OK", nil),
		autoClaimStart: "0-0",
		readResult:     redis.NewXStreamSliceCmdResult(nil, redis.Nil),
	}
	options := validConsumerOptions()
	options.ClaimIdle = 5 * time.Millisecond
	consumer, err := NewConsumer(client, &recordingHandler{}, options)
	if err != nil {
		t.Fatalf("创建 Redis Streams 消费者失败: %v", err)
	}

	if err := consumer.Run(ctx); err != nil {
		t.Fatalf("周期接管 Pending 消息时 Run() 错误 = %v", err)
	}
	if client.autoClaimCalls < 2 {
		t.Fatalf("XAUTOCLAIM 调用次数 = %d，期望至少 2", client.autoClaimCalls)
	}
}

func TestConsumerContinuesClaimingFromReturnedCursor(t *testing.T) {
	want := validChatMessage()
	payload, err := messagequeue.EncodeChatMessage(want)
	if err != nil {
		t.Fatalf("编码测试消息失败: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	client := &recordingConsumerClient{
		groupResult: redis.NewStatusResult("OK", nil),
		autoClaimResults: []autoClaimResult{
			{
				messages: []redis.XMessage{{ID: "1787241600000-0", Values: map[string]interface{}{"payload": string(payload)}}},
				next:     "1787241600000-1",
			},
			{
				messages: []redis.XMessage{{ID: "1787241600000-1", Values: map[string]interface{}{"payload": string(payload)}}},
				next:     "0-0",
			},
		},
		ackResult:  redis.NewIntResult(1, nil),
		readResult: redis.NewXStreamSliceCmdResult(nil, context.Canceled),
	}
	client.onAck = func() {
		if len(client.ackCalls) == 2 {
			cancel()
		}
	}
	consumer, err := NewConsumer(client, &recordingHandler{}, validConsumerOptions())
	if err != nil {
		t.Fatalf("创建 Redis Streams 消费者失败: %v", err)
	}

	if err := consumer.Run(ctx); err != nil {
		t.Fatalf("分页接管 Pending 消息时 Run() 错误 = %v", err)
	}
	if len(client.autoClaimArgsHistory) != 2 {
		t.Fatalf("XAUTOCLAIM 调用次数 = %d，期望 2", len(client.autoClaimArgsHistory))
	}
	if client.autoClaimArgsHistory[0].Start != "0-0" || client.autoClaimArgsHistory[1].Start != "1787241600000-1" {
		t.Fatalf("XAUTOCLAIM 游标 = [%s, %s]", client.autoClaimArgsHistory[0].Start, client.autoClaimArgsHistory[1].Start)
	}
	if !reflect.DeepEqual(client.ackCalls, [][]string{{"1787241600000-0"}, {"1787241600000-1"}}) {
		t.Fatalf("确认记录 = %v", client.ackCalls)
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
		DeadKey:      "chatroom:stream:chat:dead",
		Group:        "chatroom-message-workers",
		ConsumerName: "server-1",
		BatchSize:    32,
		BlockTimeout: time.Second,
		ClaimIdle:    30 * time.Second,
		MaxRetries:   5,
		MaxLength:    100000,
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
		{name: "死信 Stream 键缺失", client: client, handler: handler, options: changeConsumerOptions(func(options *ConsumerOptions) { options.DeadKey = " " })},
		{name: "消费者组缺失", client: client, handler: handler, options: changeConsumerOptions(func(options *ConsumerOptions) { options.Group = "" })},
		{name: "消费者名称缺失", client: client, handler: handler, options: changeConsumerOptions(func(options *ConsumerOptions) { options.ConsumerName = "" })},
		{name: "批量读取数量无效", client: client, handler: handler, options: changeConsumerOptions(func(options *ConsumerOptions) { options.BatchSize = 0 })},
		{name: "阻塞读取时间无效", client: client, handler: handler, options: changeConsumerOptions(func(options *ConsumerOptions) { options.BlockTimeout = 0 })},
		{name: "Pending 接管时间无效", client: client, handler: handler, options: changeConsumerOptions(func(options *ConsumerOptions) { options.ClaimIdle = 0 })},
		{name: "最大重试次数无效", client: client, handler: handler, options: changeConsumerOptions(func(options *ConsumerOptions) { options.MaxRetries = 0 })},
		{name: "死信 Stream 长度无效", client: client, handler: handler, options: changeConsumerOptions(func(options *ConsumerOptions) { options.MaxLength = 0 })},
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
		DeadKey:      "chatroom:stream:chat:dead",
		Group:        "chatroom-message-workers",
		ConsumerName: "server-1",
		BatchSize:    32,
		BlockTimeout: time.Second,
		ClaimIdle:    30 * time.Second,
		MaxRetries:   5,
		MaxLength:    100000,
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
	groupResult       *redis.StatusCmd
	readResult        *redis.XStreamSliceCmd
	ackResult         *redis.IntCmd
	autoClaimMessages []redis.XMessage
	autoClaimStart    string
	autoClaimErr      error
	autoClaimResults  []autoClaimResult
	pendingResult     []redis.XPendingExt
	pendingErr        error
	addID             string
	addErr            error
	onRead            func()
	onAck             func()
	onPending         func()

	groupStream          string
	groupName            string
	groupStart           string
	readArgs             *redis.XReadGroupArgs
	autoClaimArgs        *redis.XAutoClaimArgs
	autoClaimArgsHistory []*redis.XAutoClaimArgs
	pendingArgs          *redis.XPendingExtArgs
	addArgs              *redis.XAddArgs
	autoClaimCalls       int
	ackStream            string
	ackGroup             string
	ackIDs               []string
	ackCalls             [][]string
}

type autoClaimResult struct {
	messages []redis.XMessage
	next     string
	err      error
}

func (c *recordingConsumerClient) XAdd(_ context.Context, args *redis.XAddArgs) *redis.StringCmd {
	c.addArgs = args
	return redis.NewStringResult(c.addID, c.addErr)
}

func (c *recordingConsumerClient) XAutoClaim(ctx context.Context, args *redis.XAutoClaimArgs) *redis.XAutoClaimCmd {
	c.autoClaimArgs = args
	argsCopy := *args
	c.autoClaimArgsHistory = append(c.autoClaimArgsHistory, &argsCopy)
	result := autoClaimResult{messages: c.autoClaimMessages, next: c.autoClaimStart, err: c.autoClaimErr}
	if c.autoClaimCalls < len(c.autoClaimResults) {
		result = c.autoClaimResults[c.autoClaimCalls]
	}
	c.autoClaimCalls++
	cmd := redis.NewXAutoClaimCmd(ctx)
	cmd.SetVal(result.messages, result.next)
	cmd.SetErr(result.err)
	return cmd
}

func (c *recordingConsumerClient) XPendingExt(ctx context.Context, args *redis.XPendingExtArgs) *redis.XPendingExtCmd {
	c.pendingArgs = args
	cmd := redis.NewXPendingExtCmd(ctx)
	cmd.SetVal(c.pendingResult)
	cmd.SetErr(c.pendingErr)
	if c.onPending != nil {
		c.onPending()
	}
	return cmd
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
	c.ackCalls = append(c.ackCalls, append([]string(nil), ids...))
	if c.onAck != nil {
		c.onAck()
	}
	return c.ackResult
}
