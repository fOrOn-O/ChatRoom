package handler

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"ChatRoom/internal/api/middleware"
	"ChatRoom/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 统一业务错误
var (
	errGroupNotAccessible    = errors.New("group not accessible")
	errGroupPermissionDenied = errors.New("group permission denied")
	errGroupMemberNotFound   = errors.New("group member not found")
	errGroupOwnerCannotLeave = errors.New("group owner cannot leave")
	errGroupMemberLimit      = errors.New("group member limit reached")
	errInvalidInviteUsers    = errors.New("invalid invite users")
	errUseLeaveGroupEndpoint = errors.New("use leave group endpoint")
)

// 统一访问结果结构
type groupAccess struct {
	Group      model.Group
	Membership model.GroupMember
}

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
		{GroupID: group.ID, UserID: userID, Role: model.GroupRoleOwner}, // 2 = 群主
	}

	// 添加其他成员
	for _, memberID := range req.MemberIDs {
		if memberID != userID {
			members = append(members, model.GroupMember{
				GroupID: group.ID,
				UserID:  memberID,
				Role:    model.GroupRoleMember, // 0 = 普通成员
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
	currentUserID := middleware.GetUserID(c)

	var groups []model.Group

	err := h.db.WithContext(c.Request.Context()).
		Table("`groups` AS g").
		Select("g.*").
		Joins("JOIN group_members AS gm ON gm.group_id = g.id").
		Where(
			"gm.user_id = ? AND gm.status = ? AND g.status = ?",
			currentUserID,
			model.GroupMemberStatusActive,
			model.GroupStatusActive,
		).
		Order("g.updated_at DESC").
		Scan(&groups).Error

	if err != nil {
		respondGroupError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    groups,
	})
}

// GetGroup 获取群组详情
func (h *GroupHandler) GetGroup(c *gin.Context) {
	groupID, ok := parseUintParam(c, "group_id")
	if !ok {
		return
	}

	currentUserID := middleware.GetUserID(c)

	access, err := loadGroupAccess(
		h.db.WithContext(c.Request.Context()),
		groupID,
		currentUserID,
		false,
	)
	if err != nil {
		respondGroupError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    access.Group,
	})
}

type groupMemberResponse struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Role     int    `json:"role"`
}

// GetMembers 获取群成员列表
func (h *GroupHandler) GetMembers(c *gin.Context) {
	groupID, ok := parseUintParam(c, "group_id")
	if !ok {
		return
	}

	currentUserID := middleware.GetUserID(c)

	_, err := loadGroupAccess(
		h.db.WithContext(c.Request.Context()),
		groupID,
		currentUserID,
		false,
	)
	if err != nil {
		respondGroupError(c, err)
		return
	}

	var members []groupMemberResponse

	err = h.db.WithContext(c.Request.Context()).
		Table("group_members AS gm").
		Select(`
			u.id,
			u.username,
			CASE
				WHEN gm.nickname <> '' THEN gm.nickname
				ELSE u.nickname
			END AS nickname,
			u.avatar,
			gm.role
		`).
		Joins("JOIN users AS u ON u.id = gm.user_id").
		Where(
			"gm.group_id = ? AND gm.status = ? AND u.status = ?",
			groupID,
			model.GroupMemberStatusActive,
			model.UserStatusActive,
		).
		Order("gm.role DESC, gm.joined_at ASC").
		Scan(&members).Error

	if err != nil {
		respondGroupError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    members,
	})
}

// InviteMembersRequest 邀请成员请求
type InviteMembersRequest struct {
	UserIDs []uint `json:"user_ids" binding:"required,min=1,max=100,dive,gt=0"`
}

