package ws

import (
	"context"
	"errors"
	"fmt"

	"ChatRoom/internal/model"

	"gorm.io/gorm"
)

var errGroupRecipientResolutionUnavailable = errors.New("group recipient resolution unavailable")

type groupRecipientResolver func(ctx context.Context, groupID uint) ([]uint, error)

func newGroupRecipientResolver(db *gorm.DB) groupRecipientResolver {
	return func(ctx context.Context, groupID uint) ([]uint, error) {
		if db == nil {
			return nil, errGroupRecipientResolutionUnavailable
		}

		var userIDs []uint
		err := db.WithContext(ctx).
			Table("group_members AS gm").
			Joins("JOIN `groups` AS g ON g.id = gm.group_id").
			Where(
				"gm.group_id = ? AND gm.status = ? AND g.status = ? AND g.deleted_at IS NULL",
				groupID,
				model.GroupMemberStatusActive,
				model.GroupStatusActive,
			).
			Pluck("gm.user_id", &userIDs).Error
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errGroupRecipientResolutionUnavailable, err)
		}

		return userIDs, nil
	}
}

func newGroupRecipientResolutionErrorMessage(msgID string) *Message {
	return &Message{
		Type: MsgTypeError,
		Data: map[string]interface{}{
			"code":    errorCodeInternal,
			"message": "群消息接收成员查询失败，请稍后重试",
			"msg_id":  msgID,
		},
	}
}
