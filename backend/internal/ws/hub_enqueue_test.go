package ws_test

import (
	"context"
	"errors"
	"testing"
	"time"

	chatws "ChatRoom/internal/ws"
)

func TestSendPrivateReturnsErrorWhenQueueIsFull(t *testing.T) {
	hub := chatws.NewHub(nil, nil)
	message := &chatws.Message{
		MsgID:  "private-queue-full",
		Type:   chatws.MsgTypeChat,
		FromID: 7,
		ToID:   8,
		ToType: chatws.ToTypeUser,
	}

	for attempt := 0; attempt < 10_000; attempt++ {
		err := hub.SendPrivate(message)
		if errors.Is(err, chatws.ErrHubQueueFull) {
			return
		}
		if err != nil {
			t.Fatalf("send private message returned unexpected error: %v", err)
		}
	}

	t.Fatal("private message queue did not report being full")
}

func TestSendGroupReturnsErrorWhenQueueIsFull(t *testing.T) {
	hub := chatws.NewHub(nil, nil)
	message := &chatws.Message{
		MsgID:  "group-queue-full",
		Type:   chatws.MsgTypeChat,
		FromID: 7,
		ToID:   42,
		ToType: chatws.ToTypeGroup,
	}

	for attempt := 0; attempt < 10_000; attempt++ {
		err := hub.SendGroup(message)
		if errors.Is(err, chatws.ErrHubQueueFull) {
			return
		}
		if err != nil {
			t.Fatalf("send group message returned unexpected error: %v", err)
		}
	}

	t.Fatal("group message queue did not report being full")
}

func TestBroadcastReturnsErrorWhenQueueIsFull(t *testing.T) {
	hub := chatws.NewHub(nil, nil)
	message := &chatws.Message{Type: chatws.MsgTypeOnlineStatus}

	for attempt := 0; attempt < 10_000; attempt++ {
		err := hub.Broadcast(message)
		if errors.Is(err, chatws.ErrHubQueueFull) {
			return
		}
		if err != nil {
			t.Fatalf("broadcast message returned unexpected error: %v", err)
		}
	}

	t.Fatal("broadcast message queue did not report being full")
}

func TestMessageEntryPointsReturnClosedAfterShutdown(t *testing.T) {
	hub := chatws.NewHub(nil, nil)
	go hub.Run()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := hub.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown hub: %v", err)
	}

	message := &chatws.Message{Type: chatws.MsgTypeChat}
	tests := []struct {
		name string
		send func(*chatws.Message) error
	}{
		{name: "private", send: hub.SendPrivate},
		{name: "group", send: hub.SendGroup},
		{name: "broadcast", send: hub.Broadcast},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.send(message); !errors.Is(err, chatws.ErrHubClosed) {
				t.Fatalf("send after shutdown error = %v, want %v", err, chatws.ErrHubClosed)
			}
		})
	}
}
