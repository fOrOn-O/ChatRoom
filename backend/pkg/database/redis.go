package database

import (
	"context"
	"fmt"
	"log"

	"ChatRoom/pkg/config"

	"github.com/redis/go-redis/v9"
)

var RDB *redis.Client

// InitRedis 初始化 Redis
func InitRedis(cfg config.RedisConfig) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// 测试连接
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("连接 Redis 失败: %w", err)
	}

	RDB = rdb
	log.Println("Redis 连接成功")

	return rdb, nil
}
