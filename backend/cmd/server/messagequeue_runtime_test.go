package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"ChatRoom/internal/messagequeue"
	"ChatRoom/pkg/config"

	"github.com/redis/go-redis/v9"
)

func TestQueueWorkerShutdownCancelsAndWaitsForConsumer(t *testing.T) {
	consumer := &blockingQueueConsumer{
		started:  make(chan struct{}),
		canceled: make(chan struct{}),
	}
	worker := startQueueWorker(context.Background(), consumer)

	select {
	case <-consumer.started:
	case <-time.After(time.Second):
		t.Fatal("消费者未启动")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := worker.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("关闭队列消费者失败: %v", err)
	}

	select {
	case <-consumer.canceled:
	default:
		t.Fatal("关闭返回前消费者未收到取消信号")
	}
}

func TestQueueWorkerExposesUnexpectedConsumerFailure(t *testing.T) {
	wantErr := errors.New("Redis Stream 读取失败")
	worker := startQueueWorker(context.Background(), failingQueueConsumer{err: wantErr})

	select {
	case <-worker.Done():
	case <-time.After(time.Second):
		t.Fatal("消费者异常退出后未结束运行器")
	}
	if !errors.Is(worker.Err(), wantErr) {
		t.Fatalf("运行器错误 = %v，期望 %v", worker.Err(), wantErr)
	}
}

func TestStartMessageQueueRuntimeBuildsAndRunsConfiguredComponents(t *testing.T) {
	client := &blockingStreamClient{readStarted: make(chan struct{})}
	streamConfig := config.RedisStreamConfig{
		ChatKey:      "chatroom:stream:chat",
		DeadKey:      "chatroom:stream:chat:dead",
		Group:        "chatroom-message-workers",
		BatchSize:    32,
		BlockTimeout: time.Second,
		ClaimIdle:    30 * time.Second,
		MaxRetries:   5,
		MaxLength:    100000,
	}

	runtime, err := startMessageQueueRuntime(
		context.Background(),
		client,
		nopQueueHandler{},
		streamConfig,
		"chatroom-test-1",
	)
	if err != nil {
		t.Fatalf("启动消息队列运行时失败: %v", err)
	}
	if runtime.Publisher() == nil {
		t.Fatal("消息队列运行时未创建发布器")
	}

	select {
	case <-client.readStarted:
	case <-time.After(time.Second):
		t.Fatal("消息队列消费者未开始读取")
	}
	if client.readArgs.Group != streamConfig.Group || client.readArgs.Consumer != "chatroom-test-1" {
		t.Fatalf("消费者读取配置 = group %q, consumer %q", client.readArgs.Group, client.readArgs.Consumer)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := runtime.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("关闭消息队列运行时失败: %v", err)
	}
}

type blockingQueueConsumer struct {
	started  chan struct{}
	canceled chan struct{}
}

func (c *blockingQueueConsumer) Run(ctx context.Context) error {
	close(c.started)
	<-ctx.Done()
	close(c.canceled)
	return nil
}

type failingQueueConsumer struct {
	err error
}

func (c failingQueueConsumer) Run(context.Context) error {
	return c.err
}

type nopQueueHandler struct{}

func (nopQueueHandler) Handle(context.Context, messagequeue.ChatMessage) error {
	return nil
}

type blockingStreamClient struct {
	readStarted chan struct{}
	readArgs    *redis.XReadGroupArgs
}

func (c *blockingStreamClient) XAdd(context.Context, *redis.XAddArgs) *redis.StringCmd {
	return redis.NewStringResult("1-0", nil)
}

func (c *blockingStreamClient) XGroupCreateMkStream(context.Context, string, string, string) *redis.StatusCmd {
	return redis.NewStatusResult("OK", nil)
}

func (c *blockingStreamClient) XReadGroup(ctx context.Context, args *redis.XReadGroupArgs) *redis.XStreamSliceCmd {
	c.readArgs = args
	close(c.readStarted)
	<-ctx.Done()
	cmd := redis.NewXStreamSliceCmd(ctx)
	cmd.SetErr(ctx.Err())
	return cmd
}

func (c *blockingStreamClient) XAutoClaim(ctx context.Context, _ *redis.XAutoClaimArgs) *redis.XAutoClaimCmd {
	cmd := redis.NewXAutoClaimCmd(ctx)
	cmd.SetVal(nil, "0-0")
	return cmd
}

func (c *blockingStreamClient) XPendingExt(ctx context.Context, _ *redis.XPendingExtArgs) *redis.XPendingExtCmd {
	return redis.NewXPendingExtCmd(ctx)
}

func (c *blockingStreamClient) XAck(context.Context, string, string, ...string) *redis.IntCmd {
	return redis.NewIntResult(1, nil)
}
