package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestMessageLimiterRestrictsEachUserWithinWindow(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("关闭 Redis 测试客户端失败: %v", err)
		}
	})

	limiter, err := NewMessageLimiter(client, MessageOptions{
		KeyPrefix: "chatroom",
		Limit:     2,
		Window:    10 * time.Second,
	})
	if err != nil {
		t.Fatalf("创建消息限流器失败: %v", err)
	}

	ctx := context.Background()
	for attempt := 1; attempt <= 2; attempt++ {
		allowed, allowErr := limiter.Allow(ctx, 7)
		if allowErr != nil || !allowed {
			t.Fatalf("用户 7 第 %d 次检查结果 = allowed %t, err %v，期望允许", attempt, allowed, allowErr)
		}
	}
	allowed, err := limiter.Allow(ctx, 8)
	if err != nil || !allowed {
		t.Fatalf("其他用户检查结果 = allowed %t, err %v，期望不共享额度", allowed, err)
	}

	allowed, err = limiter.Allow(ctx, 7)
	if err != nil {
		t.Fatalf("用户 7 超限检查失败: %v", err)
	}
	if allowed {
		t.Fatal("用户 7 在窗口内超过消息限制后仍被允许")
	}

	server.FastForward(10 * time.Second)
	allowed, err = limiter.Allow(ctx, 7)
	if err != nil || !allowed {
		t.Fatalf("窗口结束后的检查结果 = allowed %t, err %v，期望恢复允许", allowed, err)
	}
}

func TestMessageLimiterAppliesLimitAtomicallyUnderConcurrency(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("关闭 Redis 测试客户端失败: %v", err)
		}
	})

	const limit = int64(7)
	limiter, err := NewMessageLimiter(client, MessageOptions{
		KeyPrefix: "chatroom",
		Limit:     limit,
		Window:    10 * time.Second,
	})
	if err != nil {
		t.Fatalf("创建消息限流器失败: %v", err)
	}

	var allowedCount atomic.Int64
	errorsCh := make(chan error, 32)
	var workers sync.WaitGroup
	for range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			allowed, allowErr := limiter.Allow(context.Background(), 7)
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
		t.Fatalf("并发消息限流检查失败: %v", err)
	}
	if allowedCount.Load() != limit {
		t.Fatalf("并发消息允许数量 = %d，期望 %d", allowedCount.Load(), limit)
	}
}
