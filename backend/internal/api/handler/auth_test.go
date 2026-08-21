package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ChatRoom/internal/ratelimit"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestLoginReturnsTooManyRequestsWhenLimitIsExceeded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("关闭 Redis 测试客户端失败: %v", err)
		}
	})

	limiter, err := ratelimit.NewLoginLimiter(client, ratelimit.LoginOptions{
		KeyPrefix:      "chatroom",
		IPLimit:        10,
		AccountIPLimit: 1,
		Window:         time.Minute,
	})
	if err != nil {
		t.Fatalf("创建登录限流器失败: %v", err)
	}
	if allowed, allowErr := limiter.Allow(context.Background(), "203.0.113.30", "alice"); allowErr != nil || !allowed {
		t.Fatalf("预占登录配额失败: allowed=%t err=%v", allowed, allowErr)
	}

	handler := NewAuthHandler(nil, "test-secret", limiter)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"alice","password":"secret"}`))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "203.0.113.30:54321"
	c.Request = request

	handler.Login(c)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("HTTP 状态码 = %d，期望 %d", recorder.Code, http.StatusTooManyRequests)
	}
	var response struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("解析登录限流响应失败: %v", err)
	}
	if response.Code != 2004 || response.Message != "登录尝试过于频繁，请稍后重试" {
		t.Fatalf("登录限流响应 = %+v", response)
	}
}

func TestLoginContinuesWhenRateLimiterIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	limiter, err := ratelimit.NewLoginLimiter(client, ratelimit.LoginOptions{
		KeyPrefix:      "chatroom",
		IPLimit:        10,
		AccountIPLimit: 5,
		Window:         time.Minute,
	})
	if err != nil {
		t.Fatalf("创建登录限流器失败: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("关闭 Redis 测试客户端失败: %v", err)
	}

	db, mock := newAuthHandlerTestDB(t)
	mock.ExpectQuery("SELECT .*users.*username = \\?").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "status"}))

	handler := NewAuthHandler(db, "test-secret", limiter)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"alice","password":"secret"}`))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "203.0.113.40:54321"
	c.Request = request

	handler.Login(c)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("限流器不可用时 HTTP 状态码 = %d，期望继续登录流程并返回 %d", recorder.Code, http.StatusUnauthorized)
	}
}

func newAuthHandlerTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("创建 SQL Mock 失败: %v", err)
	}
	mock.MatchExpectationsInOrder(false)
	mock.ExpectClose()
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("关闭 SQL Mock 失败: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("存在未满足的数据库调用: %v", err)
		}
	})

	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("创建 GORM 测试连接失败: %v", err)
	}
	return db, mock
}
