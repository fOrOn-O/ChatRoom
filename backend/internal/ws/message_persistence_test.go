package ws

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPrivateMessagePersistenceFailureDoesNotDeliver(t *testing.T) {
	db, mock := newMessagePersistenceTestDB(t)
	mock.ExpectExec("INSERT INTO messages").
		WithArgs("private-write-failure", uint(7), uint(8), ToTypeUser, ContentTypeText, "must not deliver").
		WillReturnError(errors.New("database unavailable"))

	hub := NewHub(db, nil)
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

	hub := NewHub(db, nil)
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

	hub := NewHub(db, nil)
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

	hub := NewHub(db, nil)
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

	hub := NewHub(db, nil)
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

	db, err := gorm.Open(mysql.New(mysql.Config{
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
	return NewHub(db.Session(&gorm.Session{DryRun: true}), nil)
}

func assertInternalMessageError(t *testing.T, message *Message, msgID string) {
	t.Helper()

	data, ok := message.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("error data type = %T, want map[string]interface{}", message.Data)
	}
	if data["code"] != errorCodeInternal || data["msg_id"] != msgID {
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
