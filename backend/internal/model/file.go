package model

import "time"

// File 文件模型
type File struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	Filename   string    `json:"filename" gorm:"size:255;not null"`
	Filepath   string    `json:"filepath" gorm:"size:500;not null"`
	Filesize   int64     `json:"filesize" gorm:"not null"`
	Filetype   string    `json:"filetype" gorm:"size:50;not null"`
	Mimetype   string    `json:"mimetype" gorm:"size:100;not null"`
	MD5        string    `json:"md5" gorm:"size:32;not null;index"`
	UploaderID uint      `json:"uploader_id" gorm:"not null;index"`
	CreatedAt  time.Time `json:"created_at"`
}

// TableName 表名
func (File) TableName() string {
	return "files"
}
