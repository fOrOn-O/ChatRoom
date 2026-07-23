package middleware

import (
	"net/http"
	"strings"

	"ChatRoom/pkg/auth"

	"github.com/gin-gonic/gin"
)

// Auth JWT 认证中间件
func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取 Token
		token := c.GetHeader("Authorization")
		if token == "" {
			// 尝试从查询参数获取（WebSocket 连接）
			token = c.Query("token")
		}

		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    1002,
				"message": "未授权：缺少 Token",
			})
			c.Abort()
			return
		}

		// 移除 Bearer 前缀
		if strings.HasPrefix(token, "Bearer ") {
			token = strings.TrimPrefix(token, "Bearer ")
		}

		// 解析 Token
		claims, err := auth.ParseToken(token, secret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    1002,
				"message": "未授权：Token 无效",
			})
			c.Abort()
			return
		}

		// 将用户信息存储到上下文
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)

		c.Next()
	}
}

// GetUserID 从上下文获取用户ID
func GetUserID(c *gin.Context) uint {
	if userID, exists := c.Get("user_id"); exists {
		return userID.(uint)
	}
	return 0
}

// GetUsername 从上下文获取用户名
func GetUsername(c *gin.Context) string {
	if username, exists := c.Get("username"); exists {
		return username.(string)
	}
	return ""
}
