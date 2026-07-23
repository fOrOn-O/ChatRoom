package model

import (
	"time"

	"gorm.io/gorm"
)

// Group 群组模型
type Group struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	Name         string         `json:"name" gorm:"size:100;not null"`
	OwnerID      uint           `json:"owner_id" gorm:"not null;index"`
	Avatar       string         `json:"avatar" gorm:"size:255"`
	Description  string         `json:"description" gorm:"size:500"`
	Announcement string         `json:"announcement" gorm:"type:text"`
	MaxMembers   int            `json:"max_members" gorm:"default:500"`
	Status       int            `json:"status" gorm:"default:1;not null"` // 0=已解散 1=正常
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 表名
func (Group) TableName() string {
	return "groups"
}
