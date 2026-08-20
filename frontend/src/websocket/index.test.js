import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

class FakeWebSocket {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3
  static instances = []

  constructor(url) {
    this.url = url
    this.readyState = FakeWebSocket.CONNECTING
    FakeWebSocket.instances.push(this)
  }

  open() {
    this.readyState = FakeWebSocket.OPEN
    this.onopen?.()
  }

  close(code = 1000, reason = '') {
    this.serverClose(code, reason)
  }

  serverClose(code, reason = '') {
    if (this.readyState === FakeWebSocket.CLOSED) return
    this.readyState = FakeWebSocket.CLOSED
    this.onclose?.({ code, reason })
  }

  send() {}

  receive(message) {
    this.onmessage?.({ data: JSON.stringify(message) })
  }
}

vi.stubGlobal('window', {
  location: { protocol: 'http:', host: 'chat.test' },
  setTimeout: (...args) => globalThis.setTimeout(...args),
  clearTimeout: (...args) => globalThis.clearTimeout(...args)
})
vi.stubGlobal('WebSocket', FakeWebSocket)

const websocket = await import('./index.js')

describe('WebSocket reconnect lifecycle', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    FakeWebSocket.instances = []
  })

  afterEach(() => {
    websocket.disconnect()
    vi.clearAllTimers()
    vi.useRealTimers()
  })

  test('a replaced session stops reconnecting and notifies the UI', () => {
    const onSessionReplaced = vi.fn()
    const unsubscribe = websocket.subscribe('sessionReplaced', onSessionReplaced)

    websocket.connect('token')
    const connection = FakeWebSocket.instances[0]
    connection.open()
    connection.serverClose(4001, 'connection replaced')

    expect(websocket.connected.value).toBe(false)
    expect(onSessionReplaced).toHaveBeenCalledWith({ code: 4001, reason: 'connection replaced' })
    vi.advanceTimersByTime(30_000)
    expect(FakeWebSocket.instances).toHaveLength(1)

    unsubscribe()
  })

  test('an abnormal close keeps the exponential reconnect behavior', () => {
    websocket.connect('token')
    const connection = FakeWebSocket.instances[0]
    connection.open()
    connection.serverClose(1006)

    vi.advanceTimersByTime(999)
    expect(FakeWebSocket.instances).toHaveLength(1)
    vi.advanceTimersByTime(1)
    expect(FakeWebSocket.instances).toHaveLength(2)
  })

  test('only a recovered connection emits the reconnected event', () => {
    const onReconnected = vi.fn()
    const unsubscribe = websocket.subscribe('reconnected', onReconnected)

    websocket.connect('token')
    const firstConnection = FakeWebSocket.instances[0]
    firstConnection.open()
    expect(onReconnected).not.toHaveBeenCalled()

    firstConnection.serverClose(1006)
    vi.advanceTimersByTime(1_000)
    const secondConnection = FakeWebSocket.instances[1]
    secondConnection.open()

    expect(onReconnected).toHaveBeenCalledOnce()
    unsubscribe()
  })

  test('disconnect cancels a pending reconnect', () => {
    websocket.connect('token')
    const connection = FakeWebSocket.instances[0]
    connection.open()
    connection.serverClose(1006)

    websocket.disconnect()
    vi.advanceTimersByTime(30_000)

    expect(FakeWebSocket.instances).toHaveLength(1)
  })

  test('error frames notify subscribers with the original message ID', () => {
    const onError = vi.fn()
    const unsubscribe = websocket.subscribe('error', onError)

    websocket.connect('token')
    const connection = FakeWebSocket.instances[0]
    connection.open()
    connection.receive({
      type: 'error',
      data: { code: 1005, message: '消息暂时无法处理', msg_id: 'message-1' }
    })

    expect(onError).toHaveBeenCalledWith({
      code: 1005,
      message: '消息暂时无法处理',
      msg_id: 'message-1'
    })

    unsubscribe()
  })
})
