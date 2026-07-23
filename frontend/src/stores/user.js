import { defineStore } from 'pinia'
import { ref } from 'vue'
import { login as loginApi, register as registerApi } from '../api/auth'
import { getProfile } from '../api/user'

export const useUserStore = defineStore('user', () => {
  const token = ref(localStorage.getItem('token') || '')
  const userInfo = ref(null)

  // 登录
  async function login(data) {
    const res = await loginApi(data)
    token.value = res.data.token
    userInfo.value = {
      id: res.data.id,
      username: res.data.username,
      nickname: res.data.nickname,
      avatar: res.data.avatar
    }
    localStorage.setItem('token', res.data.token)
    return res
  }

  // 注册
  async function register(data) {
    const res = await registerApi(data)
    token.value = res.data.token
    userInfo.value = {
      id: res.data.id,
      username: res.data.username,
      nickname: res.data.nickname
    }
    localStorage.setItem('token', res.data.token)
    return res
  }

  // 获取用户信息
  async function fetchProfile() {
    const res = await getProfile()
    userInfo.value = res.data
    return res
  }

  // 登出
  function logout() {
    token.value = ''
    userInfo.value = null
    localStorage.removeItem('token')
  }

  return {
    token,
    userInfo,
    login,
    register,
    fetchProfile,
    logout
  }
})
