package model

import (
	"time"
)

// Message 消息模型
type Message struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	MsgID       string    `json:"msg_id" gorm:"uniqueIndex;size:36;not null"`
	FromUserID  uint      `json:"from_user_id" gorm:"not null;index"`
	ToID        uint      `json:"to_id" gorm:"not null;index"`
	ToType      string    `json:"to_type" gorm:"size:10;not null;index"` // user 或 group
	ContentType string    `json:"content_type" gorm:"size:20;not null;default:text"`
	Content     string    `json:"content" gorm:"type:text;not null"`
	Extra       string    `json:"extra" gorm:"type:json"` // 扩展信息
	Status      int       `json:"status" gorm:"default:1;not null"` // 0=已删除 1=正常 2=已撤回
	CreatedAt   time.Time `json:"created_at"`
}

// TableName 表名
func (Message) TableName() string {
	return "messages"
}
