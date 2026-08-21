package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const messageLimitScript = `
local count = redis.call("INCR", KEYS[1])
if count == 1 then
    redis.call("PEXPIRE", KEYS[1], ARGV[2])
end

if count > tonumber(ARGV[1]) then
    return 0
end
return 1
`

// MessageLimiter 检查用户当前是否可以发送一条聊天消息。
type MessageLimiter interface {
	Allow(ctx context.Context, userID uint) (bool, error)
}

// MessageOptions 消息发送限流参数。
type MessageOptions struct {
	KeyPrefix string
	Limit     int64
	Window    time.Duration
}

type redisMessageLimiter struct {
	client    redis.Scripter
	script    *redis.Script
	keyPrefix string
	limit     int64
	window    time.Duration
}

// NewMessageLimiter 创建基于 Redis 固定窗口的消息限流器。
func NewMessageLimiter(client redis.Scripter, options MessageOptions) (MessageLimiter, error) {
	if client == nil {
		return nil, errors.New("Redis 客户端不能为空")
	}
	keyPrefix := strings.Trim(options.KeyPrefix, " :")
	if keyPrefix == "" {
		return nil, errors.New("Redis 键前缀不能为空")
	}
	if options.Limit <= 0 {
		return nil, errors.New("消息限流次数必须大于零")
	}
	if options.Window < time.Millisecond {
		return nil, errors.New("消息限流窗口不能小于一毫秒")
	}

	return &redisMessageLimiter{
		client:    client,
		script:    redis.NewScript(messageLimitScript),
		keyPrefix: keyPrefix,
		limit:     options.Limit,
		window:    options.Window,
	}, nil
}

func (l *redisMessageLimiter) Allow(ctx context.Context, userID uint) (bool, error) {
	if userID == 0 {
		return false, errors.New("消息限流用户不能为空")
	}
	key := l.keyPrefix + ":rate_limit:message:user:" + strconv.FormatUint(uint64(userID), 10)
	result, err := l.script.Run(
		ctx,
		l.client,
		[]string{key},
		l.limit,
		l.window.Milliseconds(),
	).Int64()
	if err != nil {
		return false, fmt.Errorf("执行 Redis 消息限流失败: %w", err)
	}
	return result == 1, nil
}
