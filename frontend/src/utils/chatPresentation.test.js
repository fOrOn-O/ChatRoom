import { describe, expect, it } from 'vitest'
import {
  canGroupMessages,
  formatDateDivider,
  formatFullTime,
  isoTimestamp,
  messageAuthorKey,
  shouldShowDateDivider
} from './chatPresentation'

describe('chat presentation helpers', () => {
  const morning = new Date(2026, 6, 31, 9, 30).getTime()

  it('groups consecutive messages from the same author within five minutes', () => {
    const first = { from_id: 2, timestamp: morning, status: 1 }
    const close = { from_id: 2, timestamp: morning + 4 * 60 * 1000, status: 1 }
    const late = { from_id: 2, timestamp: morning + 6 * 60 * 1000, status: 1 }

    expect(canGroupMessages(first, close, 1)).toBe(true)
    expect(canGroupMessages(first, late, 1)).toBe(false)
    expect(canGroupMessages(first, { ...close, from_id: 3 }, 1)).toBe(false)
  })

  it('keeps recalled and cross-day messages separate', () => {
    const first = { from_id: 2, timestamp: morning, status: 1 }
    const recalled = { from_id: 2, timestamp: morning + 1000, status: 2 }
    const nextDay = { from_id: 2, timestamp: morning + 24 * 60 * 60 * 1000, status: 1 }

    expect(canGroupMessages(first, recalled, 1)).toBe(false)
    expect(canGroupMessages(first, nextDay, 1)).toBe(false)
  })

  it('creates stable author, date-divider and timestamp values', () => {
    const messages = [
      { from_id: 1, timestamp: morning },
      { from_id: 1, timestamp: morning + 1000 },
      { from_id: 2, timestamp: morning + 24 * 60 * 60 * 1000 }
    ]

    expect(messageAuthorKey(messages[0], 1)).toBe('self')
    expect(shouldShowDateDivider(messages, 0)).toBe(true)
    expect(shouldShowDateDivider(messages, 1)).toBe(false)
    expect(shouldShowDateDivider(messages, 2)).toBe(true)
    expect(formatDateDivider(morning, morning)).toBe('今天')
    expect(formatFullTime(morning)).toMatch(/^\d{2}:\d{2}$/)
    expect(isoTimestamp(morning)).toBe(new Date(morning).toISOString())
  })
})
