package api

import (
	"net/http"

	"ChatRoom/internal/api/handler"
	"ChatRoom/internal/api/middleware"
	"ChatRoom/internal/ws"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Router 路由管理器
type Router struct {
	engine *gin.Engine
	hub    *ws.Hub
	db     *gorm.DB
	rdb    *redis.Client
	secret string
}

// NewRouter 创建新的路由
func NewRouter(hub *ws.Hub, db *gorm.DB, rdb *redis.Client, jwtSecret string) *gin.Engine {
	r := &Router{
		engine: gin.Default(),
		hub:    hub,
		db:     db,
		rdb:    rdb,
		secret: jwtSecret,
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
			authHandler := handler.NewAuthHandler(r.db, r.rdb, r.secret)
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
		}

		// 需要登录的接口
		authorized := v1.Group("")
		authorized.Use(middleware.Auth(r.secret))
		{
			// 用户相关
			userHandler := handler.NewUserHandler(r.db, r.rdb)
			user := authorized.Group("/user")
			{
				user.GET("/profile", userHandler.GetProfile)
				user.PUT("/profile", userHandler.UpdateProfile)
			}
			authorized.GET("/users/search", userHandler.SearchUsers)

			// 好友相关
			friendHandler := handler.NewFriendHandler(r.db, r.rdb)
			friend := authorized.Group("/friends")
			{
				friend.GET("", friendHandler.GetFriends)
				friend.POST("/request", friendHandler.SendRequest)
				friend.POST("/handle", friendHandler.HandleRequest)
				friend.DELETE("/:friend_id", friendHandler.DeleteFriend)
			}

			// 群组相关
			groupHandler := handler.NewGroupHandler(r.db, r.rdb)
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
			messageHandler := handler.NewMessageHandler(r.db, r.rdb)
			message := authorized.Group("/messages")
			{
				message.GET("", messageHandler.GetHistory)
				message.POST("/read", messageHandler.MarkRead)
				message.POST("/:msg_id/revoke", messageHandler.RevokeMessage)
			}

			// 文件相关
			fileHandler := handler.NewFileHandler(r.db, r.rdb)
			file := authorized.Group("/files")
			{
				file.POST("/upload", fileHandler.Upload)
				file.GET("/:file_id/download", fileHandler.Download)
			}
		}
	}

	// WebSocket（需要登录）
	r.engine.GET("/ws", middleware.Auth(r.secret), func(c *gin.Context) {
		handler.HandleWebSocket(c, r.hub, r.db)
	})

	// 静态文件（前端）
	r.engine.Static("/static", "./storage")
	r.engine.StaticFile("/", "./web/index.html")
}
