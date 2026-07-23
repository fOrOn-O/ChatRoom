import api from './index'

// 获取群组列表
export function getGroups() {
  return api.get('/groups')
}

// 创建群组
export function createGroup(data) {
  return api.post('/groups', data)
}

// 获取群组详情
export function getGroup(groupId) {
  return api.get(`/groups/${groupId}`)
}

// 获取群成员
export function getGroupMembers(groupId) {
  return api.get(`/groups/${groupId}/members`)
}

// 邀请成员
export function inviteMembers(groupId, data) {
  return api.post(`/groups/${groupId}/members`, data)
}

// 移除群成员
export function removeMember(groupId, userId) {
  return api.delete(`/groups/${groupId}/members/${userId}`)
}

// 退出群组
export function leaveGroup(groupId) {
  return api.post(`/groups/${groupId}/leave`)
}
