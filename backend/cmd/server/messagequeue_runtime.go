package main

import (
	"context"
	"sync"

	"ChatRoom/internal/messagequeue"
	"ChatRoom/internal/messagequeue/redisstream"
	"ChatRoom/pkg/config"
)

type queueConsumer interface {
	Run(context.Context) error
}

type redisStreamClient interface {
	redisstream.StreamWriter
	redisstream.ConsumerClient
}

type messageQueueRuntime struct {
	publisher messagequeue.Publisher
	worker    *queueWorker
}

type queueWorker struct {
	cancel context.CancelFunc
	done   chan struct{}

	errMu sync.RWMutex
	err   error
}

func startQueueWorker(parent context.Context, consumer queueConsumer) *queueWorker {
	ctx, cancel := context.WithCancel(parent)
	worker := &queueWorker{
		cancel: cancel,
		done:   make(chan struct{}),
	}

	go func() {
		err := consumer.Run(ctx)
		worker.errMu.Lock()
		worker.err = err
		worker.errMu.Unlock()
		close(worker.done)
	}()

	return worker
}

func (w *queueWorker) Done() <-chan struct{} {
	return w.done
}

func (w *queueWorker) Err() error {
	w.errMu.RLock()
	defer w.errMu.RUnlock()
	return w.err
}

func (w *queueWorker) Shutdown(ctx context.Context) error {
	w.cancel()
	select {
	case <-w.done:
		return w.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func startMessageQueueRuntime(
	parent context.Context,
	client redisStreamClient,
	handler messagequeue.Handler,
	streamConfig config.RedisStreamConfig,
	consumerName string,
) (*messageQueueRuntime, error) {
	publisher, err := redisstream.NewPublisher(client, streamConfig.ChatKey, streamConfig.MaxLength)
	if err != nil {
		return nil, err
	}
	consumer, err := redisstream.NewConsumer(client, handler, redisstream.ConsumerOptions{
		StreamKey:    streamConfig.ChatKey,
		DeadKey:      streamConfig.DeadKey,
		Group:        streamConfig.Group,
		ConsumerName: consumerName,
		BatchSize:    streamConfig.BatchSize,
		BlockTimeout: streamConfig.BlockTimeout,
		ClaimIdle:    streamConfig.ClaimIdle,
		MaxRetries:   streamConfig.MaxRetries,
		MaxLength:    streamConfig.MaxLength,
	})
	if err != nil {
		return nil, err
	}

	return &messageQueueRuntime{
		publisher: publisher,
		worker:    startQueueWorker(parent, consumer),
	}, nil
}

func (r *messageQueueRuntime) Publisher() messagequeue.Publisher {
	return r.publisher
}

func (r *messageQueueRuntime) Done() <-chan struct{} {
	return r.worker.Done()
}

func (r *messageQueueRuntime) Err() error {
	return r.worker.Err()
}

func (r *messageQueueRuntime) Shutdown(ctx context.Context) error {
	return r.worker.Shutdown(ctx)
}
