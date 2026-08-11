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
    chatMessages[key] = page === 1 ? incoming : [...incoming, ...(chatMessages[key] || [])]
    return data
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
    const normalized = { ...message, timestamp: messageTime(message) }
    if (existingIndex >= 0) list.splice(existingIndex, 1, { ...list[existingIndex], ...normalized })
    else list.push(normalized)

    if (incrementUnread && key !== (currentChat.value && chatKey(currentChat.value.type, currentChat.value.id))) {
      unreadCounts[key] = (unreadCounts[key] || 0) + 1
    }
    return key
  }

  function setOnlineStatus(userId, online) {
    onlineUsers[userId] = online
    const friend = friends.value.find((item) => item.id === userId)
    if (friend) friend.online = online
  }

  function removeFriend(friendId) {
    friends.value = friends.value.filter((friend) => friend.id !== friendId)
    if (currentChat.value?.type === 'user' && currentChat.value.id === friendId) currentChat.value = null
  }

  function removeGroup(groupId) {
    groups.value = groups.value.filter((group) => group.id !== groupId)
    if (currentChat.value?.type === 'group' && currentChat.value.id === groupId) currentChat.value = null
  }

  return {
    friends, groups, sessions, currentChat, chatMessages, unreadCounts, onlineUsers,
    fetchFriends, fetchGroups, setCurrentChat, fetchHistory, addMessage,
    setOnlineStatus, removeFriend, removeGroup, chatKey
  }
})
