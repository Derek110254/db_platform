<script setup lang="ts">
/**
 * LoginDialog.vue
 * ------------------------------------------------------------------
 * 该组件是「登录弹窗」，负责 SSO 单点登录接入。
 *
 * 主要功能：
 * 1. 从 SSO 回跳 URL 中提取登录 token（工作证认证）。
 * 2. 将原始 token 提交后端 /api/sso-login，由后端完成 RSA 加密、调用 SSO 鉴权、
 *    用户查找与首次登录自动注册，并下发会话。
 * 3. 从 /api/sso-config 读取 SSO 登录入口地址，供「跳转登录」按钮使用。
 *
 * 说明：前端不再做任何 RSA 加密 / SSO API 调用，全部下沉到后端（auth/sso.go）。
 */

import { ref, onMounted } from 'vue'

const props = defineProps<{
  visible: boolean
  loading: boolean
  message: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'sso-success'): void
}>()

const ssoLoading = ref(false)
const ssoMessage = ref('')

// SSO 登录页地址（从后端获取，只有这一个需要暴露给前端）
const ssoLoginUrl = ref('')

const loadSsoConfig = async () => {
  try {
    const res = await fetch('/api/sso-config')
    const data = await res.json()
    if (data.ok) {
      ssoLoginUrl.value = data.loginUrl || ''
    }
  } catch {
    // 获取失败时使用空值，用户点击时会提示
  }
}

/**
 * 前往工作证平台登录
 */
const redirectToSSO = () => {
  if (!ssoLoginUrl.value) {
    ssoMessage.value = 'SSO 配置加载失败，请刷新页面重试'
    return
  }
  const currentUrl = encodeURIComponent(window.location.href)
  window.location.href = `${ssoLoginUrl.value}?redirect_uri=${currentUrl}`
}

/**
 * SSO 自动登录：检测 URL 中的 token，发送给后端验证
 */
const handleSsoLogin = async () => {
  if (ssoLoading.value) return

  ssoLoading.value = true
  ssoMessage.value = '正在验证身份凭证...'

  try {
    // 1. 获取 URL 中的 token 参数
    const urlParams = new URLSearchParams(window.location.search)
    let tokenTmp = urlParams.get('token')

    // 兼容 hash 模式
    if (!tokenTmp && window.location.hash.includes('token=')) {
      const hashParams = new URLSearchParams(window.location.hash.split('?')[1])
      tokenTmp = hashParams.get('token')
    }

    if (!tokenTmp) {
      ssoMessage.value = '未检测到有效的 Token 参数'
      ssoLoading.value = false
      return
    }

    // 修复：浏览器可能将 + 转为空格，还原回来
    tokenTmp = tokenTmp.replace(/ /g, '+')

    // 获取 urlId（可选，从 URL 参数取）
    const urlId = urlParams.get('urlId') || urlParams.get('urlid') || ''

    ssoMessage.value = '安全校验中，正在同步系统权限...'

    // 2. 将原始 token 发送给后端，后端完成 RSA 加密 + SSO API 调用 + 用户名解析
    const loginRes = await fetch('/api/sso-login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({
        token: tokenTmp,
        urlId: urlId,
      })
    })

    const loginData = await loginRes.json()
    if (!loginRes.ok || !loginData.ok) {
      ssoMessage.value = loginData.message || '登录授权失败'
      return
    }

    // 3. 授权成功，存储状态
    const localToken = loginData.result?.token || loginData.token
    if (localToken) {
      localStorage.setItem('token', localToken)
    }
    if (loginData.username) {
      localStorage.setItem('username', loginData.username)
    }
    localStorage.setItem('isAuthenticated', 'true')
    localStorage.setItem('authTimestamp', Date.now().toString())

    ssoMessage.value = '登录成功！正在为您跳转主页...'
    emit('sso-success')

  } catch (err: any) {
    console.error(err)
    ssoMessage.value = '网络通信或脚本执行异常: ' + (err.message || '')
  } finally {
    ssoLoading.value = false
  }
}

// 挂载时：加载配置，检测 token 自动登录
onMounted(async () => {
  await loadSsoConfig()
  if (window.location.search.includes('token=') || window.location.hash.includes('token=')) {
    handleSsoLogin()
  }
})
</script>

<template>
  <div v-if="props.visible" class="login-mask" @click="emit('close')">
    <div class="login-dialog" @click.stop>
      <h2>统一身份认证</h2>

      <p class="sso-tips">本系统已接入统一登录，无需输入账号密码。</p>

      <div v-if="ssoMessage || props.message" class="login-message">
        {{ ssoMessage || props.message }}
      </div>

      <div class="login-btn-row">
        <button class="action-btn primary-btn" :disabled="ssoLoading || props.loading" @click="redirectToSSO" type="button">
          {{ ssoLoading ? '正在自动登录...' : '前往统一认证页面' }}
        </button>
        <button class="action-btn secondary-btn" @click="emit('close')" type="button">
          取消
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-mask {
  position: fixed;
  inset: 0;
  z-index: 2000;
  background: rgba(0, 0, 0, 0.35);
  display: flex;
  align-items: center;
  justify-content: center;
}

.login-dialog {
  width: 420px;
  max-width: calc(100vw - 32px);
  background: #fff;
  border-radius: 12px;
  padding: 24px;
  box-shadow: 0 12px 36px rgba(0, 0, 0, 0.18);
}

.login-dialog h2 {
  margin-top: 0;
  color: #2c3e50;
  text-align: center;
}

.sso-tips {
  color: #666;
  font-size: 15px;
  text-align: center;
  margin: 20px 0 30px;
}

.login-message {
  margin: 10px 0 14px;
  color: #409eff;
  font-size: 14px;
  text-align: center;
  word-break: break-all;
}

.login-btn-row {
  display: flex;
  gap: 12px;
  justify-content: center;
}

.action-btn {
  padding: 10px 20px;
  font-size: 16px;
  cursor: pointer;
  border: none;
  border-radius: 6px;
  color: #fff;
}

.primary-btn {
  background: #409eff;
}

.secondary-btn {
  background: #909399;
}
</style>
