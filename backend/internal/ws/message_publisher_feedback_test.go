package ws

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPrivatePublishFailureReturnsErrorToSender(t *testing.T) {
	hub := NewHub(nil)

	peer, server := websocketPairForAuthorization(t)
	client := NewClient(server, 7, "alice", nil)
	go client.WritePump()
	go client.ReadPump(hub, &recordingMessagePublisher{err: errors.New("Redis 暂时不可用")})

	const msgID = "private-queue-full-feedback"
	if err := peer.WriteJSON(map[string]any{
		"type": MsgTypeChat,
		"data": map[string]any{
			"msg_id":       msgID,
			"to_id":        8,
			"to_type":      ToTypeUser,
			"content_type": ContentTypeText,
			"content":      "must report overload",
		},
	}); err != nil {
		t.Fatalf("write private message: %v", err)
	}

	assertWebSocketInternalError(t, peer, msgID)
}

func TestGroupPublishFailureReturnsErrorToSender(t *testing.T) {
	hub := NewHub(nil)
	hub.authorizeGroupMessage = func(context.Context, uint, uint) error {
		return nil
	}

	peer, server := websocketPairForAuthorization(t)
	client := NewClient(server, 7, "alice", []uint{42})
	go client.WritePump()
	go client.ReadPump(hub, &recordingMessagePublisher{err: errors.New("Redis 暂时不可用")})

	const msgID = "group-queue-full-feedback"
	if err := peer.WriteJSON(map[string]any{
		"type": MsgTypeChat,
		"data": map[string]any{
			"msg_id":       msgID,
			"to_id":        42,
			"to_type":      ToTypeGroup,
			"content_type": ContentTypeText,
			"content":      "must report overload",
		},
	}); err != nil {
		t.Fatalf("write group message: %v", err)
	}

	assertWebSocketInternalError(t, peer, msgID)
}

func assertWebSocketInternalError(t *testing.T, peer interface {
	SetReadDeadline(time.Time) error
	ReadJSON(any) error
}, msgID string) {
	t.Helper()
	if err := peer.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	var response struct {
		Type string `json:"type"`
		Data struct {
			Code  int    `json:"code"`
			MsgID string `json:"msg_id"`
		} `json:"data"`
	}
	if err := peer.ReadJSON(&response); err != nil {
		t.Fatalf("read enqueue error response: %v", err)
	}
	if response.Type != MsgTypeError || response.Data.Code != errorCodeInternal || response.Data.MsgID != msgID {
		t.Fatalf("unexpected enqueue error response: %+v", response)
	}
}
