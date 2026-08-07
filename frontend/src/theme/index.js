export const CHAT_THEME_STORAGE_KEY = 'chatroom:workspace-theme'

export const CHAT_THEMES = Object.freeze([
  Object.freeze({ id: 'minimal', label: '极简', description: '清晰、克制的原始界面' }),
  Object.freeze({ id: 'glass', label: '玻璃拟态', description: '通透渐变与柔和景深' }),
  Object.freeze({ id: 'neumorphic', label: '新拟态', description: '柔和浮雕与触感层次' })
])

const themeIds = new Set(CHAT_THEMES.map(({ id }) => id))

export function normalizeChatTheme(value) {
  return themeIds.has(value) ? value : CHAT_THEMES[0].id
}

function resolveStorage(storage) {
  if (storage !== undefined) return storage
  try {
    return globalThis.localStorage
  } catch {
    return null
  }
}

export function readStoredChatTheme(storage) {
  try {
    return normalizeChatTheme(resolveStorage(storage)?.getItem(CHAT_THEME_STORAGE_KEY))
  } catch {
    return CHAT_THEMES[0].id
  }
}

export function writeStoredChatTheme(value, storage) {
  const theme = normalizeChatTheme(value)
  try {
    resolveStorage(storage)?.setItem(CHAT_THEME_STORAGE_KEY, theme)
  } catch {
    // Storage can be unavailable in private or restricted browser contexts.
  }
  return theme
}
