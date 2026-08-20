package database

import (
	"context"
	"fmt"
	"log"

	"ChatRoom/pkg/config"

	"github.com/redis/go-redis/v9"
)

// InitRedis 初始化 Redis
func InitRedis(cfg config.RedisConfig) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	// 测试连接；直接构造旧版配置时沿用 Redis 客户端的默认超时。
	ctx := context.Background()
	cancel := func() {}
	if cfg.DialTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, cfg.DialTimeout)
	}
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("连接 Redis 失败: %w", err)
	}

	log.Println("Redis 连接成功")

	return rdb, nil
}
