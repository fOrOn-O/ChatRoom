<template>
  <main class="auth-page">
    <section class="auth-intro" aria-label="ChatRoom 介绍">
      <div class="brand-lockup"><span class="brand-mark" aria-hidden="true"><i></i><i></i><i></i></span><span>ChatRoom</span></div>
      <div class="intro-copy"><p class="eyebrow">START A CONVERSATION</p><h1>从一个名字，开始连接。</h1><p>创建账号后，即可与好友共享消息、文件和群组对话。</p></div>
      <div class="intro-footer"><span></span>注册后立即可用</div>
    </section>

    <section class="auth-panel">
      <div class="auth-form-wrap">
        <p class="eyebrow">CREATE ACCOUNT</p><h2>创建你的账号</h2><p class="form-hint">只需几项信息，即可进入 ChatRoom。</p>
        <el-form ref="formRef" :model="form" :rules="rules" class="auth-form" @submit.prevent="handleRegister">
          <el-form-item prop="username" label="用户名"><el-input v-model.trim="form.username" autocomplete="username" placeholder="3–50 个字符" size="large" /></el-form-item>
          <el-form-item prop="nickname" label="显示名称"><el-input v-model.trim="form.nickname" placeholder="你希望大家怎样称呼你" size="large" /></el-form-item>
          <el-form-item prop="password" label="密码"><el-input v-model="form.password" autocomplete="new-password" type="password" show-password placeholder="至少 6 个字符" size="large" /></el-form-item>
          <el-form-item prop="confirmPassword" label="确认密码"><el-input v-model="form.confirmPassword" autocomplete="new-password" type="password" show-password placeholder="再次输入密码" size="large" @keyup.enter="handleRegister" /></el-form-item>
          <el-button class="submit-button" native-type="submit" :loading="loading" :disabled="loading">创建账号 <el-icon><ArrowRight /></el-icon></el-button>
        </el-form>
        <p class="auth-switch">已有账号？<router-link to="/login">直接登录</router-link></p>
      </div>
    </section>
  </main>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useUserStore } from '../../stores/user'

const router = useRouter()
const userStore = useUserStore()
const formRef = ref()
const loading = ref(false)
const form = reactive({ username: '', nickname: '', password: '', confirmPassword: '' })
const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }, { min: 3, max: 50, message: '用户名长度为 3–50 个字符', trigger: 'blur' }],
  nickname: [{ required: true, message: '请输入显示名称', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }, { min: 6, message: '密码至少 6 个字符', trigger: 'blur' }],
  confirmPassword: [{ required: true, message: '请再次输入密码', trigger: 'blur' }, { validator: (_, value, callback) => value === form.password ? callback() : callback(new Error('两次密码输入不一致')), trigger: 'blur' }]
}

async function handleRegister() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  loading.value = true
  try {
    await userStore.register({ username: form.username, nickname: form.nickname, password: form.password })
    ElMessage.success('账号创建成功')
    router.replace('/chat')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.auth-page { min-height: 100svh; display: grid; grid-template-columns: minmax(0, 1.15fr) minmax(420px, .85fr); background: #f7f8f5; }.auth-intro { position: relative; display: flex; flex-direction: column; padding: clamp(32px, 5vw, 72px); overflow: hidden; color: #f6f7f2; background: #18342b; }.auth-intro::before { content:''; position:absolute; width:min(58vw,760px); aspect-ratio:1; right:-16%; bottom:-30%; border:1px solid rgba(217,235,213,.32); border-radius:50%; box-shadow:0 0 0 66px rgba(217,235,213,.05),0 0 0 132px rgba(217,235,213,.035),0 0 0 198px rgba(217,235,213,.025); }.brand-lockup{position:relative;z-index:1;display:inline-flex;align-items:center;gap:10px;font-size:18px;font-weight:800;letter-spacing:-.7px}.brand-mark{display:inline-flex;align-items:flex-end;gap:3px;width:24px;height:24px;padding:5px;border-radius:7px;background:#d9efc6}.brand-mark i{display:block;width:3px;border-radius:999px;background:#18342b}.brand-mark i:nth-child(1){height:6px}.brand-mark i:nth-child(2){height:11px}.brand-mark i:nth-child(3){height:15px}.intro-copy{position:relative;z-index:1;max-width:600px;margin:auto 0}.eyebrow{margin:0;color:#86b697;font-size:11px;font-weight:800;letter-spacing:.16em}.intro-copy h1{max-width:600px;margin:18px 0;font-size:clamp(44px,5vw,76px);line-height:1.08;letter-spacing:-.075em}.intro-copy>p:last-child{max-width:420px;margin:0;color:#c4d6c7;font-size:16px;line-height:1.75}.intro-footer{position:relative;z-index:1;display:flex;align-items:center;gap:10px;color:#9eb9a3;font-size:12px}.intro-footer span{width:7px;height:7px;border-radius:50%;background:#b9e978;box-shadow:0 0 0 5px rgba(185,233,120,.14)}.auth-panel{display:grid;place-items:center;padding:36px}.auth-form-wrap{width:min(100%,370px)}.auth-form-wrap h2{margin:12px 0 7px;color:#16251e;font-size:30px;letter-spacing:-.05em}.form-hint{margin:0;color:#738078;font-size:14px}.auth-form{margin-top:30px}.auth-form :deep(.el-form-item){margin-bottom:15px}.auth-form :deep(.el-form-item__label){padding-bottom:7px;color:#425048;font-size:13px;font-weight:700}.auth-form :deep(.el-input__wrapper){padding:3px 12px;border-radius:10px;background:#fff;box-shadow:0 0 0 1px #dce3db inset}.auth-form :deep(.el-input__wrapper.is-focus){box-shadow:0 0 0 2px #5f8d66 inset}.submit-button{width:100%;height:47px;margin-top:5px;border:0;border-radius:10px;color:#f9fbf6;background:#28583c;font-weight:800}.submit-button:hover{background:#1d472f}.submit-button .el-icon{margin-left:9px}.auth-switch{margin:22px 0 0;color:#78837b;font-size:13px;text-align:center}.auth-switch a{margin-left:4px;color:#28583c;font-weight:800;text-decoration:none}@media(max-width:760px){.auth-page{display:block}.auth-intro{min-height:36svh;padding:28px}.intro-copy{margin:auto 0 18px}.intro-copy h1{margin:12px 0;font-size:39px}.intro-copy>p:last-child{display:none}.auth-panel{min-height:64svh;align-items:start;padding:35px 26px}.auth-form{margin-top:24px}}
</style>
