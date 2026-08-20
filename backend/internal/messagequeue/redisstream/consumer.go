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
	XAdd(ctx context.Context, args *redis.XAddArgs) *redis.StringCmd
	XGroupCreateMkStream(ctx context.Context, stream, group, start string) *redis.StatusCmd
	XReadGroup(ctx context.Context, args *redis.XReadGroupArgs) *redis.XStreamSliceCmd
	XAutoClaim(ctx context.Context, args *redis.XAutoClaimArgs) *redis.XAutoClaimCmd
	XPendingExt(ctx context.Context, args *redis.XPendingExtArgs) *redis.XPendingExtCmd
	XAck(ctx context.Context, stream, group string, ids ...string) *redis.IntCmd
}

// ConsumerOptions 定义 Redis Streams 消费参数。
type ConsumerOptions struct {
	StreamKey    string
	DeadKey      string
	Group        string
	ConsumerName string
	BatchSize    int64
	BlockTimeout time.Duration
	ClaimIdle    time.Duration
	MaxRetries   int64
	MaxLength    int64
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
	options.DeadKey = strings.TrimSpace(options.DeadKey)
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
	if options.DeadKey == "" {
		return nil, fmt.Errorf("%w: 死信 Stream 不能为空", ErrInvalidConsumerConfig)
	}
	if options.BatchSize <= 0 {
		return nil, fmt.Errorf("%w: 批量读取数量必须大于零", ErrInvalidConsumerConfig)
	}
	if options.BlockTimeout <= 0 {
		return nil, fmt.Errorf("%w: 阻塞读取时间必须大于零", ErrInvalidConsumerConfig)
	}
	if options.ClaimIdle <= 0 {
		return nil, fmt.Errorf("%w: Pending 接管时间必须大于零", ErrInvalidConsumerConfig)
	}
	if options.MaxRetries <= 0 {
		return nil, fmt.Errorf("%w: 最大重试次数必须大于零", ErrInvalidConsumerConfig)
	}
	if options.MaxLength <= 0 {
		return nil, fmt.Errorf("%w: 死信 Stream 长度必须大于零", ErrInvalidConsumerConfig)
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
	nextClaimAt := time.Time{}
	for {
		if ctx.Err() != nil {
			return nil
		}
		if !time.Now().Before(nextClaimAt) {
			if err := c.recoverPending(ctx); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return err
			}
			nextClaimAt = time.Now().Add(c.options.ClaimIdle)
			if ctx.Err() != nil {
				return nil
			}
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
				if err := c.processNewRecord(ctx, record); err != nil {
					if ctx.Err() != nil {
						return nil
					}
					return err
				}
			}
		}
	}
}

func (c *Consumer) recoverPending(ctx context.Context) error {
	start := "0-0"
	for {
		records, next, err := c.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
			Stream:   c.options.StreamKey,
			Group:    c.options.Group,
			Consumer: c.options.ConsumerName,
			MinIdle:  c.options.ClaimIdle,
			Start:    start,
			Count:    c.options.BatchSize,
		}).Result()
		if err != nil {
			return fmt.Errorf("接管 Redis Stream Pending 消息失败: %w", err)
		}
		for _, record := range records {
			if err := c.processClaimedRecord(ctx, record); err != nil {
				return err
			}
		}
		if next == "" || next == "0-0" {
			return nil
		}
		start = next
	}
}

func (c *Consumer) processNewRecord(ctx context.Context, record redis.XMessage) error {
	message, err := c.handleRecord(ctx, record)
	if err != nil {
		log.Printf("Redis Stream 消息处理失败，保留在 Pending 中: stream_id=%s msg_id=%s err=%v", record.ID, message.MsgID, err)
		return nil
	}
	return c.acknowledge(ctx, record.ID)
}

func (c *Consumer) processClaimedRecord(ctx context.Context, record redis.XMessage) error {
	message, handleErr := c.handleRecord(ctx, record)
	if handleErr == nil {
		return c.acknowledge(ctx, record.ID)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	deliveryCount, err := c.pendingDeliveryCount(ctx, record.ID)
	if err != nil {
		return err
	}
	retryCount := deliveryCount - 1
	if retryCount < 0 {
		retryCount = 0
	}
	if retryCount >= c.options.MaxRetries {
		if err := c.moveToDeadStream(ctx, record, deliveryCount, retryCount, handleErr); err != nil {
			return err
		}
		log.Printf("Redis Stream Pending 消息已转入死信: stream_id=%s msg_id=%s retry_count=%d", record.ID, message.MsgID, retryCount)
		return c.acknowledge(ctx, record.ID)
	}
	log.Printf("Redis Stream Pending 消息处理失败，继续保留: stream_id=%s msg_id=%s retry_count=%d err=%v", record.ID, message.MsgID, retryCount, handleErr)
	return nil
}

func (c *Consumer) moveToDeadStream(ctx context.Context, record redis.XMessage, deliveryCount, retryCount int64, cause error) error {
	values := make(map[string]interface{}, len(record.Values)+8)
	for key, value := range record.Values {
		values[key] = value
	}
	values["source_stream"] = c.options.StreamKey
	values["source_id"] = record.ID
	values["consumer_group"] = c.options.Group
	values["consumer"] = c.options.ConsumerName
	values["delivery_count"] = deliveryCount
	values["retry_count"] = retryCount
	values["error"] = cause.Error()
	values["failed_at"] = time.Now().UTC().Format(time.RFC3339Nano)

	if err := c.client.XAdd(ctx, &redis.XAddArgs{
		Stream: c.options.DeadKey,
		MaxLen: c.options.MaxLength,
		Approx: true,
		Values: values,
	}).Err(); err != nil {
		return fmt.Errorf("写入 Redis Stream 死信失败: %w", err)
	}
	return nil
}

func (c *Consumer) pendingDeliveryCount(ctx context.Context, recordID string) (int64, error) {
	pending, err := c.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream:   c.options.StreamKey,
		Group:    c.options.Group,
		Start:    recordID,
		End:      recordID,
		Count:    1,
		Consumer: c.options.ConsumerName,
	}).Result()
	if err != nil {
		return 0, fmt.Errorf("查询 Redis Stream Pending 消息失败: %w", err)
	}
	if len(pending) != 1 || pending[0].ID != recordID {
		return 0, fmt.Errorf("查询 Redis Stream Pending 消息失败: 未找到记录 %s", recordID)
	}
	return pending[0].RetryCount, nil
}

func (c *Consumer) handleRecord(ctx context.Context, record redis.XMessage) (messagequeue.ChatMessage, error) {
	message, err := decodeRecord(record)
	if err != nil {
		return messagequeue.ChatMessage{}, err
	}
	if err := c.handler.Handle(ctx, message); err != nil {
		return message, err
	}
	return message, nil
}

func (c *Consumer) acknowledge(ctx context.Context, recordID string) error {
	if err := c.client.XAck(ctx, c.options.StreamKey, c.options.Group, recordID).Err(); err != nil {
		return fmt.Errorf("确认 Redis Stream 消息失败: %w", err)
	}
	return nil
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
