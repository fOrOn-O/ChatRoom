package ws

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	drivermysql "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPrivateMessagePersistenceFailureDoesNotDeliver(t *testing.T) {
	db, mock := newMessagePersistenceTestDB(t)
	mock.ExpectExec("INSERT INTO messages").
		WithArgs("private-write-failure", uint(7), uint(8), ToTypeUser, ContentTypeText, "must not deliver").
		WillReturnError(errors.New("database unavailable"))

	hub := NewHub(db)
	startGroupRecipientHub(t, hub)

	sender := NewClient(nil, 7, "sender", nil)
	recipient := NewClient(nil, 8, "recipient", nil)
	registerGroupRecipientClient(t, hub, sender)
	registerGroupRecipientClient(t, hub, recipient)

	hub.SendPrivate(&Message{
		MsgID:       "private-write-failure",
		Type:        MsgTypeChat,
		FromID:      sender.UserID,
		ToID:        recipient.UserID,
		ToType:      ToTypeUser,
		ContentType: ContentTypeText,
		Content:     "must not deliver",
	})

	response := waitForClientMessageType(t, sender, MsgTypeError)
	assertInternalMessageError(t, response, "private-write-failure")
	assertClientReceivesNoChatMessage(t, recipient)
	assertClientReceivesNoMessageType(t, sender, MsgTypeChatAck)
}

func TestGroupMessagePersistenceFailureDoesNotDeliver(t *testing.T) {
	db, mock := newMessagePersistenceTestDB(t)
	mock.ExpectExec("INSERT INTO messages").
		WithArgs("group-write-failure", uint(7), uint(42), ToTypeGroup, ContentTypeText, "must not deliver").
		WillReturnError(errors.New("database unavailable"))

	hub := NewHub(db)
	hub.resolveGroupRecipients = func(context.Context, uint) ([]uint, error) {
		return []uint{7, 8}, nil
	}
	startGroupRecipientHub(t, hub)

	sender := NewClient(nil, 7, "sender", []uint{42})
	recipient := NewClient(nil, 8, "recipient", []uint{42})
	registerGroupRecipientClient(t, hub, sender)
	registerGroupRecipientClient(t, hub, recipient)

	hub.SendGroup(&Message{
		MsgID:       "group-write-failure",
		Type:        MsgTypeChat,
		FromID:      sender.UserID,
		ToID:        42,
		ToType:      ToTypeGroup,
		ContentType: ContentTypeText,
		Content:     "must not deliver",
	})

	response := waitForClientMessageType(t, sender, MsgTypeError)
	assertInternalMessageError(t, response, "group-write-failure")
	assertClientReceivesNoChatMessage(t, recipient)
	assertClientReceivesNoMessageType(t, sender, MsgTypeChatAck)
}

func TestGroupRecipientLookupFailureDoesNotPersist(t *testing.T) {
	db, _ := newMessagePersistenceTestDB(t)
	db = db.Session(&gorm.Session{DryRun: true})

	var persistenceAttempts atomic.Int32
	if err := db.Callback().Raw().Before("gorm:raw").Register("test:count-persistence", func(*gorm.DB) {
		persistenceAttempts.Add(1)
	}); err != nil {
		t.Fatalf("register persistence callback: %v", err)
	}

	hub := NewHub(db)
	hub.resolveGroupRecipients = func(context.Context, uint) ([]uint, error) {
		return nil, errGroupRecipientResolutionUnavailable
	}
	startGroupRecipientHub(t, hub)

	sender := NewClient(nil, 7, "sender", []uint{42})
	registerGroupRecipientClient(t, hub, sender)

	hub.SendGroup(&Message{
		MsgID:       "recipient-lookup-failure",
		Type:        MsgTypeChat,
		FromID:      sender.UserID,
		ToID:        42,
		ToType:      ToTypeGroup,
		ContentType: ContentTypeText,
		Content:     "must not persist",
	})

	response := waitForClientMessageType(t, sender, MsgTypeError)
	assertInternalMessageError(t, response, "recipient-lookup-failure")
	if attempts := persistenceAttempts.Load(); attempts != 0 {
		t.Fatalf("persistence attempts = %d, want 0", attempts)
	}
}

