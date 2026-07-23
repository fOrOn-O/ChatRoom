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
	hub := ws.NewHub(db, rdb)
	go hub.Run()

	// 6. 初始化路由
	router := api.NewRouter(hub, db, rdb, cfg.JWT.Secret)

	// 7. 创建 HTTP Server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 8. 优雅关闭
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听系统信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("收到关闭信号，正在优雅关闭...")
		cancel()

		// 给予 10 秒时间处理未完成的请求
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("服务器关闭错误: %v", err)
		}
	}()

	// 9. 启动服务器
	log.Printf("服务器启动在 %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("服务器启动失败: %v", err)
	}

	log.Println("服务器已关闭")
}
