package handler

import (
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"ChatRoom/internal/api/middleware"
	"ChatRoom/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// FileHandler 文件处理器
type FileHandler struct {
	db  *gorm.DB
	rdb *redis.Client
}

// NewFileHandler 创建文件处理器
func NewFileHandler(db *gorm.DB, rdb *redis.Client) *FileHandler {
	return &FileHandler{
		db:  db,
		rdb: rdb,
	}
}

// Upload 上传文件
func (h *FileHandler) Upload(c *gin.Context) {
	userID := middleware.GetUserID(c)

	// 获取文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1001,
			"message": "请选择文件",
		})
		return
	}
	defer file.Close()

	// 检查文件大小（最大 50MB）
	if header.Size > 50*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    4004,
			"message": "文件大小超过限制",
		})
		return
	}

	// 计算文件 MD5
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1005,
			"message": "文件处理失败",
		})
		return
	}
	md5Str := fmt.Sprintf("%x", hash.Sum(nil))

	// 检查文件是否已存在
	var existingFile model.File
	if err := h.db.Where("md5 = ?", md5Str).First(&existingFile).Error; err == nil {
		// 文件已存在，直接返回
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"id":       existingFile.ID,
				"filename": existingFile.Filename,
				"url":      existingFile.Filepath,
				"filesize": existingFile.Filesize,
				"filetype": existingFile.Filetype,
				"mimetype": existingFile.Mimetype,
			},
		})
		return
	}

	// 移动文件指针到开头
	file.Seek(0, 0)

	// 生成存储路径
	ext := filepath.Ext(header.Filename)

	// 修复 #10: 无扩展名文件处理
	fileType := "unknown"
	if ext != "" {
		fileType = ext[1:] // 移除开头的点
	} else {
		// 无扩展名时使用 .bin
		ext = ".bin"
	}

	// 修复 #9: 文件类型验证
	allowedTypes := map[string]bool{
		"jpg":  true, "jpeg": true, "png": true, "gif": true, "webp": true,
		"mp4":  true, "webm": true,
		"pdf":  true, "doc": true, "docx": true, "txt": true,
		"zip":  true, "rar": true,
		"bin":  true, // 无扩展名文件
	}
	if !allowedTypes[fileType] {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    4003,
			"message": "不支持的文件类型: " + fileType,
		})
		return
	}

	date := time.Now().Format("2006/01/02")
	newFilename := uuid.New().String() + ext
	storagePath := filepath.Join("storage", "files", date, newFilename)

	// 创建目录
	dir := filepath.Dir(storagePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1005,
			"message": "创建目录失败",
		})
		return
	}

	// 保存文件
	dst, err := os.Create(storagePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1005,
			"message": "保存文件失败",
		})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1005,
			"message": "保存文件失败",
		})
		return
	}

	// 保存文件信息到数据库
	fileRecord := &model.File{
		Filename:   header.Filename,
		Filepath:   "/" + storagePath,
		Filesize:   header.Size,
		Filetype:   fileType,
		Mimetype:   header.Header.Get("Content-Type"),
		MD5:        md5Str,
		UploaderID: userID,
	}

	if err := h.db.Create(fileRecord).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1005,
			"message": "保存文件信息失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"id":       fileRecord.ID,
			"filename": fileRecord.Filename,
			"url":      fileRecord.Filepath,
			"filesize": fileRecord.Filesize,
			"filetype": fileRecord.Filetype,
			"mimetype": fileRecord.Mimetype,
		},
	})
}

// Download 下载文件
func (h *FileHandler) Download(c *gin.Context) {
	fileID := c.Param("file_id")

	var file model.File
	if err := h.db.First(&file, fileID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    1004,
			"message": "文件不存在",
		})
		return
	}

	// 获取本地文件路径
	localPath := file.Filepath[1:] // 移除开头的 /

	// 检查文件是否存在
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    1004,
			"message": "文件不存在",
		})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", file.Filename))
	c.Header("Content-Type", file.Mimetype)
	c.File(localPath)
}