func TestOfflinePrivateMessageAcknowledgedAfterPersistence(t *testing.T) {
	db, mock := newMessagePersistenceTestDB(t)
	mock.ExpectExec("INSERT INTO messages").
		WithArgs("offline-private-success", uint(7), uint(8), ToTypeUser, ContentTypeText, "persisted for history").
		WillReturnResult(sqlmock.NewResult(1, 1))

	hub := NewHub(db)
	startGroupRecipientHub(t, hub)

	sender := NewClient(nil, 7, "sender", nil)
	registerGroupRecipientClient(t, hub, sender)

	hub.SendPrivate(&Message{
		MsgID:       "offline-private-success",
		Type:        MsgTypeChat,
		FromID:      sender.UserID,
		ToID:        8,
		ToType:      ToTypeUser,
		ContentType: ContentTypeText,
		Content:     "persisted for history",
	})

	response := waitForClientMessageType(t, sender, MsgTypeChatAck)
	assertMessageAck(t, response, "offline-private-success", "sent")
}

func TestOnlinePrivateMessageDeliveredAfterPersistence(t *testing.T) {
	db, mock := newMessagePersistenceTestDB(t)
	mock.ExpectExec("INSERT INTO messages").
		WithArgs("online-private-success", uint(7), uint(8), ToTypeUser, ContentTypeText, "persisted before delivery").
		WillReturnResult(sqlmock.NewResult(1, 1))

	hub := NewHub(db)
	startGroupRecipientHub(t, hub)

	sender := NewClient(nil, 7, "sender", nil)
	recipient := NewClient(nil, 8, "recipient", nil)
	registerGroupRecipientClient(t, hub, sender)
	registerGroupRecipientClient(t, hub, recipient)

	hub.SendPrivate(&Message{
		MsgID:       "online-private-success",
		Type:        MsgTypeChat,
		FromID:      sender.UserID,
		ToID:        recipient.UserID,
		ToType:      ToTypeUser,
		ContentType: ContentTypeText,
		Content:     "persisted before delivery",
	})

	received := waitForClientMessageType(t, recipient, MsgTypeChat)
	if received.MsgID != "online-private-success" {
		t.Fatalf("received message ID = %q, want %q", received.MsgID, "online-private-success")
	}
	response := waitForClientMessageType(t, sender, MsgTypeChatAck)
	assertMessageAck(t, response, "online-private-success", "sent")
}

func TestDuplicatePrivateMessageAcknowledgedWithoutRedelivery(t *testing.T) {
	db, mock := newMessagePersistenceTestDB(t)
	mock.ExpectExec("INSERT INTO messages").
		WithArgs("duplicate-private", uint(7), uint(8), ToTypeUser, ContentTypeText, "already persisted").
		WillReturnError(&drivermysql.MySQLError{Number: 1062, Message: "Duplicate entry"})
	mock.ExpectQuery("SELECT from_user_id, to_id, to_type, content_type, content FROM messages WHERE msg_id = \\? LIMIT 1").
		WithArgs("duplicate-private").
		WillReturnRows(sqlmock.NewRows([]string{
			"from_user_id", "to_id", "to_type", "content_type", "content",
		}).AddRow(uint(7), uint(8), ToTypeUser, ContentTypeText, "already persisted"))

	hub := NewHub(db)
	startGroupRecipientHub(t, hub)

	sender := NewClient(nil, 7, "sender", nil)
	recipient := NewClient(nil, 8, "recipient", nil)
	registerGroupRecipientClient(t, hub, sender)
	registerGroupRecipientClient(t, hub, recipient)

	hub.SendPrivate(&Message{
		MsgID:       "duplicate-private",
		Type:        MsgTypeChat,
		FromID:      sender.UserID,
		ToID:        recipient.UserID,
		ToType:      ToTypeUser,
		ContentType: ContentTypeText,
		Content:     "already persisted",
	})

	response := waitForClientMessageType(t, sender, MsgTypeChatAck)
	assertMessageAck(t, response, "duplicate-private", "sent")
	assertClientReceivesNoChatMessage(t, recipient)
	assertClientReceivesNoMessageType(t, sender, MsgTypeError)
}

func TestDuplicateGroupMessageAcknowledgedWithoutRedelivery(t *testing.T) {
	db, mock := newMessagePersistenceTestDB(t)
	mock.ExpectExec("INSERT INTO messages").
		WithArgs("duplicate-group", uint(7), uint(42), ToTypeGroup, ContentTypeText, "already persisted").
		WillReturnError(&drivermysql.MySQLError{Number: 1062, Message: "Duplicate entry"})
	mock.ExpectQuery("SELECT from_user_id, to_id, to_type, content_type, content FROM messages WHERE msg_id = \\? LIMIT 1").
		WithArgs("duplicate-group").
		WillReturnRows(sqlmock.NewRows([]string{
			"from_user_id", "to_id", "to_type", "content_type", "content",
		}).AddRow(uint(7), uint(42), ToTypeGroup, ContentTypeText, "already persisted"))

	hub := NewHub(db)
	hub.resolveGroupRecipients = func(context.Context, uint) ([]uint, error) {
		return []uint{7, 8}, nil
	}
	startGroupRecipientHub(t, hub)

	sender := NewClient(nil, 7, "sender", []uint{42})
	recipient := NewClient(nil, 8, "recipient", []uint{42})
	registerGroupRecipientClient(t, hub, sender)
	registerGroupRecipientClient(t, hub, recipient)

	hub.SendGroup(&Message{
		MsgID:       "duplicate-group",
		Type:        MsgTypeChat,
		FromID:      sender.UserID,
		ToID:        42,
		ToType:      ToTypeGroup,
		ContentType: ContentTypeText,
		Content:     "already persisted",
	})

	response := waitForClientMessageType(t, sender, MsgTypeChatAck)
	assertMessageAck(t, response, "duplicate-group", "sent")
	assertClientReceivesNoChatMessage(t, recipient)
	assertClientReceivesNoMessageType(t, sender, MsgTypeError)
}

