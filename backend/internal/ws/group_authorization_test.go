package ws

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ChatRoom/internal/model"

	"github.com/gorilla/websocket"
)

func TestNonMemberCannotSendGroupMessage(t *testing.T) {
	hub := NewHub(nil)
	var authorizedUserID uint
	var authorizedGroupID uint
	hub.authorizeGroupMessage = func(_ context.Context, userID uint, groupID uint) error {
		authorizedUserID = userID
		authorizedGroupID = groupID
		return errGroupMessageForbidden
	}
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := hub.Shutdown(ctx); err != nil {
			t.Errorf("shutdown hub: %v", err)
		}
	})

	peer, server := websocketPairForAuthorization(t)
	client := NewClient(server, 7, "alice")
	if err := hub.Register(client); err != nil {
		t.Fatalf("register client: %v", err)
	}
	go client.WritePump()
	go client.ReadPump(hub, &recordingMessagePublisher{err: errors.New("不应发布无权群消息")})

	const msgID = "59ffb838-6a50-4d9c-94e7-12cdd75269a1"
	err := peer.WriteJSON(map[string]any{
		"type": MsgTypeChat,
		"data": map[string]any{
			"msg_id":       msgID,
			"to_id":        42,
			"to_type":      ToTypeGroup,
			"content_type": ContentTypeText,
			"content":      "forged group message",
		},
	})
	if err != nil {
		t.Fatalf("write forged group message: %v", err)
	}

	if err := peer.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	var response struct {
		Type string `json:"type"`
		Data struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			MsgID   string `json:"msg_id"`
		} `json:"data"`
	}
	if err := peer.ReadJSON(&response); err != nil {
		t.Fatalf("read authorization response: %v", err)
	}
	if response.Type != MsgTypeError {
		t.Fatalf("response type = %q, want %q", response.Type, MsgTypeError)
	}
	if response.Data.Code != errorCodeGroupMessageForbidden {
		t.Fatalf("response code = %d, want %d", response.Data.Code, errorCodeGroupMessageForbidden)
	}
	if response.Data.MsgID != msgID {
		t.Fatalf("response msg_id = %q, want %q", response.Data.MsgID, msgID)
	}
	if authorizedUserID != 7 || authorizedGroupID != 42 {
		t.Fatalf(
			"authorization checked user %d and group %d, want user 7 and group 42",
			authorizedUserID,
			authorizedGroupID,
		)
	}

	if err := peer.SetReadDeadline(time.Now().Add(150 * time.Millisecond)); err != nil {
		t.Fatalf("set second read deadline: %v", err)
	}
	_, _, err = peer.ReadMessage()
	var netErr interface{ Timeout() bool }
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("rejected message produced another response: %v", err)
	}
}

func TestAuthorizedMemberCanSendGroupMessage(t *testing.T) {
	hub := newHubWithSuccessfulPersistence(t)
	hub.authorizeGroupMessage = func(context.Context, uint, uint) error {
		return nil
	}
	hub.resolveGroupRecipients = func(context.Context, uint) ([]uint, error) {
		return []uint{7}, nil
	}
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := hub.Shutdown(ctx); err != nil {
			t.Errorf("shutdown hub: %v", err)
		}
	})

	peer, server := websocketPairForAuthorization(t)
	client := NewClient(server, 7, "alice")
	if err := hub.Register(client); err != nil {
		t.Fatalf("register client: %v", err)
	}
	go client.WritePump()
	go client.ReadPump(hub, handlingMessagePublisher{handler: hub})

	const msgID = "6d11c235-5594-4a0f-aac7-952df0ec31f8"
	if err := peer.WriteJSON(map[string]any{
		"type": MsgTypeChat,
		"data": map[string]any{
			"msg_id":       msgID,
			"to_id":        42,
			"to_type":      ToTypeGroup,
			"content_type": ContentTypeText,
			"content":      "authorized group message",
		},
	}); err != nil {
		t.Fatalf("write authorized group message: %v", err)
	}

	if err := peer.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	var response struct {
		Type string `json:"type"`
		Data struct {
			MsgID  string `json:"msg_id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := peer.ReadJSON(&response); err != nil {
		t.Fatalf("read group message acknowledgement: %v", err)
	}
	if response.Type != MsgTypeChatAck {
		t.Fatalf("response type = %q, want %q", response.Type, MsgTypeChatAck)
	}
	if response.Data.MsgID != msgID || response.Data.Status != "sent" {
		t.Fatalf("unexpected acknowledgement: %+v", response.Data)
	}
}

func TestOnlyActiveMembersOfActiveGroupsAreAuthorized(t *testing.T) {
	tests := []struct {
		name   string
		access groupMessageAccess
		want   error
	}{
		{
			name: "active member of active group",
			access: groupMessageAccess{
				GroupStatus:  model.GroupStatusActive,
				MemberStatus: model.GroupMemberStatusActive,
			},
		},
		{
			name: "inactive member",
			access: groupMessageAccess{
				GroupStatus:  model.GroupStatusActive,
				MemberStatus: model.GroupMemberStatusInactive,
			},
			want: errGroupMessageForbidden,
		},
		{
			name: "dissolved group",
			access: groupMessageAccess{
				GroupStatus:  model.GroupStatusDissolved,
				MemberStatus: model.GroupMemberStatusActive,
			},
			want: errGroupMessageForbidden,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := authorizeGroupMessageAccess(test.access)
			if !errors.Is(err, test.want) {
				t.Fatalf("authorizeGroupMessageAccess() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestGroupAuthorizationFailsClosedWithoutDatabase(t *testing.T) {
	authorize := newGroupMessageAuthorizer(nil)
	err := authorize(context.Background(), 7, 42)
	if !errors.Is(err, errGroupAuthorizationUnavailable) {
		t.Fatalf("authorization error = %v, want %v", err, errGroupAuthorizationUnavailable)
	}
}

func websocketPairForAuthorization(t *testing.T) (*websocket.Conn, *websocket.Conn) {
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
