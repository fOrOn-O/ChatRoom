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

// FriendHandler 好友处理器
type FriendHandler struct {
	db  *gorm.DB
	rdb *redis.Client
}

// NewFriendHandler 创建好友处理器
func NewFriendHandler(db *gorm.DB, rdb *redis.Client) *FriendHandler {
	return &FriendHandler{
		db:  db,
		rdb: rdb,
	}
}

// SendRequestRequest 发送好友请求
type SendRequestRequest struct {
	UserID uint   `json:"user_id" binding:"required"`
	Remark string `json:"remark"`
}

// SendRequest 发送好友请求
func (h *FriendHandler) SendRequest(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req SendRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1001,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 不能添加自己
	if userID == req.UserID {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1001,
			"message": "不能添加自己为好友",
		})
		return
	}

	// 检查目标用户是否存在
	var targetUser model.User
	if err := h.db.First(&targetUser, req.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    1004,
			"message": "用户不存在",
		})
		return
	}

	// 检查是否已经是好友
	var count int64
	h.db.Model(&model.Friend{}).Where("user_id = ? AND friend_id = ?", userID, req.UserID).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1001,
			"message": "已经是好友",
		})
		return
	}

	// 创建好友关系（双向）
	friends := []model.Friend{
		{UserID: userID, FriendID: req.UserID, Remark: req.Remark},
		{UserID: req.UserID, FriendID: userID, Remark: ""},
	}

	if err := h.db.Create(&friends).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1005,
			"message": "添加好友失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// HandleRequestRequest 处理好友请求
type HandleRequestRequest struct {
	RequestID uint   `json:"request_id" binding:"required"`
	Action    string `json:"action" binding:"required,oneof=accept reject"`
}

// HandleRequest 处理好友请求
func (h *FriendHandler) HandleRequest(c *gin.Context) {
	var req HandleRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1001,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// TODO: 实现好友请求处理逻辑
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// GetFriends 获取好友列表
func (h *FriendHandler) GetFriends(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var friends []model.Friend
	h.db.Where("user_id = ?", userID).Find(&friends)

	// 获取好友详细信息
	var result []gin.H
	for _, friend := range friends {
		var user model.User
		h.db.First(&user, friend.FriendID)

		result = append(result, gin.H{
			"id":       user.ID,
			"username": user.Username,
			"nickname": user.Nickname,
			"avatar":   user.Avatar,
			"remark":   friend.Remark,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// DeleteFriend 删除好友
func (h *FriendHandler) DeleteFriend(c *gin.Context) {
	userID := middleware.GetUserID(c)

	friendID, err := strconv.ParseUint(c.Param("friend_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1001,
			"message": "参数错误",
		})
		return
	}

	// 删除双向好友关系
	h.db.Where("(user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)",
		userID, friendID, friendID, userID).Delete(&model.Friend{})

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}
