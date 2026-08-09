package model

import (
	"time"

	"gorm.io/gorm"
)

// 用户状态常量
const (
	UserStatusDisabled = 0
	UserStatusActive   = 1
)

// User 用户模型
type User struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Username  string         `json:"username" gorm:"uniqueIndex;size:50;not null"`
	Password  string         `json:"-" gorm:"size:255;not null"`
	Nickname  string         `json:"nickname" gorm:"size:50;not null"`
	Avatar    string         `json:"avatar" gorm:"size:255"`
	Email     string         `json:"email" gorm:"uniqueIndex;size:100"`
	Phone     string         `json:"phone" gorm:"size:20"`
	Signature string         `json:"signature" gorm:"size:255"`
	Status    int            `json:"status" gorm:"default:1;not null"` // 0=禁用 1=正常
	LastLogin *time.Time     `json:"last_login"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 表名
func (User) TableName() string {
	return "users"
}
