package ws

func newMessageEnqueueErrorMessage(msgID string) *Message {
	return &Message{
		Type: MsgTypeError,
		Data: map[string]interface{}{
			"code":    errorCodeInternal,
			"message": "消息暂时无法处理，请稍后重试",
			"msg_id":  msgID,
		},
	}
}
