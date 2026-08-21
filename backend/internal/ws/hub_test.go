package ws_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	chatws "ChatRoom/internal/ws"

	"github.com/gorilla/websocket"
)

func TestNewConnectionReplacesOldConnection(t *testing.T) {
	hub := startHub(t)

	oldPeer, oldServer := websocketPair(t)
	oldClient := chatws.NewClient(oldServer, 7, "alice")
	startClient(t, hub, oldClient)

	newPeer, newServer := websocketPair(t)
	newClient := chatws.NewClient(newServer, 7, "alice")
	startClient(t, hub, newClient)

	if err := oldPeer.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set old peer read deadline: %v", err)
	}
	_, _, err := oldPeer.ReadMessage()
	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) {
		t.Fatalf("old connection should receive a close frame, got %v", err)
	}
	if closeErr.Code != chatws.CloseCodeConnectionReplaced {
		t.Fatalf("old connection close code = %d, want %d", closeErr.Code, chatws.CloseCodeConnectionReplaced)
	}

	eventually(t, func() bool {
		return hub.GetOnlineCount() == 1 && hub.GetUserClient(7) == newClient
	})

	_ = newPeer.Close()
}

func TestClosedClientRejectsMessages(t *testing.T) {
	client := chatws.NewClient(nil, 9, "closed")
	client.Close()

	if client.TrySend(&chatws.Message{Type: chatws.MsgTypeChat}) {
		t.Fatal("closed client accepted a message")
	}
}

func TestSlowConsumerIsRemovedAndClosed(t *testing.T) {
	hub := startHub(t)
	client := chatws.NewClient(nil, 12, "slow")
	if err := hub.Register(client); err != nil {
		t.Fatalf("register slow client: %v", err)
	}
	eventually(t, func() bool { return hub.GetUserClient(12) == client })

	for client.TrySend(&chatws.Message{Type: chatws.MsgTypeChat}) {
	}

	trigger := chatws.NewClient(nil, 13, "trigger")
	if err := hub.Register(trigger); err != nil {
		t.Fatalf("register online-status trigger: %v", err)
	}

	eventually(t, func() bool { return !hub.IsUserOnline(12) })
	if client.TrySend(&chatws.Message{Type: chatws.MsgTypeChat}) {
		t.Fatal("removed slow client accepted a message")
	}
}

func TestRegisterCommitsBeforeReturning(t *testing.T) {
	hub := startHub(t)

	for userID := uint(100); userID < 120; userID++ {
		client := chatws.NewClient(nil, userID, "queued")
		if err := hub.Register(client); err != nil {
			t.Fatalf("register user %d: %v", userID, err)
		}
		if got := hub.GetUserClient(userID); got != client {
			t.Fatalf("register returned before user %d was committed", userID)
		}
	}
}

func TestHubShutdownClosesConnectionsAndRejectsRegistration(t *testing.T) {
	hub := startHub(t)

	firstPeer, firstServer := websocketPair(t)
	startClient(t, hub, chatws.NewClient(firstServer, 31, "first"))
	secondPeer, secondServer := websocketPair(t)
	startClient(t, hub, chatws.NewClient(secondServer, 32, "second"))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := hub.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown hub: %v", err)
	}

	assertCloseCode(t, firstPeer, websocket.CloseGoingAway)
	assertCloseCode(t, secondPeer, websocket.CloseGoingAway)
	if got := hub.GetOnlineCount(); got != 0 {
		t.Fatalf("online count after shutdown = %d, want 0", got)
	}
	if err := hub.Register(chatws.NewClient(nil, 33, "late")); !errors.Is(err, chatws.ErrHubClosed) {
		t.Fatalf("register after shutdown error = %v, want %v", err, chatws.ErrHubClosed)
	}
	if err := hub.Shutdown(ctx); err != nil {
		t.Fatalf("second shutdown should be idempotent: %v", err)
	}
}

func TestPeerDisconnectRemovesCurrentConnection(t *testing.T) {
	hub := startHub(t)
	peer, server := websocketPair(t)
	startClient(t, hub, chatws.NewClient(server, 41, "leaving"))

	if err := peer.Close(); err != nil {
		t.Fatalf("close peer: %v", err)
	}
	eventually(t, func() bool {
		return hub.GetOnlineCount() == 0 && !hub.IsUserOnline(41)
	})
}

func TestConcurrentConnectionChurnKeepsLatestClient(t *testing.T) {
	hub := startHub(t)
	const userID = uint(71)

	var wg sync.WaitGroup
	errs := make(chan error, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := chatws.NewClient(nil, userID, "churn")
			if err := hub.Register(client); err != nil {
				errs <- err
				return
			}
			hub.Unregister(client)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("connection churn registration failed: %v", err)
	}

	latest := chatws.NewClient(nil, userID, "latest")
	if err := hub.Register(latest); err != nil {
		t.Fatalf("register latest client: %v", err)
	}
	eventually(t, func() bool {
		return hub.GetOnlineCount() == 1 && hub.GetUserClient(userID) == latest
	})
}

func startHub(t *testing.T) *chatws.Hub {
	t.Helper()
	hub := chatws.NewHub(nil)
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := hub.Shutdown(ctx); err != nil {
			t.Errorf("shutdown hub: %v", err)
		}
	})
	return hub
}

func startClient(t *testing.T, hub *chatws.Hub, client *chatws.Client) {
	t.Helper()
	if err := hub.Register(client); err != nil {
		t.Fatalf("register client: %v", err)
	}
	go client.WritePump()
	go client.ReadPump(hub, nil)
}

func websocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	serverConn := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}).Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		serverConn <- conn
	}))
	t.Cleanup(server.Close)

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	peer, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	t.Cleanup(func() { _ = peer.Close() })

	select {
	case conn := <-serverConn:
		return peer, conn
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for server websocket")
		return nil, nil
	}
}

func eventually(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not satisfied before timeout")
}

func assertCloseCode(t *testing.T, conn *websocket.Conn, want int) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	for {
		_, _, err := conn.ReadMessage()
		if err == nil {
			// 注册期间可能已有在线状态帧排在关闭帧之前。
			continue
		}

		var closeErr *websocket.CloseError
		if !errors.As(err, &closeErr) {
			t.Fatalf("connection should receive a close frame, got %v", err)
		}
		if closeErr.Code != want {
			t.Fatalf("close code = %d, want %d", closeErr.Code, want)
		}
		return
	}
}
