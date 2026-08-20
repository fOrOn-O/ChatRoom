package redisstream

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"ChatRoom/internal/messagequeue"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrInvalidPublisherConfig 表示发布器缺少可用的 Redis Streams 配置。
	ErrInvalidPublisherConfig = errors.New("Redis Streams 发布器配置无效")
)

// StreamWriter 描述发布器使用的最小 Redis Streams 写入能力。
type StreamWriter interface {
	XAdd(ctx context.Context, args *redis.XAddArgs) *redis.StringCmd
}

// Publisher 将聊天消息发布到 Redis Streams。
type Publisher struct {
	writer    StreamWriter
	streamKey string
	maxLength int64
}

var (
	_ StreamWriter           = (*redis.Client)(nil)
	_ messagequeue.Publisher = (*Publisher)(nil)
)

// NewPublisher 创建 Redis Streams 聊天消息发布器。
func NewPublisher(writer StreamWriter, streamKey string, maxLength int64) (*Publisher, error) {
	if writer == nil {
		return nil, fmt.Errorf("%w: Redis 客户端不能为空", ErrInvalidPublisherConfig)
	}
	streamKey = strings.TrimSpace(streamKey)
	if streamKey == "" {
		return nil, fmt.Errorf("%w: Stream 键不能为空", ErrInvalidPublisherConfig)
	}
	if maxLength <= 0 {
		return nil, fmt.Errorf("%w: Stream 最大长度必须大于零", ErrInvalidPublisherConfig)
	}

	return &Publisher{
		writer:    writer,
		streamKey: streamKey,
		maxLength: maxLength,
	}, nil
}

// Publish 校验并编码聊天消息，再将其追加到 Redis Stream。
func (p *Publisher) Publish(ctx context.Context, message messagequeue.ChatMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	payload, err := messagequeue.EncodeChatMessage(message)
	if err != nil {
		return err
	}

	command := p.writer.XAdd(ctx, &redis.XAddArgs{
		Stream: p.streamKey,
		MaxLen: p.maxLength,
		Approx: true,
		ID:     "*",
		Values: map[string]interface{}{
			"msg_id":  message.MsgID,
			"payload": string(payload),
		},
	})
	if err := command.Err(); err != nil {
		return fmt.Errorf("写入 Redis Stream 失败: %w", err)
	}
	return nil
}
