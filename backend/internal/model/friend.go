package model

import "time"

// Friend 好友关系模型
type Friend struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"not null;index"`
	FriendID  uint      `json:"friend_id" gorm:"not null;index"`
	Remark    string    `json:"remark" gorm:"size:50"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName 表名
func (Friend) TableName() string {
	return "friends"
}
