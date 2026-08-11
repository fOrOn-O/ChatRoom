package handler

import (
	"net/http"
	"strconv"

	"ChatRoom/internal/api/middleware"
	"ChatRoom/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// MessageHandler 消息处理器
type MessageHandler struct {
	db  *gorm.DB
	rdb *redis.Client
}

// NewMessageHandler 创建消息处理器
func NewMessageHandler(db *gorm.DB, rdb *redis.Client) *MessageHandler {
	return &MessageHandler{
		db:  db,
		rdb: rdb,
	}
}

// GetHistory 获取历史消息
func (h *MessageHandler) GetHistory(c *gin.Context) {
	userID := middleware.GetUserID(c)

	targetID, _ := strconv.ParseUint(c.Query("target_id"), 10, 32)
	targetType := c.Query("target_type")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if targetID == 0 || targetType == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1001,
			"message": "参数错误",
		})
		return
	}

	// 权限检查
	if targetType == "group" {
		var count int64
		h.db.Model(&model.GroupMember{}).Where("group_id = ? AND user_id = ?", targetID, userID).Count(&count)
		if count == 0 {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    1003,
				"message": "无权限访问",
			})
			return
		}
	}

	// 查询消息
	var messages []model.Message
	var total int64

	query := h.db.Model(&model.Message{}).Where("to_id = ? AND to_type = ?", targetID, targetType)

	// 如果是私聊，还需要查询发送给自己的消息
	if targetType == "user" {
		query = h.db.Model(&model.Message{}).Where(
			"(from_user_id = ? AND to_id = ? AND to_type = 'user') OR (from_user_id = ? AND to_id = ? AND to_type = 'user')",
			userID, targetID, targetID, userID,
		)
	}

	query.Count(&total)
	query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&messages)

	// 转换为响应格式
	var result []gin.H
	for _, msg := range messages {
		result = append(result, gin.H{
			"msg_id":       msg.MsgID,
			"from_user_id": msg.FromUserID,
			"to_id":        msg.ToID,
			"to_type":      msg.ToType,
			"content_type": msg.ContentType,
			"content":      msg.Content,
			"extra":        msg.Extra,
			"created_at":   msg.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"total":     total,
			"page":      page,
			"page_size": pageSize,
			"list":      result,
		},
	})
}
