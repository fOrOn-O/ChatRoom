import api from './index'

// 获取好友列表
export function getFriends() {
  return api.get('/friends')
}

// 发送好友请求
export function sendFriendRequest(data) {
  return api.post('/friends/request', data)
}

// 处理好友请求（接受/拒绝）
export function handleFriendRequest(data) {
  return api.post('/friends/handle', data)
}

// 删除好友
export function deleteFriend(friendId) {
  return api.delete(`/friends/${friendId}`)
}
