import api from './index'

// 用户注册
export function register(data) {
  return api.post('/auth/register', data)
}

// 用户登录
export function login(data) {
  return api.post('/auth/login', data)
}