// InviteMembers 邀请成员
func (h *GroupHandler) InviteMembers(c *gin.Context) {
	groupID, ok := parseUintParam(c, "group_id")
	if !ok {
		return
	}

	currentUserID := middleware.GetUserID(c)

	var req InviteMembersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1001,
			"message": "邀请成员参数错误",
		})
		return
	}

	userIDs := uniqueUintIDs(req.UserIDs)
	if len(userIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1001,
			"message": "邀请用户不能为空",
		})
		return
	}

	addedCount := 0

	err := h.db.WithContext(c.Request.Context()).Transaction(
		func(tx *gorm.DB) error {
			access, err := loadGroupAccess(
				tx,
				groupID,
				currentUserID,
				true,
			)
			if err != nil {
				return err
			}

			if !canInviteMembers(access.Membership.Role) {
				return errGroupPermissionDenied
			}

			// 只允许邀请存在且状态正常的用户。
			var validUserIDs []uint

			if err := tx.Model(&model.User{}).
				Where(
					"id IN ? AND status = ?",
					userIDs,
					model.UserStatusActive,
				).
				Pluck("id", &validUserIDs).Error; err != nil {
				return err
			}

			if len(validUserIDs) != len(userIDs) {
				return errInvalidInviteUsers
			}

			// 查询已经在群里的用户。
			var existingUserIDs []uint

			if err := tx.Model(&model.GroupMember{}).
				Where(
					"group_id = ? AND user_id IN ? AND status = ?",
					groupID,
					userIDs,
					model.GroupMemberStatusActive,
				).
				Pluck("user_id", &existingUserIDs).Error; err != nil {
				return err
			}

			existingSet := make(
				map[uint]struct{},
				len(existingUserIDs),
			)

			for _, userID := range existingUserIDs {
				existingSet[userID] = struct{}{}
			}

			newUserIDs := make([]uint, 0, len(userIDs))

			for _, userID := range userIDs {
				if _, exists := existingSet[userID]; exists {
					continue
				}

				newUserIDs = append(newUserIDs, userID)
			}

			if len(newUserIDs) == 0 {
				addedCount = 0
				return nil
			}

			// 群记录已经通过 FOR UPDATE 锁定，
			// 相同群组的并发邀请会串行检查成员数量。
			var activeMemberCount int64

			if err := tx.Model(&model.GroupMember{}).
				Where(
					"group_id = ? AND status = ?",
					groupID,
					model.GroupMemberStatusActive,
				).
				Count(&activeMemberCount).Error; err != nil {
				return err
			}

			if activeMemberCount+int64(len(newUserIDs)) >
				int64(access.Group.MaxMembers) {
				return errGroupMemberLimit
			}

			now := time.Now()
			members := make(
				[]model.GroupMember,
				0,
				len(newUserIDs),
			)

			for _, userID := range newUserIDs {
				members = append(members, model.GroupMember{
					GroupID:  groupID,
					UserID:   userID,
					Role:     model.GroupRoleMember,
					Status:   model.GroupMemberStatusActive,
					JoinedAt: now,
				})
			}

			// 如果数据库中存在 status=0 的旧记录，
			// 利用组合唯一索引重新激活，避免重复键错误。
			err = tx.Clauses(
				clause.OnConflict{
					Columns: []clause.Column{
						{Name: "group_id"},
						{Name: "user_id"},
					},
					DoUpdates: clause.Assignments(
						map[string]interface{}{
							"role":      model.GroupRoleMember,
							"status":    model.GroupMemberStatusActive,
							"joined_at": now,
						},
					),
				},
			).Create(&members).Error

			if err != nil {
				return err
			}

			addedCount = len(members)
			return nil
		},
	)

	if err != nil {
		respondGroupError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"added_count": addedCount,
		},
	})
}

// RemoveMember 移除群成员
func (h *GroupHandler) RemoveMember(c *gin.Context) {
	groupID, ok := parseUintParam(c, "group_id")
	if !ok {
		return
	}

	targetUserID, ok := parseUintParam(c, "user_id")
	if !ok {
		return
	}

	currentUserID := middleware.GetUserID(c)

	if currentUserID == targetUserID {
		respondGroupError(c, errUseLeaveGroupEndpoint)
		return
	}

	err := h.db.WithContext(c.Request.Context()).Transaction(
		func(tx *gorm.DB) error {
			access, err := loadGroupAccess(
				tx,
				groupID,
				currentUserID,
				true,
			)
			if err != nil {
				return err
			}

			targetMember, err := loadActiveGroupMember(
				tx,
				groupID,
				targetUserID,
				true,
			)
			if err != nil {
				return err
			}

			if !canRemoveMember(
				access.Group.OwnerID,
				currentUserID,
				access.Membership.Role,
				targetUserID,
				targetMember.Role,
			) {
				return errGroupPermissionDenied
			}

			result := tx.
				Where(
					"group_id = ? AND user_id = ? AND status = ?",
					groupID,
					targetUserID,
					model.GroupMemberStatusActive,
				).
				Delete(&model.GroupMember{})

			if result.Error != nil {
				return result.Error
			}

			if result.RowsAffected != 1 {
				return errGroupMemberNotFound
			}

			return nil
		},
	)

	if err != nil {
		respondGroupError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// LeaveGroup 退出群组
func (h *GroupHandler) LeaveGroup(c *gin.Context) {
	groupID, ok := parseUintParam(c, "group_id")
	if !ok {
		return
	}

	currentUserID := middleware.GetUserID(c)

	err := h.db.WithContext(c.Request.Context()).Transaction(
		func(tx *gorm.DB) error {
			access, err := loadGroupAccess(
				tx,
				groupID,
				currentUserID,
				true,
			)
			if err != nil {
				return err
			}

			if access.Group.OwnerID == currentUserID ||
				access.Membership.Role == model.GroupRoleOwner {
				return errGroupOwnerCannotLeave
			}

			result := tx.
				Where(
					"group_id = ? AND user_id = ? AND status = ?",
					groupID,
					currentUserID,
					model.GroupMemberStatusActive,
				).
				Delete(&model.GroupMember{})

			if result.Error != nil {
				return result.Error
			}

			if result.RowsAffected != 1 {
				return errGroupNotAccessible
			}

			return nil
		},
	)

	if err != nil {
		respondGroupError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// 统一路径解析
func parseUintParam(c *gin.Context, name string) (uint, bool) {
	rawValue := c.Param(name)

	value, err := strconv.ParseUint(rawValue, 10, 32)
	if err != nil || value == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1001,
			"message": "参数错误",
		})
		return 0, false
	}

	return uint(value), true
}

// 统一群访问检查
func loadGroupAccess(db *gorm.DB, groupID uint, userID uint, forUpdate bool) (*groupAccess, error) {
	var group model.Group

	groupQuery := db.Where(
		"id = ? AND status = ?",
		groupID,
		model.GroupStatusActive,
	)

	if forUpdate {
		groupQuery = groupQuery.Clauses(
			clause.Locking{Strength: "UPDATE"},
		)
	}

	if err := groupQuery.First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errGroupNotAccessible
		}
		return nil, err
	}

	var membership model.GroupMember

	memberQuery := db.Where(
		"group_id = ? AND user_id = ? AND status = ?",
		groupID,
		userID,
		model.GroupMemberStatusActive,
	)

	if forUpdate {
		memberQuery = memberQuery.Clauses(
			clause.Locking{Strength: "UPDATE"},
		)
	}

	if err := memberQuery.First(&membership).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errGroupNotAccessible
		}
		return nil, err
	}

	return &groupAccess{
		Group:      group,
		Membership: membership,
	}, nil
}

