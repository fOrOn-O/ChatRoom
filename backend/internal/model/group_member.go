package model

import "time"

// GroupMember 群成员模型
type GroupMember struct {
	ID       uint      `json:"id" gorm:"primaryKey"`
	GroupID  uint      `json:"group_id" gorm:"not null;index"`
	UserID   uint      `json:"user_id" gorm:"not null;index"`
	Nickname string    `json:"nickname" gorm:"size:50"` // 群内昵称
	Role     int       `json:"role" gorm:"default:0;not null"` // 0=普通成员 1=管理员 2=群主
	Status   int       `json:"status" gorm:"default:1;not null"` // 0=已退出 1=正常
	JoinedAt time.Time `json:"joined_at"`
}

// TableName 表名
func (GroupMember) TableName() string {
	return "group_members"
}
