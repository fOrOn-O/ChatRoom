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
			"msg_id":        msg.MsgID,
			"from_user_id":  msg.FromUserID,
			"to_id":         msg.ToID,
			"to_type":       msg.ToType,
			"content_type":  msg.ContentType,
			"content":       msg.Content,
			"extra":         msg.Extra,
			"status":        msg.Status,
			"created_at":    msg.CreatedAt,
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

// MarkReadRequest 标记已读请求
type MarkReadRequest struct {
	TargetID   uint   `json:"target_id" binding:"required"`
	TargetType string `json:"target_type" binding:"required"`
	LastMsgID  string `json:"last_msg_id" binding:"required"`
}

// MarkRead 标记消息已读
func (h *MessageHandler) MarkRead(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req MarkReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1001,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 更新已读回执
	readReceipt := &model.ReadReceipt{
		UserID:      userID,
		TargetID:    req.TargetID,
		TargetType:  req.TargetType,
		LastReadMsg: req.LastMsgID,
	}

	// 使用 Upsert 模式
	h.db.Where("user_id = ? AND target_id = ? AND target_type = ?", userID, req.TargetID, req.TargetType).
		Assign(map[string]interface{}{"last_read_msg": req.LastMsgID}).
		FirstOrCreate(readReceipt)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// RevokeMessage 撤回消息
func (h *MessageHandler) RevokeMessage(c *gin.Context) {
	userID := middleware.GetUserID(c)
	msgID := c.Param("msg_id")

	var message model.Message
	if err := h.db.Where("msg_id = ?", msgID).First(&message).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    1004,
			"message": "消息不存在",
		})
		return
	}

	// 检查是否是发送者
	if message.FromUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    1003,
			"message": "只能撤回自己发送的消息",
		})
		return
	}

	// 检查时间限制（2分钟内）
	// TODO: 实现时间检查

	// 更新消息状态
	h.db.Model(&message).Update("status", 2) // 2 = 已撤回

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}
