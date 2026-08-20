import { beforeEach, describe, expect, test, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { getHistory } from '../api/message'
import { useChatStore } from './chat'
import { useUserStore } from './user'

vi.mock('../api/friend', () => ({ getFriends: vi.fn() }))
vi.mock('../api/group', () => ({ getGroups: vi.fn() }))
vi.mock('../api/message', () => ({ getHistory: vi.fn() }))
vi.mock('../api/auth', () => ({ login: vi.fn(), register: vi.fn() }))
vi.mock('../api/user', () => ({ getProfile: vi.fn() }))

vi.stubGlobal('localStorage', {
  getItem: vi.fn(() => ''),
  setItem: vi.fn(),
  removeItem: vi.fn()
})

describe('chat history synchronization', () => {
  let chatStore

  beforeEach(() => {
    setActivePinia(createPinia())
    getHistory.mockReset()
    useUserStore().userInfo = { id: 7, nickname: 'Alice' }
    chatStore = useChatStore()
  })

  test('synchronizing a loaded conversation requests and merges messages after its cursor', async () => {
    getHistory
      .mockResolvedValueOnce({
        data: {
          list: [historyMessage(41, 'message-41')],
          next_cursor: '41',
          has_more: false
        }
      })
      .mockResolvedValueOnce({
        data: {
          list: [historyMessage(42, 'message-42')],
          next_cursor: '42',
          has_more: false
        }
      })

    await chatStore.fetchHistory(8, 'user')
    const added = await chatStore.syncHistory(8, 'user')

    expect(getHistory).toHaveBeenNthCalledWith(2, {
      target_id: 8,
      target_type: 'user',
      after_id: '41',
      page_size: 100
    })
    expect(added).toBe(1)
    expect(chatStore.chatMessages.user_8.map((message) => message.msg_id)).toEqual([
      'message-41',
      'message-42'
    ])
  })

  test('synchronization confirms persisted local messages and preserves unsaved failures', async () => {
    getHistory
      .mockResolvedValueOnce({
        data: { list: [], next_cursor: '40', has_more: false }
      })
      .mockResolvedValueOnce({
        data: {
          list: [{ ...historyMessage(41, 'persisted-41'), from_user_id: 7, to_id: 8 }],
          next_cursor: '41',
          has_more: false
        }
      })

    await chatStore.fetchHistory(8, 'user')
    chatStore.addMessage(localFailedMessage('persisted-41'), { incrementUnread: false })
    chatStore.addMessage(localFailedMessage('unsaved-42'), { incrementUnread: false })

    const added = await chatStore.syncHistory(8, 'user')
    const messages = chatStore.chatMessages.user_8

    expect(added).toBe(0)
    expect(messages).toHaveLength(2)
    expect(messages.find((message) => message.msg_id === 'persisted-41')).toMatchObject({
      local_status: 'sent',
      local_error_code: null,
      local_error_message: ''
    })
    expect(messages.find((message) => message.msg_id === 'unsaved-42')).toMatchObject({
      local_status: 'failed',
      local_error_code: 'timeout'
    })
  })

  test('refreshing the first history page keeps local messages that are still unsent', async () => {
    chatStore.addMessage(localFailedMessage('unsaved-40'), { incrementUnread: false })
    getHistory.mockResolvedValueOnce({
      data: {
        list: [historyMessage(41, 'message-41')],
        next_cursor: '41',
        has_more: false
      }
    })

    await chatStore.fetchHistory(8, 'user')

    expect(chatStore.chatMessages.user_8.map((message) => message.msg_id)).toEqual([
      'message-41',
      'unsaved-40'
    ])
    expect(chatStore.chatMessages.user_8.at(-1).local_status).toBe('failed')
  })

  test('synchronization follows the cursor until all missed messages are loaded', async () => {
    getHistory
      .mockResolvedValueOnce({
        data: { list: [], next_cursor: '40', has_more: false }
      })
      .mockResolvedValueOnce({
        data: { list: [historyMessage(41, 'message-41')], next_cursor: '41', has_more: true }
      })
      .mockResolvedValueOnce({
        data: { list: [historyMessage(42, 'message-42')], next_cursor: '42', has_more: false }
      })

    await chatStore.fetchHistory(8, 'user')
    const added = await chatStore.syncHistory(8, 'user')

    expect(added).toBe(2)
    expect(getHistory).toHaveBeenNthCalledWith(3, {
      target_id: 8,
      target_type: 'user',
      after_id: '41',
      page_size: 100
    })
    expect(chatStore.historyCursors.user_8).toBe('42')
    expect(chatStore.chatMessages.user_8.map((message) => message.msg_id)).toEqual([
      'message-41',
      'message-42'
    ])
  })

  test('an in-flight synchronization merges into the latest conversation state', async () => {
    let resolveSynchronization
    getHistory
      .mockResolvedValueOnce({ data: { list: [], next_cursor: '40', has_more: false } })
      .mockImplementationOnce(() => new Promise((resolve) => { resolveSynchronization = resolve }))
      .mockResolvedValueOnce({
        data: { list: [historyMessage(42, 'message-42')], next_cursor: '42', has_more: false }
      })

    await chatStore.fetchHistory(8, 'user')
    const synchronization = chatStore.syncHistory(8, 'user')
    await chatStore.fetchHistory(8, 'user')
    resolveSynchronization({
      data: { list: [historyMessage(41, 'message-41')], next_cursor: '41', has_more: false }
    })
    await synchronization

    expect(chatStore.historyCursors.user_8).toBe('42')
    expect(chatStore.chatMessages.user_8.map((message) => message.msg_id)).toEqual([
      'message-41',
      'message-42'
    ])
  })

  test('reconnect synchronization updates every loaded conversation and unread count', async () => {
    getHistory
      .mockResolvedValueOnce({ data: { list: [], next_cursor: '10', has_more: false } })
      .mockResolvedValueOnce({ data: { list: [], next_cursor: '20', has_more: false } })
      .mockResolvedValueOnce({
        data: { list: [historyMessage(11, 'private-11')], next_cursor: '11', has_more: false }
      })
      .mockResolvedValueOnce({
        data: {
          list: [{ ...historyMessage(21, 'group-21'), to_id: 9, to_type: 'group' }],
          next_cursor: '21',
          has_more: false
        }
      })

    await chatStore.fetchHistory(8, 'user')
    await chatStore.fetchHistory(9, 'group')
    chatStore.setCurrentChat({ id: 8, type: 'user' })

    const result = await chatStore.syncLoadedHistories()

    expect(result).toEqual({
      addedByKey: { user_8: 1, group_9: 1 },
      failedKeys: []
    })
    expect(chatStore.unreadCounts.user_8).toBe(0)
    expect(chatStore.unreadCounts.group_9).toBe(1)
    expect(chatStore.chatMessages.user_8.at(-1).msg_id).toBe('private-11')
    expect(chatStore.chatMessages.group_9.at(-1).msg_id).toBe('group-21')
  })

  test('a duplicate real-time message is not counted as unread twice', () => {
    const message = {
      msg_id: 'message-41',
      from_id: 8,
      to_id: 7,
      to_type: 'user',
      content_type: 'text',
      content: 'hello',
      timestamp: Date.now()
    }

    expect(chatStore.addMessage(message)).toEqual({ key: 'user_8', added: true })
    expect(chatStore.addMessage(message)).toEqual({ key: 'user_8', added: false })
    expect(chatStore.chatMessages.user_8).toHaveLength(1)
    expect(chatStore.unreadCounts.user_8).toBe(1)
  })

  test('removing a group also removes its synchronization state', async () => {
    getHistory.mockResolvedValueOnce({
      data: {
        list: [{ ...historyMessage(21, 'group-21'), to_id: 9, to_type: 'group' }],
        next_cursor: '21',
        has_more: false
      }
    })
    await chatStore.fetchHistory(9, 'group')
    chatStore.unreadCounts.group_9 = 3

    chatStore.removeGroup(9)

    expect(chatStore.chatMessages.group_9).toBeUndefined()
    expect(chatStore.historyCursors.group_9).toBeUndefined()
    expect(chatStore.unreadCounts.group_9).toBeUndefined()
  })
})

function historyMessage(id, msgId) {
  return {
    id,
    msg_id: msgId,
    from_user_id: 8,
    to_id: 7,
    to_type: 'user',
    content_type: 'text',
    content: msgId,
    created_at: `2026-08-20T12:00:${id}.000Z`
  }
}

function localFailedMessage(msgId) {
  return {
    msg_id: msgId,
    from_id: 7,
    to_id: 8,
    to_type: 'user',
    content_type: 'text',
    content: msgId,
    timestamp: Date.now(),
    local_status: 'failed',
    local_error_code: 'timeout',
    local_error_message: '发送超时，请重试'
  }
}
