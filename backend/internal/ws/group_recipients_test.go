package ws

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRemovedOnlineMemberDoesNotReceiveGroupMessage(t *testing.T) {
	const (
		groupID         = uint(42)
		senderID        = uint(7)
		removedMemberID = uint(8)
	)

	hub := newHubWithSuccessfulPersistence(t)
	hub.resolveGroupRecipients = func(context.Context, uint) ([]uint, error) {
		return []uint{senderID}, nil
	}
	startGroupRecipientHub(t, hub)

	sender := NewClient(nil, senderID, "sender", []uint{groupID})
	removedMember := NewClient(nil, removedMemberID, "removed", []uint{groupID})
	registerGroupRecipientClient(t, hub, sender)
	registerGroupRecipientClient(t, hub, removedMember)

	hub.SendGroup(&Message{
		MsgID:       "56aa3068-f76d-4756-92c6-257ea30f8714",
		Type:        MsgTypeChat,
		FromID:      senderID,
		ToID:        groupID,
		ToType:      ToTypeGroup,
		ContentType: ContentTypeText,
		Content:     "current members only",
	})

	waitForClientMessageType(t, sender, MsgTypeChatAck)
	assertClientReceivesNoChatMessage(t, removedMember)
}

func TestNewlyInvitedOnlineMemberReceivesGroupMessageWithoutReconnect(t *testing.T) {
	const (
		groupID         = uint(42)
		senderID        = uint(7)
		invitedMemberID = uint(9)
	)

	hub := newHubWithSuccessfulPersistence(t)
	hub.resolveGroupRecipients = func(context.Context, uint) ([]uint, error) {
		return []uint{senderID, invitedMemberID}, nil
	}
	startGroupRecipientHub(t, hub)

	sender := NewClient(nil, senderID, "sender", []uint{groupID})
	invitedMember := NewClient(nil, invitedMemberID, "invited", nil)
	registerGroupRecipientClient(t, hub, sender)
	registerGroupRecipientClient(t, hub, invitedMember)

	message := &Message{
		MsgID:       "0182d374-a195-4e70-890c-416e33967f76",
		Type:        MsgTypeChat,
		FromID:      senderID,
		ToID:        groupID,
		ToType:      ToTypeGroup,
		ContentType: ContentTypeText,
		Content:     "welcome to the group",
	}
	hub.SendGroup(message)

	received := waitForClientMessageType(t, invitedMember, MsgTypeChat)
	if received.MsgID != message.MsgID {
		t.Fatalf("received message ID = %q, want %q", received.MsgID, message.MsgID)
	}
}

func TestGroupRecipientLookupFailureDoesNotDeliverMessage(t *testing.T) {
	const (
		groupID  = uint(42)
		senderID = uint(7)
		memberID = uint(10)
	)

	hub := NewHub(nil)
	hub.resolveGroupRecipients = func(context.Context, uint) ([]uint, error) {
		return nil, errGroupRecipientResolutionUnavailable
	}
	startGroupRecipientHub(t, hub)

	sender := NewClient(nil, senderID, "sender", []uint{groupID})
	member := NewClient(nil, memberID, "member", []uint{groupID})
	registerGroupRecipientClient(t, hub, sender)
	registerGroupRecipientClient(t, hub, member)

	const msgID = "0d2ce48e-5dbd-4e36-a7e8-90405bc09c7f"
	hub.SendGroup(&Message{
		MsgID:       msgID,
		Type:        MsgTypeChat,
		FromID:      senderID,
		ToID:        groupID,
		ToType:      ToTypeGroup,
		ContentType: ContentTypeText,
		Content:     "must not leak on lookup failure",
	})

	response := waitForClientMessageType(t, sender, MsgTypeError)
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("error data type = %T, want map[string]interface{}", response.Data)
	}
	if data["code"] != errorCodeInternal || data["msg_id"] != msgID {
		t.Fatalf("unexpected error data: %#v", data)
	}
	assertClientReceivesNoChatMessage(t, member)
}

func TestGroupRecipientResolverFailsClosedWithoutDatabase(t *testing.T) {
	resolve := newGroupRecipientResolver(nil)
	userIDs, err := resolve(context.Background(), 42)
	if !errors.Is(err, errGroupRecipientResolutionUnavailable) {
		t.Fatalf("recipient resolution error = %v, want %v", err, errGroupRecipientResolutionUnavailable)
	}
	if userIDs != nil {
		t.Fatalf("recipient IDs = %v, want nil", userIDs)
	}
}

func startGroupRecipientHub(t *testing.T, hub *Hub) {
	t.Helper()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := hub.Shutdown(ctx); err != nil {
			t.Errorf("shutdown hub: %v", err)
		}
	})
}

func registerGroupRecipientClient(t *testing.T, hub *Hub, client *Client) {
	t.Helper()
	if err := hub.Register(client); err != nil {
		t.Fatalf("register client %d: %v", client.UserID, err)
	}
}

func waitForClientMessageType(t *testing.T, client *Client, messageType string) *Message {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()

	for {
		select {
		case msg := <-client.Send:
			if msg.Type == messageType {
				return msg
			}
		case <-timer.C:
			t.Fatalf("client %d did not receive message type %q", client.UserID, messageType)
			return nil
		}
	}
}

func assertClientReceivesNoChatMessage(t *testing.T, client *Client) {
	t.Helper()
	timer := time.NewTimer(150 * time.Millisecond)
	defer timer.Stop()

	for {
		select {
		case msg := <-client.Send:
			if msg.Type == MsgTypeChat {
				t.Fatalf("client %d unexpectedly received a chat message", client.UserID)
			}
		case <-timer.C:
			return
		}
	}
}
