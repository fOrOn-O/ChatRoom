package redisstream

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"ChatRoom/internal/messagequeue"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrInvalidConsumerConfig 表示消费者缺少可用的 Redis Streams 配置。
	ErrInvalidConsumerConfig = errors.New("Redis Streams 消费者配置无效")
)

// ConsumerClient 描述消费者使用的最小 Redis Streams 能力。
type ConsumerClient interface {
	XGroupCreateMkStream(ctx context.Context, stream, group, start string) *redis.StatusCmd
	XReadGroup(ctx context.Context, args *redis.XReadGroupArgs) *redis.XStreamSliceCmd
	XAck(ctx context.Context, stream, group string, ids ...string) *redis.IntCmd
}

// ConsumerOptions 定义 Redis Streams 消费参数。
type ConsumerOptions struct {
	StreamKey    string
	Group        string
	ConsumerName string
	BatchSize    int64
	BlockTimeout time.Duration
}

// Consumer 从 Redis Streams 读取并处理聊天消息。
type Consumer struct {
	client  ConsumerClient
	handler messagequeue.Handler
	options ConsumerOptions
}

var _ ConsumerClient = (*redis.Client)(nil)

// NewConsumer 创建 Redis Streams 聊天消息消费者。
func NewConsumer(client ConsumerClient, handler messagequeue.Handler, options ConsumerOptions) (*Consumer, error) {
	options.StreamKey = strings.TrimSpace(options.StreamKey)
	options.Group = strings.TrimSpace(options.Group)
	options.ConsumerName = strings.TrimSpace(options.ConsumerName)

	if client == nil {
		return nil, fmt.Errorf("%w: Redis 客户端不能为空", ErrInvalidConsumerConfig)
	}
	if handler == nil {
		return nil, fmt.Errorf("%w: 消息处理器不能为空", ErrInvalidConsumerConfig)
	}
	if options.StreamKey == "" || options.Group == "" || options.ConsumerName == "" {
		return nil, fmt.Errorf("%w: Stream、消费者组和消费者名称不能为空", ErrInvalidConsumerConfig)
	}
	if options.BatchSize <= 0 {
		return nil, fmt.Errorf("%w: 批量读取数量必须大于零", ErrInvalidConsumerConfig)
	}
	if options.BlockTimeout <= 0 {
		return nil, fmt.Errorf("%w: 阻塞读取时间必须大于零", ErrInvalidConsumerConfig)
	}

	return &Consumer{client: client, handler: handler, options: options}, nil
}

// Run 创建消费者组并持续消费新消息，Context 取消时正常退出。
func (c *Consumer) Run(ctx context.Context) error {
	if ctx.Err() != nil {
		return nil
	}
	if err := c.client.XGroupCreateMkStream(ctx, c.options.StreamKey, c.options.Group, "0").Err(); err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("创建 Redis Streams 消费者组失败: %w", err)
	}

	for {
		if ctx.Err() != nil {
			return nil
		}

		streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    c.options.Group,
			Consumer: c.options.ConsumerName,
			Streams:  []string{c.options.StreamKey, ">"},
			Count:    c.options.BatchSize,
			Block:    c.options.BlockTimeout,
			NoAck:    false,
		}).Result()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("读取 Redis Stream 失败: %w", err)
		}

		for _, stream := range streams {
			for _, record := range stream.Messages {
				message, err := decodeRecord(record)
				if err != nil {
					log.Printf("Redis Stream 消息载荷无效，保留在 Pending 中: stream_id=%s err=%v", record.ID, err)
					continue
				}
				if err := c.handler.Handle(ctx, message); err != nil {
					log.Printf("Redis Stream 消息处理失败，保留在 Pending 中: stream_id=%s msg_id=%s err=%v", record.ID, message.MsgID, err)
					continue
				}
				if err := c.client.XAck(ctx, c.options.StreamKey, c.options.Group, record.ID).Err(); err != nil {
					if ctx.Err() != nil {
						return nil
					}
					return fmt.Errorf("确认 Redis Stream 消息失败: %w", err)
				}
			}
		}
	}
}

func decodeRecord(record redis.XMessage) (messagequeue.ChatMessage, error) {
	value, ok := record.Values["payload"]
	if !ok {
		return messagequeue.ChatMessage{}, fmt.Errorf("%w: Stream 记录 %s 缺少 payload", messagequeue.ErrInvalidChatMessage, record.ID)
	}

	var payload []byte
	switch value := value.(type) {
	case string:
		payload = []byte(value)
	case []byte:
		payload = value
	default:
		return messagequeue.ChatMessage{}, fmt.Errorf("%w: Stream 记录 %s 的 payload 类型为 %T", messagequeue.ErrInvalidChatMessage, record.ID, value)
	}

	message, err := messagequeue.DecodeChatMessage(payload)
	if err != nil {
		return messagequeue.ChatMessage{}, fmt.Errorf("解码 Stream 记录 %s 失败: %w", record.ID, err)
	}
	return message, nil
}
