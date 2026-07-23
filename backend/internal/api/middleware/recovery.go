package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// Recovery 恢复中间件
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 打印堆栈信息
				log.Printf("[Recovery] panic recovered: %v\n%s", err, debug.Stack())

				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    1005,
					"message": "服务器内部错误",
				})
				c.Abort()
			}
		}()

		c.Next()
	}
}
