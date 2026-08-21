package ws

import (
	"context"
	"testing"
	"time"
)

func TestReplacingClientRemovesOldGroupMembership(t *testing.T) {
	hub := NewHub(nil)
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := hub.Shutdown(ctx); err != nil {
			t.Errorf("shutdown hub: %v", err)
		}
	})

	oldClient := NewClient(nil, 7, "alice", []uint{61})
	if err := hub.Register(oldClient); err != nil {
		t.Fatalf("register old client: %v", err)
	}
	newClient := NewClient(nil, 7, "alice", []uint{62})
	if err := hub.Register(newClient); err != nil {
		t.Fatalf("register new client: %v", err)
	}

	hub.mu.RLock()
	_, oldGroupExists := hub.groups[61]
	newGroupClient := hub.groups[62][7]
	hub.mu.RUnlock()

	if oldGroupExists {
		t.Fatal("old group membership remains after connection replacement")
	}
	if newGroupClient != newClient {
		t.Fatal("new group does not point to the replacement client")
	}
}
