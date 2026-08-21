package ratelimit

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestLoginLimiterRestrictsAccountAndIPWithinWindow(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("关闭 Redis 测试客户端失败: %v", err)
		}
	})

	limiter, err := NewLoginLimiter(client, LoginOptions{
		KeyPrefix:      "chatroom",
		IPLimit:        10,
		AccountIPLimit: 1,
		Window:         time.Minute,
	})
	if err != nil {
		t.Fatalf("创建登录限流器失败: %v", err)
	}

	ctx := context.Background()
	allowed, err := limiter.Allow(ctx, "203.0.113.10", "Alice")
	if err != nil || !allowed {
		t.Fatalf("首次登录检查结果 = allowed %t, err %v，期望允许", allowed, err)
	}
	for _, key := range server.Keys() {
		if strings.Contains(key, "203.0.113.10") || strings.Contains(strings.ToLower(key), "alice") {
			t.Fatalf("Redis 限流键泄露原始登录标识: %q", key)
		}
	}

	allowed, err = limiter.Allow(ctx, "203.0.113.10", " alice ")
	if err != nil {
		t.Fatalf("第二次登录检查失败: %v", err)
	}
	if allowed {
		t.Fatal("同一账号与 IP 在窗口内超过限制后仍被允许")
	}

	allowed, err = limiter.Allow(ctx, "203.0.113.10", "bob")
	if err != nil || !allowed {
		t.Fatalf("同一 IP 下其他账号的检查结果 = allowed %t, err %v，期望允许", allowed, err)
	}

	server.FastForward(time.Minute)
	allowed, err = limiter.Allow(ctx, "203.0.113.10", "Alice")
	if err != nil || !allowed {
		t.Fatalf("窗口结束后的检查结果 = allowed %t, err %v，期望恢复允许", allowed, err)
	}
}

func TestLoginLimiterRestrictsDifferentAccountsFromSameIP(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("关闭 Redis 测试客户端失败: %v", err)
		}
	})

	limiter, err := NewLoginLimiter(client, LoginOptions{
		KeyPrefix:      "chatroom",
		IPLimit:        2,
		AccountIPLimit: 10,
		Window:         time.Minute,
	})
	if err != nil {
		t.Fatalf("创建登录限流器失败: %v", err)
	}

	ctx := context.Background()
	for _, username := range []string{"alice", "bob"} {
		allowed, allowErr := limiter.Allow(ctx, "203.0.113.20", username)
		if allowErr != nil || !allowed {
			t.Fatalf("账号 %q 的检查结果 = allowed %t, err %v，期望允许", username, allowed, allowErr)
		}
	}

	allowed, err := limiter.Allow(ctx, "203.0.113.20", "carol")
	if err != nil {
		t.Fatalf("第三次 IP 登录检查失败: %v", err)
	}
	if allowed {
		t.Fatal("同一 IP 使用不同账号超过限制后仍被允许")
	}
}

func TestLoginLimiterAppliesLimitAtomicallyUnderConcurrency(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("关闭 Redis 测试客户端失败: %v", err)
		}
	})

	const limit = int64(7)
	limiter, err := NewLoginLimiter(client, LoginOptions{
		KeyPrefix:      "chatroom",
		IPLimit:        limit,
		AccountIPLimit: limit,
		Window:         time.Minute,
	})
	if err != nil {
		t.Fatalf("创建登录限流器失败: %v", err)
	}

	var allowedCount atomic.Int64
	errorsCh := make(chan error, 32)
	var workers sync.WaitGroup
	for range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			allowed, allowErr := limiter.Allow(context.Background(), "203.0.113.50", "alice")
			if allowErr != nil {
				errorsCh <- allowErr
				return
			}
			if allowed {
				allowedCount.Add(1)
			}
		}()
	}
	workers.Wait()
	close(errorsCh)
	for err := range errorsCh {
		t.Fatalf("并发登录检查失败: %v", err)
	}
	if allowedCount.Load() != limit {
		t.Fatalf("并发请求允许数量 = %d，期望 %d", allowedCount.Load(), limit)
	}
}
