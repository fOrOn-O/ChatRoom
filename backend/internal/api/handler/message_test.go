package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ChatRoom/internal/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestExitedMemberCannotReadGroupHistory(t *testing.T) {
	db, mock := newMessageHandlerTestDB(t)
	mock.ExpectQuery("SELECT .*group_status.*member_status.*FROM .*groups.*JOIN group_members.*").
		WillReturnRows(sqlmock.NewRows([]string{"group_status", "member_status"}).
			AddRow(model.GroupStatusActive, model.GroupMemberStatusInactive))

	response := performHistoryRequest(t, NewMessageHandler(db), 7, "/messages?target_id=42&target_type=group")

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusForbidden, response.Body.String())
	}
	if bodyCode(t, response) != 1003 {
		t.Fatalf("response code = %d, want 1003", bodyCode(t, response))
	}
}

func TestGroupHistoryCountFailureReturnsInternalError(t *testing.T) {
	db, mock := newMessageHandlerTestDB(t)
	expectActiveGroupHistoryAccess(mock)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `messages` WHERE to_id = \\? AND to_type = \\?").
		WithArgs(uint64(42), "group").
		WillReturnError(errors.New("database unavailable"))

	response := performHistoryRequest(t, NewMessageHandler(db), 7, "/messages?target_id=42&target_type=group")

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusInternalServerError, response.Body.String())
	}
	if bodyCode(t, response) != 1005 {
		t.Fatalf("response code = %d, want 1005", bodyCode(t, response))
	}
}

func TestGroupHistoryListFailureReturnsInternalError(t *testing.T) {
	db, mock := newMessageHandlerTestDB(t)
	expectActiveGroupHistoryAccess(mock)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `messages` WHERE to_id = \\? AND to_type = \\?").
		WithArgs(uint64(42), "group").
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(1))
	mock.ExpectQuery("SELECT \\* FROM `messages` WHERE to_id = \\? AND to_type = \\? ORDER BY created_at DESC LIMIT \\?").
		WithArgs(uint64(42), "group", 20).
		WillReturnError(errors.New("database unavailable"))

	response := performHistoryRequest(t, NewMessageHandler(db), 7, "/messages?target_id=42&target_type=group")

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusInternalServerError, response.Body.String())
	}
	if bodyCode(t, response) != 1005 {
		t.Fatalf("response code = %d, want 1005", bodyCode(t, response))
	}
}

func TestDissolvedGroupCannotReadHistory(t *testing.T) {
	db, mock := newMessageHandlerTestDB(t)
	mock.ExpectQuery("SELECT .*group_status.*member_status.*FROM .*groups.*JOIN group_members.*").
		WillReturnRows(sqlmock.NewRows([]string{"group_status", "member_status"}).
			AddRow(model.GroupStatusDissolved, model.GroupMemberStatusActive))

	response := performHistoryRequest(t, NewMessageHandler(db), 7, "/messages?target_id=42&target_type=group")

	if response.Code != http.StatusForbidden || bodyCode(t, response) != 1003 {
		t.Fatalf("unexpected response: status=%d code=%d body=%s", response.Code, bodyCode(t, response), response.Body.String())
	}
}

