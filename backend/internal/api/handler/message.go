package handler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"

	"ChatRoom/internal/api/middleware"
	"ChatRoom/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var errGroupHistoryForbidden = errors.New("group history access forbidden")

type groupHistoryAccess struct {
	GroupStatus  int `gorm:"column:group_status"`
	MemberStatus int `gorm:"column:member_status"`
}

const (
	defaultHistoryPage     = 1
	defaultHistoryPageSize = 20
	maxHistoryPage         = 10_000
	maxHistoryPageSize     = 100
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

	targetID, targetIDErr := strconv.ParseUint(c.Query("target_id"), 10, 32)
	targetType := c.Query("target_type")

	if targetIDErr != nil || targetID == 0 || (targetType != "user" && targetType != "group") {
		respondMessageBadRequest(c)
		return
	}

	page, pageSize, ok := parseHistoryPagination(c)
	if !ok {
		return
	}
	afterID, incremental, ok := parseHistoryCursor(c)
	if !ok {
		return
	}
	if incremental && page != defaultHistoryPage {
		respondMessageBadRequest(c)
		return
	}

	// 权限检查
	if targetType == "group" {
		if err := authorizeGroupHistory(c.Request.Context(), h.db, uint(targetID), userID); err != nil {
			if !errors.Is(err, errGroupHistoryForbidden) {
				log.Printf("查询群历史访问权限失败: group_id=%d user_id=%d err=%v", targetID, userID, err)
				respondMessageInternalError(c)
				return
			}

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
	if incremental {
		query = query.Where("id > ?", afterID)
	}

	if err := query.Count(&total).Error; err != nil {
		log.Printf("统计历史消息失败: target_id=%d target_type=%s err=%v", targetID, targetType, err)
		respondMessageInternalError(c)
		return
	}

	var listQuery *gorm.DB
	if incremental {
		listQuery = query.Order("id ASC").Limit(pageSize)
	} else {
		listQuery = query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize)
	}
	if err := listQuery.Find(&messages).Error; err != nil {
		log.Printf("查询历史消息失败: target_id=%d target_type=%s err=%v", targetID, targetType, err)
		respondMessageInternalError(c)
		return
	}

	// 转换为响应格式
	var result []gin.H
	nextCursor := afterID
	for _, msg := range messages {
		if uint64(msg.ID) > nextCursor {
			nextCursor = uint64(msg.ID)
		}
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
			"total":       total,
			"page":        page,
			"page_size":   pageSize,
			"list":        result,
			"next_cursor": strconv.FormatUint(nextCursor, 10),
			"has_more":    incremental && total > int64(len(messages)),
		},
	})
}

func parseHistoryCursor(c *gin.Context) (uint64, bool, bool) {
	raw, exists := c.GetQuery("after_id")
	if !exists {
		return 0, false, true
	}

	afterID, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		respondMessageBadRequest(c)
		return 0, false, false
	}
	return afterID, true, true
}

func parseHistoryPagination(c *gin.Context) (int, int, bool) {
	page, err := strconv.Atoi(c.DefaultQuery("page", strconv.Itoa(defaultHistoryPage)))
	if err != nil || page < 1 || page > maxHistoryPage {
		respondMessageBadRequest(c)
		return 0, 0, false
	}

	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", strconv.Itoa(defaultHistoryPageSize)))
	if err != nil || pageSize < 1 || pageSize > maxHistoryPageSize {
		respondMessageBadRequest(c)
		return 0, 0, false
	}

	return page, pageSize, true
}

func respondMessageBadRequest(c *gin.Context) {
	c.JSON(http.StatusBadRequest, gin.H{
		"code":    1001,
		"message": "参数错误",
	})
}

func respondMessageInternalError(c *gin.Context) {
	c.JSON(http.StatusInternalServerError, gin.H{
		"code":    1005,
		"message": "服务器内部错误",
	})
}

func authorizeGroupHistory(ctx context.Context, db *gorm.DB, groupID uint, userID uint) error {
	var access groupHistoryAccess
	err := db.WithContext(ctx).
		Table("`groups` AS g").
		Select("g.status AS group_status, gm.status AS member_status").
		Joins("JOIN group_members AS gm ON gm.group_id = g.id AND gm.user_id = ?", userID).
		Where("g.id = ? AND g.deleted_at IS NULL", groupID).
		Take(&access).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errGroupHistoryForbidden
	}
	if err != nil {
		return err
	}
	if access.GroupStatus != model.GroupStatusActive || access.MemberStatus != model.GroupMemberStatusActive {
		return errGroupHistoryForbidden
	}

	return nil
}
