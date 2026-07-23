import { ref } from 'vue'

const socket = ref(null)
const connected = ref(false)
const callbacks = {
  message: new Set(),
  onlineStatus: new Set(),
  typing: new Set(),
  ack: new Set()
}

let reconnectTimer = null
let reconnectAttempts = 0
let shouldReconnect = false
let activeToken = ''

function websocketUrl(token) {
  if (import.meta.env.VITE_WS_URL) {
    return `${import.meta.env.VITE_WS_URL}?token=${encodeURIComponent(token)}`
  }

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${window.location.host}/ws?token=${encodeURIComponent(token)}`
}

function notify(type, payload) {
  callbacks[type].forEach((callback) => callback(payload))
}

function scheduleReconnect() {
  if (!shouldReconnect || reconnectTimer) return
  const delay = Math.min(1000 * 2 ** reconnectAttempts, 10000)
  reconnectAttempts += 1
  reconnectTimer = window.setTimeout(() => {
    reconnectTimer = null
    connect(activeToken)
  }, delay)
}

export function connect(token) {
  if (!token || (socket.value && [WebSocket.CONNECTING, WebSocket.OPEN].includes(socket.value.readyState))) {
    return
  }

  activeToken = token
  shouldReconnect = true
  const ws = new WebSocket(websocketUrl(token))
  socket.value = ws

  ws.onopen = () => {
    if (socket.value !== ws) return
    connected.value = true
    reconnectAttempts = 0
  }

  ws.onmessage = (event) => {
    try {
      const message = JSON.parse(event.data)
      if (message.type === 'chat') notify('message', message)
      if (message.type === 'online_status') notify('onlineStatus', message.data || message)
      if (message.type === 'typing') notify('typing', message)
      if (message.type === 'chat_ack') notify('ack', message.data || message)
    } catch {
      // 无法解析的帧不应中断当前连接。
    }
  }

  ws.onclose = () => {
    if (socket.value === ws) {
      socket.value = null
      connected.value = false
      scheduleReconnect()
    }
  }

  ws.onerror = () => ws.close()
}

export function disconnect() {
  shouldReconnect = false
  activeToken = ''
  if (reconnectTimer) window.clearTimeout(reconnectTimer)
  reconnectTimer = null
  reconnectAttempts = 0
  socket.value?.close()
  socket.value = null
  connected.value = false
}

export function sendMessage(message) {
  if (!socket.value || socket.value.readyState !== WebSocket.OPEN) return false
  socket.value.send(JSON.stringify(message))
  return true
}

export function createMessageId() {
  return crypto.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2, 11)}`
}

export function sendChatMessage({ msgId = createMessageId(), toId, toType, contentType, content }) {
  const sent = sendMessage({
    type: 'chat',
    data: {
      msg_id: msgId,
      to_id: toId,
      to_type: toType,
      content_type: contentType,
      content
    }
  })
  return sent ? msgId : ''
}

export function sendReadReceipt(targetId, targetType, lastMsgId) {
  return sendMessage({
    type: 'read_receipt',
    data: { target_id: targetId, target_type: targetType, last_msg_id: lastMsgId }
  })
}

export function sendTyping(targetId, targetType) {
  return sendMessage({
    type: 'typing',
    data: { target_id: targetId, target_type: targetType }
  })
}

export function subscribe(type, callback) {
  callbacks[type]?.add(callback)
  return () => callbacks[type]?.delete(callback)
}

export { connected }
