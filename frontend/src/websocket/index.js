import { ref, shallowRef } from 'vue'

export const CLOSE_CODE_CONNECTION_REPLACED = 4001

const socket = shallowRef(null)
const connected = ref(false)
const callbacks = {
  message: new Set(),
  onlineStatus: new Set(),
  ack: new Set(),
  error: new Set(),
  reconnected: new Set(),
  sessionReplaced: new Set()
}

let reconnectTimer = null
let reconnectAttempts = 0
let shouldReconnect = false
let activeToken = ''
let hasOpenedConnection = false

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
    const recovered = hasOpenedConnection
    hasOpenedConnection = true
    connected.value = true
    reconnectAttempts = 0
    if (recovered) notify('reconnected')
  }

  ws.onmessage = (event) => {
    try {
      const message = JSON.parse(event.data)
      if (message.type === 'chat') notify('message', message)
      if (message.type === 'online_status') notify('onlineStatus', message.data || message)
      if (message.type === 'chat_ack') notify('ack', message.data || message)
      if (message.type === 'error') notify('error', message.data || message)
    } catch {
      // 无法解析的帧不应中断当前连接。
    }
  }

  ws.onclose = (event) => {
    if (socket.value === ws) {
      socket.value = null
      connected.value = false

      if (event.code === CLOSE_CODE_CONNECTION_REPLACED) {
        shouldReconnect = false
        reconnectAttempts = 0
        if (reconnectTimer) window.clearTimeout(reconnectTimer)
        reconnectTimer = null
        notify('sessionReplaced', { code: event.code, reason: event.reason })
        return
      }

      scheduleReconnect()
    }
  }

  ws.onerror = () => ws.close()
}

export function disconnect() {
  shouldReconnect = false
  activeToken = ''
  hasOpenedConnection = false
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

export function subscribe(type, callback) {
  callbacks[type]?.add(callback)
  return () => callbacks[type]?.delete(callback)
}

export { connected }