// loadActiveGroupMember目标成员查询
func loadActiveGroupMember(db *gorm.DB, groupID uint, userID uint, forUpdate bool) (*model.GroupMember, error) {
	var member model.GroupMember

	query := db.Where("group_id = ? AND user_id = ? AND status = ?", groupID, userID, model.GroupMemberStatusActive)

	if forUpdate {
		query = query.Clauses(
			clause.Locking{Strength: "UPDATE"},
		)
	}

	if err := query.First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errGroupMemberNotFound
		}
		return nil, err
	}

	return &member, nil
}

// 邀请权限判断
func canInviteMembers(role int) bool {
	return role == model.GroupRoleAdmin || role == model.GroupRoleOwner
}

// 移除成员权限判断
func canRemoveMember(
	groupOwnerID uint,
	actorID uint,
	actorRole int,
	targetID uint,
	targetRole int,
) bool {
	// 自己退出必须调用 LeaveGroup
	if actorID == targetID {
		return false
	}

	// 群主永远不能通过移除成员接口被删除
	if targetID == groupOwnerID ||
		targetRole == model.GroupRoleOwner {
		return false
	}

	// 群主可以移除普通成员和管理员
	if actorID == groupOwnerID &&
		actorRole == model.GroupRoleOwner {
		return true
	}

	// 管理员只能移除普通成员
	if actorRole == model.GroupRoleAdmin &&
		targetRole == model.GroupRoleMember {
		return true
	}

	return false
}

// 统一错误响应
func respondGroupError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errGroupNotAccessible):
		c.JSON(http.StatusNotFound, gin.H{
			"code":    3001,
			"message": "群组不存在或无权访问",
		})

	case errors.Is(err, errGroupPermissionDenied):
		c.JSON(http.StatusForbidden, gin.H{
			"code":    1003,
			"message": "无权限执行该操作",
		})

	case errors.Is(err, errGroupMemberNotFound):
		c.JSON(http.StatusNotFound, gin.H{
			"code":    3002,
			"message": "群成员不存在",
		})

	case errors.Is(err, errGroupOwnerCannotLeave):
		c.JSON(http.StatusConflict, gin.H{
			"code":    3004,
			"message": "群主需要先转让群主或解散群组",
		})

	case errors.Is(err, errGroupMemberLimit):
		c.JSON(http.StatusConflict, gin.H{
			"code":    3005,
			"message": "群成员数量已达到上限",
		})

	case errors.Is(err, errInvalidInviteUsers):
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1001,
			"message": "邀请列表中包含无效用户",
		})

	case errors.Is(err, errUseLeaveGroupEndpoint):
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1001,
			"message": "不能通过移除成员接口退出群组",
		})

	default:
		log.Printf("群组操作失败: %v", err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1005,
			"message": "服务器内部错误",
		})
	}
}

// 用户 ID 去重
func uniqueUintIDs(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	result := make([]uint, 0, len(ids))

	for _, id := range ids {
		if id == 0 {
			continue
		}

		if _, exists := seen[id]; exists {
			continue
		}

		seen[id] = struct{}{}
		result = append(result, id)
	}

	return result
}
