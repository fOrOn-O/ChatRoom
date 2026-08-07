const GROUP_WINDOW_MS = 5 * 60 * 1000

const timeFormatter = new Intl.DateTimeFormat('zh-CN', {
  hour: '2-digit',
  minute: '2-digit',
  hourCycle: 'h23'
})

const compactDateFormatter = new Intl.DateTimeFormat('zh-CN', {
  month: '2-digit',
  day: '2-digit'
})

const fullDateFormatter = new Intl.DateTimeFormat('zh-CN', {
  month: 'long',
  day: 'numeric',
  weekday: 'short'
})

function validDate(timestamp) {
  const date = new Date(timestamp)
  return Number.isNaN(date.getTime()) ? null : date
}

function isSameDay(left, right) {
  return left.getFullYear() === right.getFullYear()
    && left.getMonth() === right.getMonth()
    && left.getDate() === right.getDate()
}

export function messageAuthorKey(message, selfId) {
  const senderId = message?.from_id ?? message?.from_user_id
  return senderId === selfId ? 'self' : `user:${senderId ?? 'unknown'}`
}

export function canGroupMessages(previous, current, selfId) {
  if (!previous || !current || previous.status === 2 || current.status === 2) return false
  const previousDate = validDate(previous.timestamp)
  const currentDate = validDate(current.timestamp)
  if (!previousDate || !currentDate || !isSameDay(previousDate, currentDate)) return false
  return messageAuthorKey(previous, selfId) === messageAuthorKey(current, selfId)
    && Math.abs(currentDate.getTime() - previousDate.getTime()) <= GROUP_WINDOW_MS
}

export function formatShortTime(timestamp, now = Date.now()) {
  const date = validDate(timestamp)
  const today = validDate(now)
  if (!date || !today) return ''
  return isSameDay(date, today) ? timeFormatter.format(date) : compactDateFormatter.format(date)
}

export function formatFullTime(timestamp) {
  const date = validDate(timestamp)
  if (!date) return ''
  return timeFormatter.format(date)
}

export function formatDateDivider(timestamp, now = Date.now()) {
  const date = validDate(timestamp)
  const today = validDate(now)
  if (!date || !today) return ''
  if (isSameDay(date, today)) return '今天'

  const yesterday = new Date(today)
  yesterday.setDate(yesterday.getDate() - 1)
  if (isSameDay(date, yesterday)) return '昨天'
  return fullDateFormatter.format(date)
}

export function shouldShowDateDivider(messages, index) {
  const currentDate = validDate(messages[index]?.timestamp)
  if (!currentDate) return index === 0
  const previousDate = validDate(messages[index - 1]?.timestamp)
  return !previousDate || !isSameDay(previousDate, currentDate)
}

export function isoTimestamp(timestamp) {
  const date = validDate(timestamp)
  return date?.toISOString() || undefined
}
