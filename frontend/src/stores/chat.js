import { computed, reactive, ref } from 'vue'
import { defineStore } from 'pinia'
import { getFriends } from '../api/friend'
import { getGroups } from '../api/group'
import { getHistory } from '../api/message'
import { useUserStore } from './user'

function chatKey(type, id) {
  return `${type}_${id}`
}

function messageTime(message) {
  if (message.timestamp) {
    return message.timestamp < 10000000000 ? message.timestamp * 1000 : message.timestamp
  }
  return message.created_at ? new Date(message.created_at).getTime() : Date.now()
}

export const useChatStore = defineStore('chat', () => {
  const userStore = useUserStore()
  const friends = ref([])
  const groups = ref([])
  const currentChat = ref(null)
  const chatMessages = reactive({})
  const historyCursors = reactive({})
  const unreadCounts = reactive({})
  const onlineUsers = reactive({})
  const sessions = computed(() => [...friends.value, ...groups.value])

  async function fetchFriends() {
    const { data } = await getFriends()
    friends.value = (data || []).map((friend) => ({ ...friend, type: 'user', online: Boolean(onlineUsers[friend.id]) }))
    return friends.value
  }

  async function fetchGroups() {
    const { data } = await getGroups()
    groups.value = (data || []).map((group) => ({ ...group, type: 'group', member_count: group.member_count || 0 }))
    return groups.value
  }

  function setCurrentChat(chat) {
    currentChat.value = chat
    unreadCounts[chatKey(chat.type, chat.id)] = 0
  }

  async function fetchHistory(targetId, targetType, page = 1) {
    const { data } = await getHistory({ target_id: targetId, target_type: targetType, page, page_size: 50 })
    const key = chatKey(targetType, targetId)
    const incoming = [...(data?.list || [])].reverse().map((message) => ({ ...message, timestamp: messageTime(message) }))
    if (page === 1) {
      const incomingIDs = new Set(incoming.map((message) => message.msg_id))
      const unsent = (chatMessages[key] || []).filter(
        (message) => ['sending', 'failed'].includes(message.local_status) && !incomingIDs.has(message.msg_id)
      )
      chatMessages[key] = [...incoming, ...unsent].sort((left, right) => left.timestamp - right.timestamp)
    } else {
      chatMessages[key] = [...incoming, ...(chatMessages[key] || [])]
    }
    updateHistoryCursor(key, data?.next_cursor)
    return data
  }

  async function syncHistory(targetId, targetType) {
    const key = chatKey(targetType, targetId)
    let added = 0
    let cursor = historyCursors[key] || '0'

    for (;;) {
      const { data } = await getHistory({
        target_id: targetId,
        target_type: targetType,
        after_id: cursor,
        page_size: 100
      })
      const list = chatMessages[key] || (chatMessages[key] = [])
      for (const message of data?.list || []) {
        const normalized = { ...message, timestamp: messageTime(message) }
        const existingIndex = list.findIndex((item) => item.msg_id === normalized.msg_id)
        if (existingIndex >= 0) {
          const existing = list[existingIndex]
          const merged = { ...existing, ...normalized }
          if (['sending', 'failed'].includes(existing.local_status)) {
            Object.assign(merged, {
              local_status: 'sent',
              local_error_code: null,
              local_error_message: ''
            })
          }
          list.splice(existingIndex, 1, merged)
        } else {
          list.push(normalized)
          added += 1
        }
      }
      list.sort((left, right) => left.timestamp - right.timestamp)
      const nextCursor = String(data?.next_cursor ?? cursor)
      updateHistoryCursor(key, nextCursor)
      if (!data?.has_more) return added
      if (BigInt(nextCursor) <= BigInt(cursor)) throw new Error('消息同步游标未推进')
      cursor = nextCursor
    }
  }

  async function syncLoadedHistories() {
    const targets = Object.keys(chatMessages).map((key) => {
      const separator = key.indexOf('_')
      return { key, type: key.slice(0, separator), id: Number(key.slice(separator + 1)) }
    })
    const settled = await Promise.allSettled(
      targets.map(async (target) => ({ ...target, added: await syncHistory(target.id, target.type) }))
    )
    const addedByKey = {}
    const failedKeys = []
    const activeKey = currentChat.value && chatKey(currentChat.value.type, currentChat.value.id)

    settled.forEach((result, index) => {
      const key = targets[index].key
      if (result.status === 'rejected') {
        failedKeys.push(key)
        return
      }
      addedByKey[key] = result.value.added
      if (key !== activeKey && result.value.added > 0) {
        unreadCounts[key] = (unreadCounts[key] || 0) + result.value.added
      }
    })

    return { addedByKey, failedKeys }
  }

  function updateHistoryCursor(key, cursor) {
    if (cursor === undefined || cursor === null || cursor === '') return
    const next = String(cursor)
    const current = historyCursors[key]
    if (!current || BigInt(next) > BigInt(current)) historyCursors[key] = next
  }

  function messageKey(message) {
    if (message.to_type === 'group') return chatKey('group', message.to_id)
    const selfId = userStore.userInfo?.id
    return chatKey('user', message.from_id === selfId ? message.to_id : message.from_id)
  }

  function addMessage(message, { incrementUnread = true } = {}) {
    const key = messageKey(message)
    const list = chatMessages[key] || (chatMessages[key] = [])
    const existingIndex = list.findIndex((item) => item.msg_id === message.msg_id)
    const added = existingIndex < 0
    const normalized = { ...message, timestamp: messageTime(message) }
    if (existingIndex >= 0) list.splice(existingIndex, 1, { ...list[existingIndex], ...normalized })
    else list.push(normalized)

    if (added && incrementUnread && key !== (currentChat.value && chatKey(currentChat.value.type, currentChat.value.id))) {
      unreadCounts[key] = (unreadCounts[key] || 0) + 1
    }
    return { key, added }
  }

  function updateMessage(msgId, patch) {
    for (const list of Object.values(chatMessages)) {
      const message = list.find((item) => item.msg_id === msgId)
      if (!message) continue
      Object.assign(message, patch)
      return message
    }
    return null
  }

  function setOnlineStatus(userId, online) {
    onlineUsers[userId] = online
    const friend = friends.value.find((item) => item.id === userId)
    if (friend) friend.online = online
  }

  function removeFriend(friendId) {
    friends.value = friends.value.filter((friend) => friend.id !== friendId)
    if (currentChat.value?.type === 'user' && currentChat.value.id === friendId) currentChat.value = null
    removeConversation('user', friendId)
  }

  function removeGroup(groupId) {
    groups.value = groups.value.filter((group) => group.id !== groupId)
    if (currentChat.value?.type === 'group' && currentChat.value.id === groupId) currentChat.value = null
    removeConversation('group', groupId)
  }

  function removeConversation(type, id) {
    const key = chatKey(type, id)
    delete chatMessages[key]
    delete historyCursors[key]
    delete unreadCounts[key]
  }

  return {
    friends, groups, sessions, currentChat, chatMessages, historyCursors, unreadCounts, onlineUsers,
    fetchFriends, fetchGroups, setCurrentChat, fetchHistory, syncHistory, syncLoadedHistories, addMessage, updateMessage,
    setOnlineStatus, removeFriend, removeGroup, chatKey
  }
})
