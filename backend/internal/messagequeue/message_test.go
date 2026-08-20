package messagequeue

import (
	"errors"
	"reflect"
	"testing"
)

func TestChatMessageCanBeEncodedAndDecoded(t *testing.T) {
	want := validChatMessage()

	payload, err := EncodeChatMessage(want)
	if err != nil {
		t.Fatalf("编码聊天消息失败: %v", err)
	}

	got, err := DecodeChatMessage(payload)
	if err != nil {
		t.Fatalf("解码聊天消息失败: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("解码结果 = %+v，期望 %+v", got, want)
	}
}

func TestEncodeChatMessageRejectsMissingMessageID(t *testing.T) {
	message := validChatMessage()
	message.MsgID = ""

	_, err := EncodeChatMessage(message)
	if !errors.Is(err, ErrInvalidChatMessage) {
		t.Fatalf("编码缺少消息编号的消息时错误 = %v，期望 ErrInvalidChatMessage", err)
	}
}

func TestChatMessageValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ChatMessage)
	}{
		{name: "协议版本缺失", mutate: func(message *ChatMessage) { message.Version = 0 }},
		{name: "发送者缺失", mutate: func(message *ChatMessage) { message.FromID = 0 }},
		{name: "发送者名称缺失", mutate: func(message *ChatMessage) { message.FromName = "" }},
		{name: "接收目标缺失", mutate: func(message *ChatMessage) { message.ToID = 0 }},
		{name: "接收类型不支持", mutate: func(message *ChatMessage) { message.ToType = "channel" }},
		{name: "内容类型不支持", mutate: func(message *ChatMessage) { message.ContentType = "voice" }},
		{name: "消息内容缺失", mutate: func(message *ChatMessage) { message.Content = "" }},
		{name: "时间戳缺失", mutate: func(message *ChatMessage) { message.Timestamp = 0 }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			message := validChatMessage()
			test.mutate(&message)

			if err := message.Validate(); !errors.Is(err, ErrInvalidChatMessage) {
				t.Fatalf("Validate() 错误 = %v，期望 ErrInvalidChatMessage", err)
			}
		})
	}
}

func TestDecodeChatMessageRejectsUnsupportedVersion(t *testing.T) {
	payload := []byte(`{"version":2,"msg_id":"59ffb838-6a50-4d9c-94e7-12cdd75269a1","from_id":7,"from_name":"alice","to_id":8,"to_type":"user","content_type":"text","content":"你好","timestamp":1787241600}`)

	_, err := DecodeChatMessage(payload)
	if !errors.Is(err, ErrUnsupportedChatMessageVersion) {
		t.Fatalf("解码未知版本消息时错误 = %v，期望 ErrUnsupportedChatMessageVersion", err)
	}
}

func validChatMessage() ChatMessage {
	return ChatMessage{
		Version:     ChatMessageVersion,
		MsgID:       "59ffb838-6a50-4d9c-94e7-12cdd75269a1",
		FromID:      7,
		FromName:    "alice",
		ToID:        8,
		ToType:      "user",
		ContentType: "text",
		Content:     "你好",
		Timestamp:   1787241600,
	}
}
