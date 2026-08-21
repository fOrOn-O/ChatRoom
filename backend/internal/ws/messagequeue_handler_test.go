package ws

import (
	"context"
	"errors"
	"testing"

	"ChatRoom/internal/messagequeue"

	"github.com/DATA-DOG/go-sqlmock"
	drivermysql "github.com/go-sql-driver/mysql"
)

func TestQueuedPrivateMessagePersistsDeliversAndAcknowledges(t *testing.T) {
	db, mock := newMessagePersistenceTestDB(t)
	mock.ExpectExec("INSERT INTO messages").
		WithArgs("queued-private-success", uint(7), uint(8), ToTypeUser, ContentTypeText, "persisted before delivery").
		WillReturnResult(sqlmock.NewResult(1, 1))

	hub := NewHub(db, nil)
	startGroupRecipientHub(t, hub)
	sender, recipient := registerQueuedMessageClients(t, hub, nil)
	queued := newQueuedChatMessage("queued-private-success", recipient.UserID, messagequeue.ToTypeUser, "persisted before delivery")

	if err := hub.Handle(context.Background(), queued); err != nil {
		t.Fatalf("处理队列私聊消息失败: %v", err)
	}
	received := waitForClientMessageType(t, recipient, MsgTypeChat)
	if received.MsgID != queued.MsgID || received.FromName != queued.FromName || received.Timestamp != queued.Timestamp {
		t.Fatalf("接收消息 = %+v，期望保留队列消息信息 %+v", received, queued)
	}
	ack := waitForClientMessageType(t, sender, MsgTypeChatAck)
	assertMessageAck(t, ack, queued.MsgID, "sent")
}

func TestQueuedPrivatePersistenceFailureReturnsErrorWithoutDelivery(t *testing.T) {
	db, mock := newMessagePersistenceTestDB(t)
	mock.ExpectExec("INSERT INTO messages").
		WithArgs("queued-private-failure", uint(7), uint(8), ToTypeUser, ContentTypeText, "must stay pending").
		WillReturnError(errors.New("database unavailable"))

	hub := NewHub(db, nil)
	startGroupRecipientHub(t, hub)
	sender, recipient := registerQueuedMessageClients(t, hub, nil)
	queued := newQueuedChatMessage("queued-private-failure", recipient.UserID, messagequeue.ToTypeUser, "must stay pending")

	err := hub.Handle(context.Background(), queued)
	if !errors.Is(err, errMessagePersistenceUnavailable) {
		t.Fatalf("持久化失败时 Handle() 错误 = %v，期望 errMessagePersistenceUnavailable", err)
	}
	response := waitForClientMessageType(t, sender, MsgTypeError)
	assertInternalMessageError(t, response, queued.MsgID)
	assertClientReceivesNoChatMessage(t, recipient)
	assertClientReceivesNoMessageType(t, sender, MsgTypeChatAck)
}

func TestQueuedGroupRecipientFailureReturnsErrorWithoutPersistence(t *testing.T) {
	db, _ := newMessagePersistenceTestDB(t)
	hub := NewHub(db, nil)
	hub.authorizeGroupMessage = func(context.Context, uint, uint) error {
		return nil
	}
	hub.resolveGroupRecipients = func(context.Context, uint) ([]uint, error) {
		return nil, errGroupRecipientResolutionUnavailable
	}
	startGroupRecipientHub(t, hub)
	sender, recipient := registerQueuedMessageClients(t, hub, []uint{42})
	queued := newQueuedChatMessage("queued-group-recipient-failure", 42, messagequeue.ToTypeGroup, "must stay pending")

	err := hub.Handle(context.Background(), queued)
	if !errors.Is(err, errGroupRecipientResolutionUnavailable) {
		t.Fatalf("成员解析失败时 Handle() 错误 = %v，期望 errGroupRecipientResolutionUnavailable", err)
	}
	response := waitForClientMessageType(t, sender, MsgTypeError)
	assertInternalMessageError(t, response, queued.MsgID)
	assertClientReceivesNoChatMessage(t, recipient)
	assertClientReceivesNoMessageType(t, sender, MsgTypeChatAck)
}

func TestQueuedGroupMessageRejectsInactiveSenderBeforeProcessing(t *testing.T) {
	db, _ := newMessagePersistenceTestDB(t)
	hub := NewHub(db, nil)
	authorizationCalls := 0
	hub.authorizeGroupMessage = func(_ context.Context, userID uint, groupID uint) error {
		authorizationCalls++
		if userID != 7 || groupID != 42 {
			t.Fatalf("权限校验参数 = user_id %d, group_id %d，期望 7 和 42", userID, groupID)
		}
		return errGroupMessageForbidden
	}
	hub.resolveGroupRecipients = func(context.Context, uint) ([]uint, error) {
		t.Fatal("权限拒绝后不应解析群成员")
		return nil, nil
	}
	startGroupRecipientHub(t, hub)
	sender, recipient := registerQueuedMessageClients(t, hub, []uint{42})
	queued := newQueuedChatMessage("queued-group-forbidden", 42, messagequeue.ToTypeGroup, "must stay pending")

	err := hub.Handle(context.Background(), queued)
	if !errors.Is(err, errGroupMessageForbidden) {
		t.Fatalf("无权发送时 Handle() 错误 = %v，期望 errGroupMessageForbidden", err)
	}
	if authorizationCalls != 1 {
		t.Fatalf("权限校验次数 = %d，期望 1", authorizationCalls)
	}
	assertClientReceivesNoChatMessage(t, recipient)
	assertClientReceivesNoMessageType(t, sender, MsgTypeChatAck)
	assertClientReceivesNoMessageType(t, sender, MsgTypeError)
}

