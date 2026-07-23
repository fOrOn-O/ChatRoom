import api from './index'

// 获取历史消息
export function getHistory(params) {
  return api.get('/messages', { params })
}

// 标记已读
export function markRead(data) {
  return api.post('/messages/read', data)
}

// 撤回消息
export function revokeMessage(msgId) {
  return api.post(`/messages/${msgId}/revoke`)
}

// 上传文件
export function uploadFile(file) {
  const formData = new FormData()
  formData.append('file', file)
  return api.post('/files/upload', formData, {
    headers: {
      'Content-Type': 'multipart/form-data'
    }
  })
}

export function downloadFile(fileId) {
  return api.get('/files/' + fileId + '/download', { responseType: 'blob' })
}

// 后端把 storage 映射到 /static，上传接口返回的旧路径需要在客户端兼容转换。
export function resolveStoredFileUrl(url) {
  if (!url) return ''
  if (/^https?:\/\//i.test(url)) return url
  if (url.startsWith('/storage/')) return `/static/${url.slice('/storage/'.length)}`
  return url
}
