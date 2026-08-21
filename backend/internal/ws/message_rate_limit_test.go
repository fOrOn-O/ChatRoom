package ws

import (
	"testing"
	"time"

	"ChatRoom/internal/messagequeue"
	"ChatRoom/internal/ratelimit"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestMessageRateLimitRejectsWithoutPublishingOrClosingConnection(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		if err := redisClient.Close(); err != nil {
			t.Errorf("关闭 Redis 测试客户端失败: %v", err)
		}
	})
	limiter, err := ratelimit.NewMessageLimiter(redisClient, ratelimit.MessageOptions{
		KeyPrefix: "chatroom",
		Limit:     1,
		Window:    10 * time.Second,
	})
	if err != nil {
		t.Fatalf("创建消息限流器失败: %v", err)
	}

	hub := NewHub(nil)
	publisher := &recordingMessagePublisher{messages: make(chan messagequeue.ChatMessage, 2)}
	peer, server := websocketPairForAuthorization(t)
	client := NewClient(server, 7, "alice")
	go client.WritePump()
	go client.ReadPump(hub, publisher, limiter)

	writePrivateChatForRateLimit(t, peer, "message-limit-first")
	select {
	case published := <-publisher.messages:
		if published.MsgID != "message-limit-first" {
			t.Fatalf("首次发布消息编号 = %q，期望 %q", published.MsgID, "message-limit-first")
		}
	case <-time.After(time.Second):
		t.Fatal("额度内消息未发布")
	}

	writePrivateChatForRateLimit(t, peer, "message-limit-rejected")
	if err := peer.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("设置 WebSocket 读取超时失败: %v", err)
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
		t.Fatalf("读取消息限流响应失败: %v", err)
	}
	if response.Type != MsgTypeError || response.Data.Code != errorCodeRateLimited ||
		response.Data.Message != "消息发送过于频繁，请稍后重试" || response.Data.MsgID != "message-limit-rejected" {
		t.Fatalf("消息限流响应 = %+v", response)
	}
	select {
	case published := <-publisher.messages:
		t.Fatalf("超限消息仍被发布: %+v", published)
	case <-time.After(150 * time.Millisecond):
	}

	redisServer.FastForward(10 * time.Second)
	writePrivateChatForRateLimit(t, peer, "message-limit-recovered")
	select {
	case published := <-publisher.messages:
		if published.MsgID != "message-limit-recovered" {
			t.Fatalf("窗口恢复后的消息编号 = %q，期望 %q", published.MsgID, "message-limit-recovered")
		}
	case <-time.After(time.Second):
		t.Fatal("窗口恢复后同一连接未能继续发布消息")
	}
}

func TestMessagePublishesWhenRateLimiterIsUnavailable(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	limiter, err := ratelimit.NewMessageLimiter(redisClient, ratelimit.MessageOptions{
		KeyPrefix: "chatroom",
		Limit:     1,
		Window:    10 * time.Second,
	})
	if err != nil {
		t.Fatalf("创建消息限流器失败: %v", err)
	}
	if err := redisClient.Close(); err != nil {
		t.Fatalf("关闭 Redis 测试客户端失败: %v", err)
	}

	hub := NewHub(nil)
	publisher := &recordingMessagePublisher{messages: make(chan messagequeue.ChatMessage, 1)}
	peer, server := websocketPairForAuthorization(t)
	client := NewClient(server, 7, "alice")
	go client.WritePump()
	go client.ReadPump(hub, publisher, limiter)

	writePrivateChatForRateLimit(t, peer, "message-limit-unavailable")
	select {
	case published := <-publisher.messages:
		if published.MsgID != "message-limit-unavailable" {
			t.Fatalf("限流器不可用时发布消息编号 = %q，期望 %q", published.MsgID, "message-limit-unavailable")
		}
	case <-time.After(time.Second):
		t.Fatal("限流器不可用时消息未继续发布")
	}
}

func writePrivateChatForRateLimit(t *testing.T, peer interface{ WriteJSON(any) error }, msgID string) {
	t.Helper()
	if err := peer.WriteJSON(map[string]any{
		"type": MsgTypeChat,
		"data": map[string]any{
			"msg_id":       msgID,
			"to_id":        8,
			"to_type":      ToTypeUser,
			"content_type": ContentTypeText,
			"content":      "rate limited message",
		},
	}); err != nil {
		t.Fatalf("写入 WebSocket 聊天消息失败: %v", err)
	}
}
