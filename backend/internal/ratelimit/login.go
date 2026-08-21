package ratelimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const loginLimitScript = `
local ip_count = redis.call("INCR", KEYS[1])
if ip_count == 1 then
    redis.call("PEXPIRE", KEYS[1], ARGV[3])
end

local account_ip_count = redis.call("INCR", KEYS[2])
if account_ip_count == 1 then
    redis.call("PEXPIRE", KEYS[2], ARGV[3])
end

if ip_count > tonumber(ARGV[1]) or account_ip_count > tonumber(ARGV[2]) then
    return 0
end
return 1
`

// LoginLimiter 检查一次登录尝试是否可以继续。
type LoginLimiter interface {
	Allow(ctx context.Context, ip string, username string) (bool, error)
}

// LoginOptions 登录限流参数。
type LoginOptions struct {
	KeyPrefix      string
	IPLimit        int64
	AccountIPLimit int64
	Window         time.Duration
}

type redisLoginLimiter struct {
	client         redis.Scripter
	script         *redis.Script
	keyPrefix      string
	ipLimit        int64
	accountIPLimit int64
	window         time.Duration
}

// NewLoginLimiter 创建基于 Redis 固定窗口的登录限流器。
func NewLoginLimiter(client redis.Scripter, options LoginOptions) (LoginLimiter, error) {
	if client == nil {
		return nil, errors.New("Redis 客户端不能为空")
	}
	keyPrefix := strings.Trim(options.KeyPrefix, " :")
	if keyPrefix == "" {
		return nil, errors.New("Redis 键前缀不能为空")
	}
	if options.IPLimit <= 0 || options.AccountIPLimit <= 0 {
		return nil, errors.New("登录限流次数必须大于零")
	}
	if options.Window < time.Millisecond {
		return nil, errors.New("登录限流窗口不能小于一毫秒")
	}

	return &redisLoginLimiter{
		client:         client,
		script:         redis.NewScript(loginLimitScript),
		keyPrefix:      keyPrefix,
		ipLimit:        options.IPLimit,
		accountIPLimit: options.AccountIPLimit,
		window:         options.Window,
	}, nil
}

func (l *redisLoginLimiter) Allow(ctx context.Context, ip string, username string) (bool, error) {
	ip = strings.TrimSpace(ip)
	username = strings.ToLower(strings.TrimSpace(username))
	if ip == "" || username == "" {
		return false, errors.New("登录限流标识不能为空")
	}

	keys := []string{
		l.keyPrefix + ":rate_limit:login:ip:" + hashLoginIdentifier(ip),
		l.keyPrefix + ":rate_limit:login:account_ip:" + hashLoginIdentifier(username+"\x00"+ip),
	}
	result, err := l.script.Run(
		ctx,
		l.client,
		keys,
		l.ipLimit,
		l.accountIPLimit,
		l.window.Milliseconds(),
	).Int64()
	if err != nil {
		return false, fmt.Errorf("执行 Redis 登录限流失败: %w", err)
	}
	return result == 1, nil
}

func hashLoginIdentifier(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