func TestMessageIDConflictWithDifferentContentIsRejected(t *testing.T) {
	db, mock := newMessagePersistenceTestDB(t)
	mock.ExpectExec("INSERT INTO messages").
		WithArgs("content-conflict", uint(7), uint(8), ToTypeUser, ContentTypeText, "new content").
		WillReturnError(&drivermysql.MySQLError{Number: 1062, Message: "Duplicate entry"})
	mock.ExpectQuery("SELECT from_user_id, to_id, to_type, content_type, content FROM messages WHERE msg_id = \\? LIMIT 1").
		WithArgs("content-conflict").
		WillReturnRows(sqlmock.NewRows([]string{
			"from_user_id", "to_id", "to_type", "content_type", "content",
		}).AddRow(uint(7), uint(8), ToTypeUser, ContentTypeText, "original content"))

	hub := NewHub(db)
	startGroupRecipientHub(t, hub)

	sender := NewClient(nil, 7, "sender", nil)
	recipient := NewClient(nil, 8, "recipient", nil)
	registerGroupRecipientClient(t, hub, sender)
	registerGroupRecipientClient(t, hub, recipient)

	hub.SendPrivate(&Message{
		MsgID:       "content-conflict",
		Type:        MsgTypeChat,
		FromID:      sender.UserID,
		ToID:        recipient.UserID,
		ToType:      ToTypeUser,
		ContentType: ContentTypeText,
		Content:     "new content",
	})

	response := waitForClientMessageType(t, sender, MsgTypeError)
	assertMessageErrorCode(t, response, "content-conflict", errorCodeInvalidMessage)
	assertClientReceivesNoChatMessage(t, recipient)
	assertClientReceivesNoMessageType(t, sender, MsgTypeChatAck)
}

func TestMessageIDConflictWithDifferentSenderIsRejected(t *testing.T) {
	db, mock := newMessagePersistenceTestDB(t)
	mock.ExpectExec("INSERT INTO messages").
		WithArgs("sender-conflict", uint(7), uint(8), ToTypeUser, ContentTypeText, "same content").
		WillReturnError(&drivermysql.MySQLError{Number: 1062, Message: "Duplicate entry"})
	mock.ExpectQuery("SELECT from_user_id, to_id, to_type, content_type, content FROM messages WHERE msg_id = \\? LIMIT 1").
		WithArgs("sender-conflict").
		WillReturnRows(sqlmock.NewRows([]string{
			"from_user_id", "to_id", "to_type", "content_type", "content",
		}).AddRow(uint(9), uint(8), ToTypeUser, ContentTypeText, "same content"))

	hub := NewHub(db)
	startGroupRecipientHub(t, hub)

	sender := NewClient(nil, 7, "sender", nil)
	recipient := NewClient(nil, 8, "recipient", nil)
	registerGroupRecipientClient(t, hub, sender)
	registerGroupRecipientClient(t, hub, recipient)

	hub.SendPrivate(&Message{
		MsgID:       "sender-conflict",
		Type:        MsgTypeChat,
		FromID:      sender.UserID,
		ToID:        recipient.UserID,
		ToType:      ToTypeUser,
		ContentType: ContentTypeText,
		Content:     "same content",
	})

	response := waitForClientMessageType(t, sender, MsgTypeError)
	assertMessageErrorCode(t, response, "sender-conflict", errorCodeInvalidMessage)
	assertClientReceivesNoChatMessage(t, recipient)
	assertClientReceivesNoMessageType(t, sender, MsgTypeChatAck)
}

