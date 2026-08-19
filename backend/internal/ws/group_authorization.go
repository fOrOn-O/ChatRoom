package ws

import (
	"context"
	"errors"
	"fmt"

	"ChatRoom/internal/model"

	"gorm.io/gorm"
)

const (
	errorCodeGroupMessageForbidden = 3001
	errorCodeInternal              = 1005
)

var (
	errGroupMessageForbidden         = errors.New("group does not exist or sender is not an active member")
	errGroupAuthorizationUnavailable = errors.New("group message authorization unavailable")
)

type groupMessageAuthorizer func(ctx context.Context, userID uint, groupID uint) error

type groupMessageAccess struct {
	GroupStatus  int `gorm:"column:group_status"`
	MemberStatus int `gorm:"column:member_status"`
}

func newGroupMessageAuthorizer(db *gorm.DB) groupMessageAuthorizer {
	return func(ctx context.Context, userID uint, groupID uint) error {
		if db == nil {
			return errGroupAuthorizationUnavailable
		}

		var access groupMessageAccess
		err := db.WithContext(ctx).
			Table("`groups` AS g").
			Select("g.status AS group_status, gm.status AS member_status").
			Joins("JOIN group_members AS gm ON gm.group_id = g.id AND gm.user_id = ?", userID).
			Where("g.id = ? AND g.deleted_at IS NULL", groupID).
			Take(&access).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errGroupMessageForbidden
		}
		if err != nil {
			return fmt.Errorf("%w: %v", errGroupAuthorizationUnavailable, err)
		}

		return authorizeGroupMessageAccess(access)
	}
}

func authorizeGroupMessageAccess(access groupMessageAccess) error {
	if access.GroupStatus != model.GroupStatusActive || access.MemberStatus != model.GroupMemberStatusActive {
		return errGroupMessageForbidden
	}
	return nil
}

func newGroupAuthorizationErrorMessage(msgID string, err error) *Message {
	code := errorCodeInternal
	message := "群消息权限校验失败，请稍后重试"
	if errors.Is(err, errGroupMessageForbidden) {
		code = errorCodeGroupMessageForbidden
		message = "群组不存在或无权发送消息"
	}

	return &Message{
		Type: MsgTypeError,
		Data: map[string]interface{}{
			"code":    code,
			"message": message,
			"msg_id":  msgID,
		},
	}
}
