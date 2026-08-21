package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ChatRoom/internal/api"
	"ChatRoom/internal/ratelimit"
	"ChatRoom/internal/ws"
	"ChatRoom/pkg/config"
	"ChatRoom/pkg/database"
	"ChatRoom/pkg/logger"
)

func main() {
	// 1. 加载配置
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 2. 初始化日志
	logger.Init(cfg.Log)
	defer logger.Sync()

	// 3. 初始化数据库
	db, err := database.Init(cfg.Database)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer database.Close()

	// 4. 初始化 Redis
	rdb, err := database.InitRedis(cfg.Redis)
	if err != nil {
		log.Fatalf("初始化 Redis 失败: %v", err)
	}
	defer rdb.Close()

	// 5. 创建 WebSocket Hub
	hub := ws.NewHub(db)
	go hub.Run()
	hostname, hostnameErr := os.Hostname()
	if hostnameErr != nil || hostname == "" {
		hostname = "chatroom"
	}
	queueRuntime, err := startMessageQueueRuntime(
		context.Background(),
		rdb,
		hub,
		cfg.Redis.Stream,
		fmt.Sprintf("%s-%d", hostname, os.Getpid()),
	)
	if err != nil {
		log.Fatalf("初始化 Redis Streams 消息队列失败: %v", err)
	}
	loginLimiter, err := ratelimit.NewLoginLimiter(rdb, ratelimit.LoginOptions{
		KeyPrefix:      cfg.Redis.KeyPrefix,
		IPLimit:        cfg.Redis.RateLimit.Login.IPLimit,
		AccountIPLimit: cfg.Redis.RateLimit.Login.AccountIPLimit,
		Window:         cfg.Redis.RateLimit.Login.Window,
	})
	if err != nil {
		log.Fatalf("初始化登录限流器失败: %v", err)
	}

	// 6. 初始化路由
	router := api.NewRouter(hub, db, queueRuntime.Publisher(), loginLimiter, cfg.JWT.Secret)

	// 7. 创建 HTTP Server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 监听系统信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	shutdownComplete := make(chan struct{})
	queueFailure := make(chan error, 1)
	go func() {
		<-queueRuntime.Done()
		if err := queueRuntime.Err(); err != nil {
			queueFailure <- err
		}
	}()

	go func() {
		defer close(shutdownComplete)
		var consumerFailure error
		select {
		case <-sigCh:
			log.Println("收到关闭信号，正在优雅关闭...")
		case consumerFailure = <-queueFailure:
			log.Printf("Redis Streams 消费者异常退出，正在关闭服务: %v", consumerFailure)
		}

		// 给予 10 秒时间处理未完成的请求
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		serverShutdown := make(chan error, 1)
		go func() {
			serverShutdown <- server.Shutdown(shutdownCtx)
		}()

		if err := queueRuntime.Shutdown(shutdownCtx); err != nil && consumerFailure == nil {
			log.Printf("Redis Streams 消费者关闭错误: %v", err)
		}
		if err := hub.Shutdown(shutdownCtx); err != nil {
			log.Printf("WebSocket Hub 关闭错误: %v", err)
		}
		if err := <-serverShutdown; err != nil {
			log.Printf("服务器关闭错误: %v", err)
		}
	}()

	// 9. 启动服务器
	log.Printf("服务器启动在 %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if shutdownErr := queueRuntime.Shutdown(shutdownCtx); shutdownErr != nil {
			log.Printf("Redis Streams 消费者关闭错误: %v", shutdownErr)
		}
		if shutdownErr := hub.Shutdown(shutdownCtx); shutdownErr != nil {
			log.Printf("WebSocket Hub 关闭错误: %v", shutdownErr)
		}
		shutdownCancel()
		log.Fatalf("服务器启动失败: %v", err)
	}
	<-shutdownComplete

	log.Println("服务器已关闭")
}