func TestQueuedPrivateDuplicateAcknowledgesWithoutRedelivery(t *testing.T) {
	db, mock := newMessagePersistenceTestDB(t)
	mock.ExpectExec("INSERT INTO messages").
		WithArgs("queued-private-duplicate", uint(7), uint(8), ToTypeUser, ContentTypeText, "deliver only once").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO messages").
		WithArgs("queued-private-duplicate", uint(7), uint(8), ToTypeUser, ContentTypeText, "deliver only once").
		WillReturnError(&drivermysql.MySQLError{Number: 1062, Message: "Duplicate entry"})
	mock.ExpectQuery("SELECT from_user_id, to_id, to_type, content_type, content FROM messages WHERE msg_id = \\? LIMIT 1").
		WithArgs("queued-private-duplicate").
		WillReturnRows(sqlmock.NewRows([]string{
			"from_user_id", "to_id", "to_type", "content_type", "content",
		}).AddRow(uint(7), uint(8), ToTypeUser, ContentTypeText, "deliver only once"))

	hub := NewHub(db, nil)
	startGroupRecipientHub(t, hub)
	sender, recipient := registerQueuedMessageClients(t, hub, nil)
	queued := newQueuedChatMessage("queued-private-duplicate", recipient.UserID, messagequeue.ToTypeUser, "deliver only once")

	if err := hub.Handle(context.Background(), queued); err != nil {
		t.Fatalf("首次处理队列消息失败: %v", err)
	}
	if err := hub.Handle(context.Background(), queued); err != nil {
		t.Fatalf("重复处理队列消息失败: %v", err)
	}
	received := waitForClientMessageType(t, recipient, MsgTypeChat)
	if received.MsgID != queued.MsgID {
		t.Fatalf("接收消息编号 = %q，期望 %q", received.MsgID, queued.MsgID)
	}
	assertMessageAck(t, waitForClientMessageType(t, sender, MsgTypeChatAck), queued.MsgID, "sent")
	assertMessageAck(t, waitForClientMessageType(t, sender, MsgTypeChatAck), queued.MsgID, "sent")
	assertClientReceivesNoChatMessage(t, recipient)
}

func TestQueuedGroupMessageResolvesRecipientsPersistsAndDelivers(t *testing.T) {
	db, mock := newMessagePersistenceTestDB(t)
	mock.ExpectExec("INSERT INTO messages").
		WithArgs("queued-group-success", uint(7), uint(42), ToTypeGroup, ContentTypeText, "current members only").
		WillReturnResult(sqlmock.NewResult(1, 1))

	hub := NewHub(db, nil)
	authorizationCalls := 0
	hub.authorizeGroupMessage = func(_ context.Context, userID uint, groupID uint) error {
		authorizationCalls++
		if userID != 7 || groupID != 42 {
			t.Fatalf("权限校验参数 = user_id %d, group_id %d，期望 7 和 42", userID, groupID)
		}
		return nil
	}
	hub.resolveGroupRecipients = func(context.Context, uint) ([]uint, error) {
		return []uint{7, 8}, nil
	}
	startGroupRecipientHub(t, hub)
	sender, recipient := registerQueuedMessageClients(t, hub, []uint{42})
	queued := newQueuedChatMessage("queued-group-success", 42, messagequeue.ToTypeGroup, "current members only")

	if err := hub.Handle(context.Background(), queued); err != nil {
		t.Fatalf("处理队列群聊消息失败: %v", err)
	}
	if authorizationCalls != 1 {
		t.Fatalf("权限校验次数 = %d，期望 1", authorizationCalls)
	}
	received := waitForClientMessageType(t, recipient, MsgTypeChat)
	if received.MsgID != queued.MsgID || received.ToType != ToTypeGroup {
		t.Fatalf("接收群消息 = %+v，期望消息编号 %q", received, queued.MsgID)
	}
	assertMessageAck(t, waitForClientMessageType(t, sender, MsgTypeChatAck), queued.MsgID, "sent")
}

func registerQueuedMessageClients(t *testing.T, hub *Hub, groupIDs []uint) (*Client, *Client) {
	t.Helper()
	sender := NewClient(nil, 7, "sender", groupIDs)
	recipient := NewClient(nil, 8, "recipient", groupIDs)
	registerGroupRecipientClient(t, hub, sender)
	registerGroupRecipientClient(t, hub, recipient)
	return sender, recipient
}

func newQueuedChatMessage(msgID string, toID uint, toType string, content string) messagequeue.ChatMessage {
	return messagequeue.ChatMessage{
		Version:     messagequeue.ChatMessageVersion,
		MsgID:       msgID,
		FromID:      7,
		FromName:    "sender",
		ToID:        toID,
		ToType:      toType,
		ContentType: messagequeue.ContentTypeText,
		Content:     content,
		Timestamp:   1787241600,
	}
}
