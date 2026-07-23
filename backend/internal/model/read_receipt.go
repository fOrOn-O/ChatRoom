package model

import "time"

// ReadReceipt 已读回执模型
type ReadReceipt struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	UserID     uint      `json:"user_id" gorm:"not null;index"`
	TargetID   uint      `json:"target_id" gorm:"not null;index"`
	TargetType string    `json:"target_type" gorm:"size:10;not null"` // user 或 group
	LastReadMsg string   `json:"last_read_msg" gorm:"size:36;not null"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TableName 表名
func (ReadReceipt) TableName() string {
	return "read_receipts"
}
