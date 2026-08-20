export const MESSAGE_ACK_TIMEOUT = 10_000

export function createMessageDeliveryController({
  send,
  createId,
  updateMessage,
  timeoutMs = MESSAGE_ACK_TIMEOUT,
  setTimer = globalThis.setTimeout,
  clearTimer = globalThis.clearTimeout
}) {
  const pending = new Map()

  function clearPending(msgId) {
    const timer = pending.get(msgId)
    if (timer === undefined) return false
    clearTimer(timer)
    pending.delete(msgId)
    return true
  }

  function track(msgId) {
    clearPending(msgId)
    updateMessage(msgId, {
      local_status: 'sending',
      local_error_code: null,
      local_error_message: ''
    })
    const timer = setTimer(() => {
      pending.delete(msgId)
      updateMessage(msgId, {
        local_status: 'failed',
        local_error_code: 'timeout',
        local_error_message: '发送超时，请重试'
      })
    }, timeoutMs)
    pending.set(msgId, timer)
  }

  function acknowledge(ack) {
    if (ack?.status !== 'sent' || !ack.msg_id) return false
    clearPending(ack.msg_id)
    return Boolean(updateMessage(ack.msg_id, {
      local_status: 'sent',
      local_error_code: null,
      local_error_message: ''
    }))
  }

  function reject(error) {
    if (!error?.msg_id) return false
    clearPending(error.msg_id)
    return Boolean(updateMessage(error.msg_id, {
      local_status: 'failed',
      local_error_code: error.code ?? null,
      local_error_message: error.message || '消息发送失败，请稍后重试'
    }))
  }

  function retry(message) {
    if (!message?.msg_id || message.local_status !== 'failed') return ''
    const previousMsgId = message.msg_id
    const msgId = Number(message.local_error_code) === 1001 ? createId() : previousMsgId
    const sentId = send({
      msgId,
      toId: message.to_id,
      toType: message.to_type,
      contentType: message.content_type,
      content: message.content
    })
    if (!sentId) return ''
    if (sentId !== previousMsgId) {
      clearPending(previousMsgId)
      updateMessage(previousMsgId, { msg_id: sentId })
    }
    track(sentId)
    return sentId
  }

  function failPending(message = '连接已断开，请重试') {
    pending.forEach((timer, msgId) => {
      clearTimer(timer)
      updateMessage(msgId, {
        local_status: 'failed',
        local_error_code: 'disconnected',
        local_error_message: message
      })
    })
    pending.clear()
  }

  function dispose() {
    failPending()
  }

  return { track, acknowledge, reject, retry, failPending, dispose }
}