func TestActiveMemberCanReadGroupHistory(t *testing.T) {
	db, mock := newMessageHandlerTestDB(t)
	expectActiveGroupHistoryAccess(mock)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `messages` WHERE to_id = \\? AND to_type = \\?").
		WithArgs(uint64(42), "group").
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(1))
	mock.ExpectQuery("SELECT \\* FROM `messages` WHERE to_id = \\? AND to_type = \\? ORDER BY created_at DESC LIMIT \\?").
		WithArgs(uint64(42), "group", defaultHistoryPageSize).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "msg_id", "from_user_id", "to_id", "to_type", "content_type", "content", "extra", "created_at",
		}).AddRow(1, "message-id", 7, 42, "group", "text", "hello", "{}", time.Now()))

	response := performHistoryRequest(t, NewMessageHandler(db), 7, "/messages?target_id=42&target_type=group")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	var body struct {
		Code int `json:"code"`
		Data struct {
			Total int `json:"total"`
			List  []struct {
				MsgID string `json:"msg_id"`
			} `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body.Code != 0 || body.Data.Total != 1 || len(body.Data.List) != 1 || body.Data.List[0].MsgID != "message-id" {
		t.Fatalf("unexpected response body: %s", response.Body.String())
	}
}

func TestPrivateHistoryAfterIDReturnsOnlyNewMessagesInAscendingOrder(t *testing.T) {
	db, mock := newMessageHandlerTestDB(t)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `messages` WHERE .*to_type = 'user'.*AND id > \\?").
		WithArgs(uint(7), uint64(8), uint64(8), uint(7), uint64(41)).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(2))
	mock.ExpectQuery("SELECT \\* FROM `messages` WHERE .*to_type = 'user'.*AND id > \\? ORDER BY id ASC LIMIT \\?").
		WithArgs(uint(7), uint64(8), uint64(8), uint(7), uint64(41), defaultHistoryPageSize).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "msg_id", "from_user_id", "to_id", "to_type", "content_type", "content", "extra", "created_at",
		}).
			AddRow(42, "message-42", 8, 7, "user", "text", "first", "{}", time.Now()).
			AddRow(43, "message-43", 7, 8, "user", "text", "second", "{}", time.Now()))

	response := performHistoryRequest(
		t,
		NewMessageHandler(db),
		7,
		"/messages?target_id=8&target_type=user&after_id=41",
	)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	var body struct {
		Data struct {
			NextCursor string `json:"next_cursor"`
			HasMore    bool   `json:"has_more"`
			List       []struct {
				MsgID string `json:"msg_id"`
			} `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body.Data.NextCursor != "43" || body.Data.HasMore {
		t.Fatalf("cursor = %q, has_more = %t; body = %s", body.Data.NextCursor, body.Data.HasMore, response.Body.String())
	}
	if len(body.Data.List) != 2 || body.Data.List[0].MsgID != "message-42" || body.Data.List[1].MsgID != "message-43" {
		t.Fatalf("unexpected message order: %s", response.Body.String())
	}
}

func TestIncrementalHistoryReportsMoreMessages(t *testing.T) {
	db, mock := newMessageHandlerTestDB(t)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `messages` WHERE .*to_type = 'user'.*AND id > \\?").
		WithArgs(uint(7), uint64(8), uint64(8), uint(7), uint64(41)).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(2))
	mock.ExpectQuery("SELECT \\* FROM `messages` WHERE .*to_type = 'user'.*AND id > \\? ORDER BY id ASC LIMIT \\?").
		WithArgs(uint(7), uint64(8), uint64(8), uint(7), uint64(41), 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "msg_id", "from_user_id", "to_id", "to_type", "content_type", "content", "extra", "created_at",
		}).AddRow(42, "message-42", 8, 7, "user", "text", "first", "{}", time.Now()))

	response := performHistoryRequest(
		t,
		NewMessageHandler(db),
		7,
		"/messages?target_id=8&target_type=user&after_id=41&page_size=1",
	)

	var body struct {
		Data struct {
			NextCursor string `json:"next_cursor"`
			HasMore    bool   `json:"has_more"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if response.Code != http.StatusOK || body.Data.NextCursor != "42" || !body.Data.HasMore {
		t.Fatalf("unexpected response: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestNonMemberCannotReadGroupHistory(t *testing.T) {
	db, mock := newMessageHandlerTestDB(t)
	mock.ExpectQuery("SELECT .*group_status.*member_status.*FROM .*groups.*JOIN group_members.*").
		WillReturnRows(sqlmock.NewRows([]string{"group_status", "member_status"}))

	response := performHistoryRequest(t, NewMessageHandler(db), 7, "/messages?target_id=42&target_type=group")

	if response.Code != http.StatusForbidden || bodyCode(t, response) != 1003 {
		t.Fatalf("unexpected response: status=%d code=%d body=%s", response.Code, bodyCode(t, response), response.Body.String())
	}
}

func TestGroupHistoryAuthorizationFailureReturnsInternalError(t *testing.T) {
	db, mock := newMessageHandlerTestDB(t)
	mock.ExpectQuery("SELECT .*group_status.*member_status.*FROM .*groups.*JOIN group_members.*").
		WillReturnError(errors.New("database unavailable"))

	response := performHistoryRequest(t, NewMessageHandler(db), 7, "/messages?target_id=42&target_type=group")

	if response.Code != http.StatusInternalServerError || bodyCode(t, response) != 1005 {
		t.Fatalf("unexpected response: status=%d code=%d body=%s", response.Code, bodyCode(t, response), response.Body.String())
	}
}

func TestHistoryRejectsInvalidPagination(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{name: "page is not a number", query: "page=abc"},
		{name: "page is zero", query: "page=0"},
		{name: "page is negative", query: "page=-1"},
		{name: "page exceeds maximum", query: "page=10001"},
		{name: "page size is not a number", query: "page_size=abc"},
		{name: "page size is zero", query: "page_size=0"},
		{name: "page size is negative", query: "page_size=-1"},
		{name: "page size exceeds maximum", query: "page_size=101"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := performHistoryRequest(
				t,
				NewMessageHandler(nil),
				7,
				"/messages?target_id=8&target_type=user&"+test.query,
			)

			if response.Code != http.StatusBadRequest || bodyCode(t, response) != 1001 {
				t.Fatalf("unexpected response: status=%d code=%d body=%s", response.Code, bodyCode(t, response), response.Body.String())
			}
		})
	}
}

func TestHistoryRejectsInvalidAfterID(t *testing.T) {
	tests := []string{"", "abc", "-1", "18446744073709551616"}
	for _, afterID := range tests {
		t.Run("after_id="+afterID, func(t *testing.T) {
			response := performHistoryRequest(
				t,
				NewMessageHandler(nil),
				7,
				"/messages?target_id=8&target_type=user&after_id="+afterID,
			)

			if response.Code != http.StatusBadRequest || bodyCode(t, response) != 1001 {
				t.Fatalf("unexpected response: status=%d code=%d body=%s", response.Code, bodyCode(t, response), response.Body.String())
			}
		})
	}
}

func TestHistoryRejectsCursorCombinedWithLaterPage(t *testing.T) {
	db, _ := newMessageHandlerTestDB(t)
	response := performHistoryRequest(
		t,
		NewMessageHandler(db),
		7,
		"/messages?target_id=8&target_type=user&after_id=41&page=2",
	)

	if response.Code != http.StatusBadRequest || bodyCode(t, response) != 1001 {
		t.Fatalf("unexpected response: status=%d code=%d body=%s", response.Code, bodyCode(t, response), response.Body.String())
	}
}

func TestHistoryRejectsInvalidTarget(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{name: "target ID is missing", query: "target_type=user"},
		{name: "target ID is zero", query: "target_id=0&target_type=user"},
		{name: "target ID is not a number", query: "target_id=abc&target_type=user"},
		{name: "target ID exceeds uint32", query: "target_id=4294967296&target_type=user"},
		{name: "target type is missing", query: "target_id=8"},
		{name: "target type is unsupported", query: "target_id=8&target_type=channel"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := performHistoryRequest(t, NewMessageHandler(nil), 7, "/messages?"+test.query)

			if response.Code != http.StatusBadRequest || bodyCode(t, response) != 1001 {
				t.Fatalf("unexpected response: status=%d code=%d body=%s", response.Code, bodyCode(t, response), response.Body.String())
			}
		})
	}
}

func TestHistoryAcceptsMaximumPagination(t *testing.T) {
	db, mock := newMessageHandlerTestDB(t)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `messages` WHERE .*to_type = 'user'.*").
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(0))
	mock.ExpectQuery("SELECT \\* FROM `messages` WHERE .*to_type = 'user'.*ORDER BY created_at DESC LIMIT \\? OFFSET \\?").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), 100, 999900).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "msg_id", "from_user_id", "to_id", "to_type", "content_type", "content", "extra", "created_at",
		}))

	response := performHistoryRequest(
		t,
		NewMessageHandler(db),
		7,
		"/messages?target_id=8&target_type=user&page=10000&page_size=100",
	)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	var body struct {
		Data struct {
			Page     int `json:"page"`
			PageSize int `json:"page_size"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body.Data.Page != maxHistoryPage || body.Data.PageSize != maxHistoryPageSize {
		t.Fatalf("pagination = (%d, %d), want (%d, %d)", body.Data.Page, body.Data.PageSize, maxHistoryPage, maxHistoryPageSize)
	}
}

func expectActiveGroupHistoryAccess(mock sqlmock.Sqlmock) {
	mock.ExpectQuery("SELECT .*group_status.*member_status.*FROM .*groups.*JOIN group_members.*").
		WillReturnRows(sqlmock.NewRows([]string{"group_status", "member_status"}).
			AddRow(model.GroupStatusActive, model.GroupMemberStatusActive))
}

func newMessageHandlerTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sql mock: %v", err)
	}
	mock.MatchExpectationsInOrder(false)
	mock.ExpectClose()
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close sql mock: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet database expectations: %v", err)
		}
	})

	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open gorm database: %v", err)
	}

	return db, mock
}

func performHistoryRequest(t *testing.T, handler *MessageHandler, userID uint, target string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, target, nil)
	context.Set("user_id", userID)

	handler.GetHistory(context)
	return recorder
}

func bodyCode(t *testing.T, response *httptest.ResponseRecorder) int {
	t.Helper()

	var body struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	return body.Code
}
