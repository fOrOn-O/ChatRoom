import { describe, expect, it, vi } from 'vitest'
import {
  CHAT_THEME_STORAGE_KEY,
  normalizeChatTheme,
  readStoredChatTheme,
  writeStoredChatTheme
} from './index'

describe('workspace theme preferences', () => {
  it('accepts supported themes and falls back to minimal', () => {
    expect(normalizeChatTheme('glass')).toBe('glass')
    expect(normalizeChatTheme('neumorphic')).toBe('neumorphic')
    expect(normalizeChatTheme('unknown')).toBe('minimal')
  })

  it('reads and writes a valid persisted theme', () => {
    const storage = {
      getItem: vi.fn(() => 'glass'),
      setItem: vi.fn()
    }

    expect(readStoredChatTheme(storage)).toBe('glass')
    expect(writeStoredChatTheme('neumorphic', storage)).toBe('neumorphic')
    expect(storage.setItem).toHaveBeenCalledWith(CHAT_THEME_STORAGE_KEY, 'neumorphic')
  })

  it('keeps the interface usable when storage is unavailable', () => {
    const storage = {
      getItem: vi.fn(() => { throw new Error('blocked') }),
      setItem: vi.fn(() => { throw new Error('blocked') })
    }

    expect(readStoredChatTheme(storage)).toBe('minimal')
    expect(writeStoredChatTheme('glass', storage)).toBe('glass')
  })
})
