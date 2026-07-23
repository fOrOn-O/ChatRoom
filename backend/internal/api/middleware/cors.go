package middleware

import (
	"github.com/gin-gonic/gin"
)

// CORS 跨域中间件
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 修复 #7: 限制来源为前端域名
		origin := c.Request.Header.Get("Origin")
		allowedOrigins := map[string]bool{
			"http://localhost:5173": true, // Vite 开发服务器
			"http://localhost:3000": true, // 备用端口
		}

		if allowedOrigins[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
		} else {
			// 开发环境允许所有，生产环境应该严格限制
			c.Header("Access-Control-Allow-Origin", "http://localhost:5173")
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
