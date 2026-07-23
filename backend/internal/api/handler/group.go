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

// GroupHandler 群组处理器
type GroupHandler struct {
	db  *gorm.DB
	rdb *redis.Client
}

// NewGroupHandler 创建群组处理器
func NewGroupHandler(db *gorm.DB, rdb *redis.Client) *GroupHandler {
	return &GroupHandler{
		db:  db,
		rdb: rdb,
	}
}

// CreateGroupRequest 创建群组请求
type CreateGroupRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=100"`
	Description string `json:"description"`
	MemberIDs   []uint `json:"member_ids"`
}

// CreateGroup 创建群组
func (h *GroupHandler) CreateGroup(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1001,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 使用事务创建群组
	tx := h.db.Begin()

	// 创建群组
	group := &model.Group{
		Name:        req.Name,
		OwnerID:     userID,
		Description: req.Description,
	}

	if err := tx.Create(group).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1005,
			"message": "创建群组失败",
		})
		return
	}

	// 添加创建者为群主
	members := []model.GroupMember{
		{GroupID: group.ID, UserID: userID, Role: 2}, // 2 = 群主
	}

	// 添加其他成员
	for _, memberID := range req.MemberIDs {
		if memberID != userID {
			members = append(members, model.GroupMember{
				GroupID: group.ID,
				UserID:  memberID,
				Role:    0, // 0 = 普通成员
			})
		}
	}

	if err := tx.Create(&members).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1005,
			"message": "添加成员失败",
		})
		return
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"id":           group.ID,
			"name":         group.Name,
			"owner_id":     group.OwnerID,
			"description":  group.Description,
			"member_count": len(members),
		},
	})
}

// GetGroups 获取用户的群组列表
func (h *GroupHandler) GetGroups(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var members []model.GroupMember
	h.db.Where("user_id = ?", userID).Find(&members)

	var groupIDs []uint
	for _, m := range members {
		groupIDs = append(groupIDs, m.GroupID)
	}

	var groups []model.Group
	h.db.Where("id IN ?", groupIDs).Find(&groups)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    groups,
	})
}

// GetGroup 获取群组详情
func (h *GroupHandler) GetGroup(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("group_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1001,
			"message": "参数错误",
		})
		return
	}

	var group model.Group
	if err := h.db.First(&group, groupID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    3001,
			"message": "群组不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    group,
	})
}

// GetMembers 获取群成员列表
func (h *GroupHandler) GetMembers(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("group_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1001,
			"message": "参数错误",
		})
		return
	}

	var members []model.GroupMember
	h.db.Where("group_id = ?", groupID).Find(&members)

	var result []gin.H
	for _, m := range members {
		var user model.User
		h.db.First(&user, m.UserID)

		result = append(result, gin.H{
			"id":       user.ID,
			"username": user.Username,
			"nickname": user.Nickname,
			"avatar":   user.Avatar,
			"role":     m.Role,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// InviteMembersRequest 邀请成员请求
type InviteMembersRequest struct {
	UserIDs []uint `json:"user_ids" binding:"required"`
}

// InviteMembers 邀请成员
func (h *GroupHandler) InviteMembers(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("group_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1001,
			"message": "参数错误",
		})
		return
	}

	var req InviteMembersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1001,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	var members []model.GroupMember
	for _, userID := range req.UserIDs {
		members = append(members, model.GroupMember{
			GroupID: uint(groupID),
			UserID:  userID,
			Role:    0,
		})
	}

	if err := h.db.Create(&members).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1005,
			"message": "邀请成员失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// RemoveMember 移除群成员
func (h *GroupHandler) RemoveMember(c *gin.Context) {
	groupID, _ := strconv.ParseUint(c.Param("group_id"), 10, 32)
	userID, _ := strconv.ParseUint(c.Param("user_id"), 10, 32)

	h.db.Where("group_id = ? AND user_id = ?", groupID, userID).Delete(&model.GroupMember{})

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// LeaveGroup 退出群组
func (h *GroupHandler) LeaveGroup(c *gin.Context) {
	userID := middleware.GetUserID(c)
	groupID, _ := strconv.ParseUint(c.Param("group_id"), 10, 32)

	h.db.Where("group_id = ? AND user_id = ?", groupID, userID).Delete(&model.GroupMember{})

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}