func TestMessageIDConflictWithDifferentTargetIsRejected(t *testing.T) {
	db, mock := newMessagePersistenceTestDB(t)
	mock.ExpectExec("INSERT INTO messages").
		WithArgs("target-conflict", uint(7), uint(8), ToTypeUser, ContentTypeText, "same content").
		WillReturnError(&drivermysql.MySQLError{Number: 1062, Message: "Duplicate entry"})
	mock.ExpectQuery("SELECT from_user_id, to_id, to_type, content_type, content FROM messages WHERE msg_id = \\? LIMIT 1").
		WithArgs("target-conflict").
		WillReturnRows(sqlmock.NewRows([]string{
			"from_user_id", "to_id", "to_type", "content_type", "content",
		}).AddRow(uint(7), uint(9), ToTypeUser, ContentTypeText, "same content"))

	hub := NewHub(db)
	startGroupRecipientHub(t, hub)

	sender := NewClient(nil, 7, "sender", nil)
	recipient := NewClient(nil, 8, "recipient", nil)
	registerGroupRecipientClient(t, hub, sender)
	registerGroupRecipientClient(t, hub, recipient)

	hub.SendPrivate(&Message{
		MsgID:       "target-conflict",
		Type:        MsgTypeChat,
		FromID:      sender.UserID,
		ToID:        recipient.UserID,
		ToType:      ToTypeUser,
		ContentType: ContentTypeText,
		Content:     "same content",
	})

	response := waitForClientMessageType(t, sender, MsgTypeError)
	assertMessageErrorCode(t, response, "target-conflict", errorCodeInvalidMessage)
	assertClientReceivesNoChatMessage(t, recipient)
	assertClientReceivesNoMessageType(t, sender, MsgTypeChatAck)
}

func TestDuplicateMessageLookupFailureDoesNotAcknowledgeOrDeliver(t *testing.T) {
	db, mock := newMessagePersistenceTestDB(t)
	mock.ExpectExec("INSERT INTO messages").
		WithArgs("duplicate-lookup-failure", uint(7), uint(8), ToTypeUser, ContentTypeText, "unknown duplicate").
		WillReturnError(&drivermysql.MySQLError{Number: 1062, Message: "Duplicate entry"})
	mock.ExpectQuery("SELECT from_user_id, to_id, to_type, content_type, content FROM messages WHERE msg_id = \\? LIMIT 1").
		WithArgs("duplicate-lookup-failure").
		WillReturnError(errors.New("database unavailable"))

	hub := NewHub(db)
	startGroupRecipientHub(t, hub)

	sender := NewClient(nil, 7, "sender", nil)
	recipient := NewClient(nil, 8, "recipient", nil)
	registerGroupRecipientClient(t, hub, sender)
	registerGroupRecipientClient(t, hub, recipient)

	hub.SendPrivate(&Message{
		MsgID:       "duplicate-lookup-failure",
		Type:        MsgTypeChat,
		FromID:      sender.UserID,
		ToID:        recipient.UserID,
		ToType:      ToTypeUser,
		ContentType: ContentTypeText,
		Content:     "unknown duplicate",
	})

	response := waitForClientMessageType(t, sender, MsgTypeError)
	assertInternalMessageError(t, response, "duplicate-lookup-failure")
	assertClientReceivesNoChatMessage(t, recipient)
	assertClientReceivesNoMessageType(t, sender, MsgTypeChatAck)
}

func newMessagePersistenceTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
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

	db, err := gorm.Open(gormmysql.New(gormmysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open gorm database: %v", err)
	}

	return db, mock
}

func newHubWithSuccessfulPersistence(t *testing.T) *Hub {
	t.Helper()
	db, _ := newMessagePersistenceTestDB(t)
	return NewHub(db.Session(&gorm.Session{DryRun: true}))
}

func assertInternalMessageError(t *testing.T, message *Message, msgID string) {
	t.Helper()
	assertMessageErrorCode(t, message, msgID, errorCodeInternal)
}

func assertMessageErrorCode(t *testing.T, message *Message, msgID string, code int) {
	t.Helper()

	data, ok := message.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("error data type = %T, want map[string]interface{}", message.Data)
	}
	if data["code"] != code || data["msg_id"] != msgID {
		t.Fatalf("unexpected error data: %#v", data)
	}
}

func assertMessageAck(t *testing.T, message *Message, msgID string, status string) {
	t.Helper()

	data, ok := message.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("ack data type = %T, want map[string]interface{}", message.Data)
	}
	if data["msg_id"] != msgID || data["status"] != status {
		t.Fatalf("unexpected ack data: %#v", data)
	}
}

func assertClientReceivesNoMessageType(t *testing.T, client *Client, messageType string) {
	t.Helper()

	timer := time.NewTimer(150 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case message := <-client.Send:
			if message.Type == messageType {
				t.Fatalf("client %d unexpectedly received message type %q", client.UserID, messageType)
			}
		case <-timer.C:
			return
		}
	}
}
