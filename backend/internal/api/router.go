package api

import (
	"net/http"

	"ChatRoom/internal/api/handler"
	"ChatRoom/internal/api/middleware"
	"ChatRoom/internal/messagequeue"
	"ChatRoom/internal/ratelimit"
	"ChatRoom/internal/ws"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Router 路由管理器
type Router struct {
	engine         *gin.Engine
	hub            *ws.Hub
	db             *gorm.DB
	publisher      messagequeue.Publisher
	loginLimiter   ratelimit.LoginLimiter
	messageLimiter ratelimit.MessageLimiter
	secret         string
}

// NewRouter 创建新的路由
func NewRouter(
	hub *ws.Hub,
	db *gorm.DB,
	publisher messagequeue.Publisher,
	loginLimiter ratelimit.LoginLimiter,
	messageLimiter ratelimit.MessageLimiter,
	jwtSecret string,
) *gin.Engine {
	r := &Router{
		engine:         gin.Default(),
		hub:            hub,
		db:             db,
		publisher:      publisher,
		loginLimiter:   loginLimiter,
		messageLimiter: messageLimiter,
		secret:         jwtSecret,
	}

	r.setupMiddleware()
	r.setupRoutes()

	return r.engine
}

// setupMiddleware 设置中间件
func (r *Router) setupMiddleware() {
	// CORS 中间件
	r.engine.Use(middleware.CORS())

	// 日志中间件
	r.engine.Use(middleware.Logger())

	// 恢复中间件
	r.engine.Use(middleware.Recovery())
}

// setupRoutes 设置路由
func (r *Router) setupRoutes() {
	// 健康检查
	r.engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	// API v1
	v1 := r.engine.Group("/api/v1")
	{
		// 认证相关（不需要登录）
		auth := v1.Group("/auth")
		{
			authHandler := handler.NewAuthHandler(r.db, r.secret, r.loginLimiter)
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
		}

		// 需要登录的接口
		authorized := v1.Group("")
		authorized.Use(middleware.Auth(r.secret))
		{
			// 用户相关
			userHandler := handler.NewUserHandler(r.db)
			user := authorized.Group("/user")
			{
				user.GET("/profile", userHandler.GetProfile)
				user.PUT("/profile", userHandler.UpdateProfile)
			}
			authorized.GET("/users/search", userHandler.SearchUsers)

			// 好友相关
			friendHandler := handler.NewFriendHandler(r.db)
			friend := authorized.Group("/friends")
			{
				friend.GET("", friendHandler.GetFriends)
				friend.POST("/request", friendHandler.SendRequest)
				friend.POST("/handle", friendHandler.HandleRequest)
				friend.DELETE("/:friend_id", friendHandler.DeleteFriend)
			}

			// 群组相关
			groupHandler := handler.NewGroupHandler(r.db)
			group := authorized.Group("/groups")
			{
				group.GET("", groupHandler.GetGroups)
				group.POST("", groupHandler.CreateGroup)
				group.GET("/:group_id", groupHandler.GetGroup)
				group.GET("/:group_id/members", groupHandler.GetMembers)
				group.POST("/:group_id/members", groupHandler.InviteMembers)
				group.DELETE("/:group_id/members/:user_id", groupHandler.RemoveMember)
				group.POST("/:group_id/leave", groupHandler.LeaveGroup)
			}

			// 消息相关
			messageHandler := handler.NewMessageHandler(r.db)
			message := authorized.Group("/messages")
			{
				message.GET("", messageHandler.GetHistory)
			}

			// 文件相关
			fileHandler := handler.NewFileHandler(r.db)
			file := authorized.Group("/files")
			{
				file.POST("/upload", fileHandler.Upload)
				file.GET("/:file_id/download", fileHandler.Download)
			}
		}
	}

	// WebSocket（需要登录）
	r.engine.GET("/ws", middleware.Auth(r.secret), func(c *gin.Context) {
		handler.HandleWebSocket(c, r.hub, r.publisher, r.messageLimiter)
	})

	// 静态文件（前端）
	r.engine.Static("/static", "./storage")
	r.engine.StaticFile("/", "./web/index.html")
}
