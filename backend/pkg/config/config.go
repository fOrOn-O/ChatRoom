package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Log      LogConfig      `mapstructure:"log"`
	File     FileConfig     `mapstructure:"file"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"` // debug/release
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	Database     string `mapstructure:"database"`
	Charset      string `mapstructure:"charset"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Host         string            `mapstructure:"host"`
	Port         int               `mapstructure:"port"`
	Password     string            `mapstructure:"password"`
	DB           int               `mapstructure:"db"`
	KeyPrefix    string            `mapstructure:"key_prefix"`
	DialTimeout  time.Duration     `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration     `mapstructure:"read_timeout"`
	WriteTimeout time.Duration     `mapstructure:"write_timeout"`
	Stream       RedisStreamConfig `mapstructure:"stream"`
}

// RedisStreamConfig Redis Streams 配置
type RedisStreamConfig struct {
	ChatKey      string        `mapstructure:"chat_key"`
	DeadKey      string        `mapstructure:"dead_key"`
	Group        string        `mapstructure:"group"`
	BatchSize    int64         `mapstructure:"batch_size"`
	BlockTimeout time.Duration `mapstructure:"block_timeout"`
	ClaimIdle    time.Duration `mapstructure:"claim_idle"`
	MaxRetries   int64         `mapstructure:"max_retries"`
	MaxLength    int64         `mapstructure:"max_length"`
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret string        `mapstructure:"secret"`
	Expire time.Duration `mapstructure:"expire"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"` // json/text
	Output string `mapstructure:"output"` // stdout/file
}

// FileConfig 文件配置
type FileConfig struct {
	MaxSize      int64    `mapstructure:"max_size"`
	AllowedTypes []string `mapstructure:"allowed_types"`
	StoragePath  string   `mapstructure:"storage_path"`
}

// Load 加载配置
func Load(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	// 设置默认值
	setDefaults(&config)

	return &config, nil
}

// setDefaults 设置默认值
func setDefaults(config *Config) {
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}
	if config.Server.Mode == "" {
		config.Server.Mode = "debug"
	}
	if config.Database.Charset == "" {
		config.Database.Charset = "utf8mb4"
	}
	if config.Database.MaxIdleConns == 0 {
		config.Database.MaxIdleConns = 10
	}
	if config.Database.MaxOpenConns == 0 {
		config.Database.MaxOpenConns = 100
	}
	config.Redis.KeyPrefix = strings.Trim(config.Redis.KeyPrefix, " :")
	if config.Redis.KeyPrefix == "" {
		config.Redis.KeyPrefix = "chatroom"
	}
	if config.Redis.DialTimeout <= 0 {
		config.Redis.DialTimeout = 5 * time.Second
	}
	if config.Redis.ReadTimeout <= 0 {
		config.Redis.ReadTimeout = 3 * time.Second
	}
	if config.Redis.WriteTimeout <= 0 {
		config.Redis.WriteTimeout = 3 * time.Second
	}
	if config.Redis.Stream.ChatKey == "" {
		config.Redis.Stream.ChatKey = redisKey(config.Redis.KeyPrefix, "stream", "chat")
	}
	if config.Redis.Stream.DeadKey == "" {
		config.Redis.Stream.DeadKey = redisKey(config.Redis.KeyPrefix, "stream", "chat", "dead")
	}
	if config.Redis.Stream.Group == "" {
		config.Redis.Stream.Group = config.Redis.KeyPrefix + "-message-workers"
	}
	if config.Redis.Stream.BatchSize <= 0 {
		config.Redis.Stream.BatchSize = 32
	}
	if config.Redis.Stream.BlockTimeout <= 0 {
		config.Redis.Stream.BlockTimeout = time.Second
	}
	if config.Redis.Stream.ClaimIdle <= 0 {
		config.Redis.Stream.ClaimIdle = 30 * time.Second
	}
	if config.Redis.Stream.MaxRetries <= 0 {
		config.Redis.Stream.MaxRetries = 5
	}
	if config.Redis.Stream.MaxLength <= 0 {
		config.Redis.Stream.MaxLength = 100000
	}
	if config.JWT.Expire == 0 {
		config.JWT.Expire = 24 * time.Hour
	}
	if config.File.MaxSize == 0 {
		config.File.MaxSize = 50 * 1024 * 1024 // 50MB
	}
}

func redisKey(prefix string, parts ...string) string {
	segments := make([]string, 0, len(parts)+1)
	if prefix = strings.Trim(prefix, " :"); prefix != "" {
		segments = append(segments, prefix)
	}
	for _, part := range parts {
		if part = strings.Trim(part, " :"); part != "" {
			segments = append(segments, part)
		}
	}
	return strings.Join(segments, ":")
}
