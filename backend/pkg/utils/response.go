package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// Error 错误响应
func Error(c *gin.Context, code int, httpStatus int, message string) {
	c.JSON(httpStatus, Response{
		Code:    code,
		Message: message,
	})
}

// BadRequest 参数错误
func BadRequest(c *gin.Context, message string) {
	Error(c, 1001, http.StatusBadRequest, message)
}

// Unauthorized 未授权
func Unauthorized(c *gin.Context, message string) {
	Error(c, 1002, http.StatusUnauthorized, message)
}

// Forbidden 禁止访问
func Forbidden(c *gin.Context, message string) {
	Error(c, 1003, http.StatusForbidden, message)
}

// NotFound 资源不存在
func NotFound(c *gin.Context, message string) {
	Error(c, 1004, http.StatusNotFound, message)
}

// ServerError 服务器错误
func ServerError(c *gin.Context, message string) {
	Error(c, 1005, http.StatusInternalServerError, message)
}
