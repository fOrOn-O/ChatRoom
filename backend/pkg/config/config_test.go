package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadAppliesRedisOperationalDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte("redis:\n  host: localhost\n  port: 6379\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("写入测试配置失败: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	if cfg.Redis.KeyPrefix != "chatroom" {
		t.Fatalf("Redis 键前缀 = %q，期望 %q", cfg.Redis.KeyPrefix, "chatroom")
	}
	if cfg.Redis.DialTimeout != 5*time.Second {
		t.Fatalf("Redis 连接超时 = %v，期望 %v", cfg.Redis.DialTimeout, 5*time.Second)
	}
	if cfg.Redis.ReadTimeout != 3*time.Second {
		t.Fatalf("Redis 读取超时 = %v，期望 %v", cfg.Redis.ReadTimeout, 3*time.Second)
	}
	if cfg.Redis.WriteTimeout != 3*time.Second {
		t.Fatalf("Redis 写入超时 = %v，期望 %v", cfg.Redis.WriteTimeout, 3*time.Second)
	}
	if cfg.Redis.Stream.ChatKey != "chatroom:stream:chat" {
		t.Fatalf("聊天 Stream 键 = %q，期望 %q", cfg.Redis.Stream.ChatKey, "chatroom:stream:chat")
	}
	if cfg.Redis.Stream.DeadKey != "chatroom:stream:chat:dead" {
		t.Fatalf("死信 Stream 键 = %q，期望 %q", cfg.Redis.Stream.DeadKey, "chatroom:stream:chat:dead")
	}
	if cfg.Redis.Stream.Group != "chatroom-message-workers" {
		t.Fatalf("消费组 = %q，期望 %q", cfg.Redis.Stream.Group, "chatroom-message-workers")
	}
	if cfg.Redis.Stream.BatchSize != 32 {
		t.Fatalf("批量读取数量 = %d，期望 %d", cfg.Redis.Stream.BatchSize, 32)
	}
	if cfg.Redis.Stream.BlockTimeout != time.Second {
		t.Fatalf("阻塞读取时间 = %v，期望 %v", cfg.Redis.Stream.BlockTimeout, time.Second)
	}
	if cfg.Redis.Stream.ClaimIdle != 30*time.Second {
		t.Fatalf("Pending 接管时间 = %v，期望 %v", cfg.Redis.Stream.ClaimIdle, 30*time.Second)
	}
	if cfg.Redis.Stream.MaxRetries != 5 {
		t.Fatalf("最大重试次数 = %d，期望 %d", cfg.Redis.Stream.MaxRetries, 5)
	}
	if cfg.Redis.Stream.MaxLength != 100000 {
		t.Fatalf("Stream 最大长度 = %d，期望 %d", cfg.Redis.Stream.MaxLength, 100000)
	}
	if cfg.Redis.RateLimit.Login.IPLimit != 30 {
		t.Fatalf("登录 IP 限制 = %d，期望 %d", cfg.Redis.RateLimit.Login.IPLimit, 30)
	}
	if cfg.Redis.RateLimit.Login.AccountIPLimit != 5 {
		t.Fatalf("登录账号与 IP 组合限制 = %d，期望 %d", cfg.Redis.RateLimit.Login.AccountIPLimit, 5)
	}
	if cfg.Redis.RateLimit.Login.Window != time.Minute {
		t.Fatalf("登录限流窗口 = %v，期望 %v", cfg.Redis.RateLimit.Login.Window, time.Minute)
	}
}

func TestLoadReplacesInvalidRedisOperationalValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`redis:
  host: localhost
  port: 6379
  key_prefix: " team: "
  dial_timeout: -1s
  read_timeout: -1s
  write_timeout: -1s
  stream:
    batch_size: -1
    block_timeout: -1s
    claim_idle: -1s
    max_retries: -1
    max_length: -1
  rate_limit:
    login:
      ip_limit: -1
      account_ip_limit: -1
      window: -1s
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("写入测试配置失败: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	if cfg.Redis.KeyPrefix != "team" {
		t.Fatalf("Redis 键前缀 = %q，期望去除分隔符后的 %q", cfg.Redis.KeyPrefix, "team")
	}
	if cfg.Redis.DialTimeout != 5*time.Second || cfg.Redis.ReadTimeout != 3*time.Second || cfg.Redis.WriteTimeout != 3*time.Second {
		t.Fatalf("非法 Redis 超时未回落到默认值: dial=%v read=%v write=%v", cfg.Redis.DialTimeout, cfg.Redis.ReadTimeout, cfg.Redis.WriteTimeout)
	}
	if cfg.Redis.Stream.ChatKey != "team:stream:chat" || cfg.Redis.Stream.DeadKey != "team:stream:chat:dead" {
		t.Fatalf("Stream 键未使用规范化前缀: chat=%q dead=%q", cfg.Redis.Stream.ChatKey, cfg.Redis.Stream.DeadKey)
	}
	if cfg.Redis.Stream.BatchSize != 32 || cfg.Redis.Stream.BlockTimeout != time.Second || cfg.Redis.Stream.ClaimIdle != 30*time.Second || cfg.Redis.Stream.MaxRetries != 5 || cfg.Redis.Stream.MaxLength != 100000 {
		t.Fatalf("非法 Stream 参数未回落到默认值: %+v", cfg.Redis.Stream)
	}
	loginLimit := cfg.Redis.RateLimit.Login
	if loginLimit.IPLimit != 30 || loginLimit.AccountIPLimit != 5 || loginLimit.Window != time.Minute {
		t.Fatalf("非法登录限流参数未回落到默认值: %+v", loginLimit)
	}
}

func TestLoadPreservesExplicitRedisOperationalValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`redis:
  host: redis.internal
  port: 6380
  key_prefix: im
  dial_timeout: 8s
  read_timeout: 4s
  write_timeout: 6s
  stream:
    chat_key: custom:chat
    dead_key: custom:dead
    group: custom-workers
    batch_size: 64
    block_timeout: 2s
    claim_idle: 45s
    max_retries: 8
    max_length: 200000
  rate_limit:
    login:
      ip_limit: 60
      account_ip_limit: 8
      window: 2m
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("写入测试配置失败: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	stream := cfg.Redis.Stream
	if cfg.Redis.KeyPrefix != "im" || cfg.Redis.DialTimeout != 8*time.Second || cfg.Redis.ReadTimeout != 4*time.Second || cfg.Redis.WriteTimeout != 6*time.Second {
		t.Fatalf("显式 Redis 配置被意外覆盖: %+v", cfg.Redis)
	}
	if stream.ChatKey != "custom:chat" || stream.DeadKey != "custom:dead" || stream.Group != "custom-workers" || stream.BatchSize != 64 || stream.BlockTimeout != 2*time.Second || stream.ClaimIdle != 45*time.Second || stream.MaxRetries != 8 || stream.MaxLength != 200000 {
		t.Fatalf("显式 Stream 配置被意外覆盖: %+v", stream)
	}
	loginLimit := cfg.Redis.RateLimit.Login
	if loginLimit.IPLimit != 60 || loginLimit.AccountIPLimit != 8 || loginLimit.Window != 2*time.Minute {
		t.Fatalf("显式登录限流配置被意外覆盖: %+v", loginLimit)
	}
}
