import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'
import { createMessageDeliveryController } from './messageDelivery'

describe('message delivery controller', () => {
  let message
  let updateMessage

  beforeEach(() => {
    vi.useFakeTimers()
    message = { msg_id: 'message-1', local_status: 'sending' }
    updateMessage = vi.fn((msgId, patch) => {
      if (message.msg_id !== msgId) return null
      Object.assign(message, patch)
      return message
    })
  })

  afterEach(() => {
    vi.clearAllTimers()
    vi.useRealTimers()
  })

  test('an acknowledgement marks a tracked message as sent', () => {
    const delivery = createMessageDeliveryController({
      send: vi.fn(),
      createId: vi.fn(),
      updateMessage
    })

    delivery.track(message.msg_id)
    expect(delivery.acknowledge({ msg_id: message.msg_id, status: 'sent' })).toBe(true)
    expect(message).toMatchObject({
      local_status: 'sent',
      local_error_code: null,
      local_error_message: ''
    })
  })

  test('a message without an acknowledgement fails after the timeout', () => {
    const delivery = createMessageDeliveryController({
      send: vi.fn(),
      createId: vi.fn(),
      updateMessage,
      timeoutMs: 10_000
    })

    delivery.track(message.msg_id)
    vi.advanceTimersByTime(9_999)
    expect(message.local_status).toBe('sending')

    vi.advanceTimersByTime(1)
    expect(message).toMatchObject({
      local_status: 'failed',
      local_error_code: 'timeout',
      local_error_message: '发送超时，请重试'
    })
  })

  test('a late acknowledgement recovers a timed-out message', () => {
    const delivery = createMessageDeliveryController({
      send: vi.fn(),
      createId: vi.fn(),
      updateMessage,
      timeoutMs: 10_000
    })

    delivery.track(message.msg_id)
    vi.advanceTimersByTime(10_000)
    expect(message.local_status).toBe('failed')

    expect(delivery.acknowledge({ msg_id: message.msg_id, status: 'sent' })).toBe(true)
    expect(message).toMatchObject({
      local_status: 'sent',
      local_error_code: null,
      local_error_message: ''
    })
  })

  test('a server error fails the matching message and cancels its timeout', () => {
    const delivery = createMessageDeliveryController({
      send: vi.fn(),
      createId: vi.fn(),
      updateMessage,
      timeoutMs: 10_000
    })

    delivery.track(message.msg_id)
    expect(delivery.reject({
      msg_id: message.msg_id,
      code: 1005,
      message: '消息暂时无法处理，请稍后重试'
    })).toBe(true)
    expect(message).toMatchObject({
      local_status: 'failed',
      local_error_code: 1005,
      local_error_message: '消息暂时无法处理，请稍后重试'
    })

    vi.advanceTimersByTime(10_000)
    expect(message.local_error_code).toBe(1005)
  })

  test('retrying a temporary failure reuses the original message ID', () => {
    Object.assign(message, {
      to_id: 8,
      to_type: 'user',
      content_type: 'text',
      content: 'hello',
      local_status: 'failed',
      local_error_code: 1005,
      local_error_message: '消息暂时无法处理'
    })
    const send = vi.fn(({ msgId }) => msgId)
    const delivery = createMessageDeliveryController({
      send,
      createId: vi.fn(() => 'message-2'),
      updateMessage
    })

    expect(delivery.retry(message)).toBe('message-1')
    expect(send).toHaveBeenCalledWith({
      msgId: 'message-1',
      toId: 8,
      toType: 'user',
      contentType: 'text',
      content: 'hello'
    })
    expect(message).toMatchObject({
      msg_id: 'message-1',
      local_status: 'sending',
      local_error_code: null,
      local_error_message: ''
    })
  })

  test('retrying a message ID conflict generates a new ID for the same bubble', () => {
    Object.assign(message, {
      to_id: 8,
      to_type: 'user',
      content_type: 'text',
      content: 'hello',
      local_status: 'failed',
      local_error_code: 1001,
      local_error_message: '消息编号冲突'
    })
    const send = vi.fn(({ msgId }) => msgId)
    const createId = vi.fn(() => 'message-2')
    const delivery = createMessageDeliveryController({ send, createId, updateMessage })

    expect(delivery.retry(message)).toBe('message-2')
    expect(createId).toHaveBeenCalledOnce()
    expect(send).toHaveBeenCalledWith({
      msgId: 'message-2',
      toId: 8,
      toType: 'user',
      contentType: 'text',
      content: 'hello'
    })
    expect(message).toMatchObject({
      msg_id: 'message-2',
      local_status: 'sending',
      local_error_code: null,
      local_error_message: ''
    })
  })

  test('disposing marks pending messages as failed and clears their timers', () => {
    const delivery = createMessageDeliveryController({
      send: vi.fn(),
      createId: vi.fn(),
      updateMessage,
      timeoutMs: 10_000
    })

    delivery.track(message.msg_id)
    delivery.dispose()
    expect(message).toMatchObject({
      local_status: 'failed',
      local_error_code: 'disconnected',
      local_error_message: '连接已断开，请重试'
    })

    vi.advanceTimersByTime(10_000)
    expect(message.local_error_code).toBe('disconnected')
  })
})
