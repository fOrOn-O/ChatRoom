package handler

import (
	"log"
	"net/http"

	"ChatRoom/internal/api/middleware"
	"ChatRoom/internal/messagequeue"
	"ChatRoom/internal/ratelimit"
	"ChatRoom/internal/ws"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// 修复 #7: 限制来源
		return true // TODO: 生产环境应该限制为前端域名
	},
}

// HandleWebSocket 处理 WebSocket 连接
func HandleWebSocket(c *gin.Context, hub *ws.Hub, publisher messagequeue.Publisher, messageLimiter ratelimit.MessageLimiter) {
	// 从中间件获取用户信息
	userID := middleware.GetUserID(c)
	username := middleware.GetUsername(c)

	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    1002,
			"message": "未授权",
		})
		return
	}

	// 升级 HTTP 连接为 WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket 升级失败: %v", err)
		return
	}

	// 创建客户端
	client := ws.NewClient(conn, userID, username)

	// 注册客户端到 Hub
	if err := hub.Register(client); err != nil {
		log.Printf("WebSocket 注册失败: %v", err)
		client.CloseWithReason(websocket.CloseTryAgainLater, "server unavailable")
		return
	}

	// 启动读写 Goroutine
	go client.WritePump()
	go client.ReadPump(hub, publisher, messageLimiter)
}
