import api from './index'

// 获取用户信息
export function getProfile() {
  return api.get('/user/profile')
}

// 更新用户信息
export function updateProfile(data) {
  return api.put('/user/profile', data)
}

// 搜索用户
export function searchUsers(keyword) {
  return api.get('/users/search', { params: { keyword } })
}
