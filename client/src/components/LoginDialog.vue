<script setup lang="ts">
import { ref, onMounted } from 'vue'
import JSEncrypt from 'encryptlong'

const props = defineProps<{
  visible: boolean
  loading: boolean
  message: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'sso-success'): void // 登录彻底成功后通知父组件
}>()

const ssoLoading = ref(false)
const ssoMessage = ref('')

// 第三方单点登录系统接口配置
const globalVariables = {
  gzzApiUrl: 'http://10.228.131.51/qhgzz/authority/isLogin',
  loginUrl: 'http://10.228.131.27:8080/futuresAuthority/#/FirstPage'
}

// 第三方认证要求的 RSA 公钥
const publicKey = 'MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDAK2IaLMengXdADXKwf469axbR+irOW3CwsfuyCwn8SO/iVpvFL498XAvcGLQB1qQtlmEN2Llv2zzq5qa8gNf7HEDPJJgzVaFV+RMdDn9PDBiV2jrY9UEuj5vh87I5u5hWcKLa9lwnO/H1gnaimIQdqVOG432B6i9qIsVx26N4sQIDAQAB'

/**
 * 前往第三方平台登录
 */
const redirectToSSO = () => {
  const currentUrl = encodeURIComponent(window.location.href)
  window.location.href = `${globalVariables.loginUrl}?redirect_uri=${currentUrl}`
}

/**
 * 核心自动化逻辑：检测、清洗、RSA加密、第三方校验、本地后端同步凭证、全自动登录
 */
const handleSsoLogin = async () => {
  if (ssoLoading.value) return // 阻止并发重复请求
  
  ssoLoading.value = true
  ssoMessage.value = '正在验证第三方身份凭证...'

  try {
    // 1. 获取 URL 中的参数
    const urlParams = new URLSearchParams(window.location.search)
    let tokenTmp = urlParams.get('token')
    
    // 兼容 hash 模式下的参数提取
    if (!tokenTmp && window.location.hash.includes('token=')) {
        const hashParams = new URLSearchParams(window.location.hash.split('?')[1])
        tokenTmp = hashParams.get('token')
    }

    if (!tokenTmp) {
      ssoMessage.value = '未检测到有效的 Token 参数'
      ssoLoading.value = false
      return
    }

    // 🔥 修复 1：把被浏览器错误解析为空格的字符，全部强刷回 '+'
    tokenTmp = tokenTmp.replace(/ /g, '+')

    // 🔥 修复 2：获取 urlId，并强制转为 Number 数字类型（Integer），默认 1573 兜底
    const urlidRaw = urlParams.get('urlId') || urlParams.get('urlid') || '1573'
    const urlidNum = Number(urlidRaw)

    // 🔥 修复 3：使用 jsencrypt 进行安全的 RSA 加密
    const encryptor = new JSEncrypt()
    
    // 补全标准的 PEM 头尾格式（如果缺少的话）
    const formattedPublicKey = publicKey.includes('BEGIN PUBLIC KEY') 
      ? publicKey 
      : `-----BEGIN PUBLIC KEY-----\n${publicKey}\n-----END PUBLIC KEY-----`
      
    encryptor.setPublicKey(formattedPublicKey)
    
    // 尝试加密
    const encryptedToken = encryptor.encryptLong(tokenTmp)

    if (!encryptedToken) {
      ssoMessage.value = '凭证加密处理失败，请重试'
      ssoLoading.value = false
      return
    }

    // 排查日志打印
    if (!encryptedToken) {
      console.error('[SSO Debug] 加密失败!')
      console.error('[SSO Debug] 尝试加密的 Token 原文:', tokenTmp)
      console.error('[SSO Debug] Token 长度:', tokenTmp.length, '(注: 1024位RSA公钥极限长度通常为 117 字节)')
      console.error('[SSO Debug] 使用的公钥:', formattedPublicKey)
      
      if (tokenTmp.length > 117) {
         ssoMessage.value = `凭证过长(${tokenTmp.length}字节)，超出RSA加密极限，请联系第三方确认加密方案`
      } else {
         ssoMessage.value = '公钥格式或加密过程异常，请按F12查看控制台'
      }
      ssoLoading.value = false
      return
    }

    // 2. 发起 POST 请求至第三方认证服务器校验加密后的 Token
    const ssoRes = await fetch(globalVariables.gzzApiUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ 
        token: encryptedToken, 
        urlId: urlidNum 
      })
    })
    
    const ssoData = await ssoRes.json()
    console.log('[SSO Debug] 第三方校验接口返回数据:', ssoData)

    if (ssoData.status !== 0 && ssoData.status !== '0') {
      ssoMessage.value = '第三方校验失败：' + (ssoData.msg || '未知错误')
      return
    }

    // 3. 高健壮性解析 username：精准切分出 "username=xxxx" 中的用户账号
    let username = ''
    if (typeof ssoData.result === 'string') {
       if (ssoData.result.includes('username=')) {
           username = ssoData.result.split('username=')[1].split('&')[0]
       } else if (ssoData.result.includes('=')) {
           username = ssoData.result.split('=')[1]
       } else {
           username = ssoData.result
       }
    } else if (ssoData.result && ssoData.result.username) {
       username = ssoData.result.username
    }

    username = username.trim()
    console.log('[SSO Debug] 成功提取目标用户名:', username)

    if (!username) {
      ssoMessage.value = '从第三方返回报文中解析用户名失败'
      return
    }

    ssoMessage.value = `安全校验通过，正在同步系统权限...`

    // 4. 调用你的 Go 后端免密登录接口，让后端写入 Session 并生成本地凭证Cookie
    const loginRes = await fetch('/api/sso-login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include', // 关键：允许本地请求跨域写入管控库需要的 Session Cookie
      body: JSON.stringify({ 
        username: username, 
        token: tokenTmp 
      })
    })
    
    const loginData = await loginRes.json()
    if (!loginRes.ok || !loginData.ok) {
      ssoMessage.value = loginData.message || '本地系统免密登录授权失败'
      return
    }

    // 5. 授权成功，将本地系统的核心状态存入 localStorage 供后续鉴权页面读取
    const localToken = loginData.result?.token || loginData.token
    if (localToken) {
      localStorage.setItem('token', localToken)
    }
    localStorage.setItem('username', username)
    localStorage.setItem('urlid', String(urlidNum))
    localStorage.setItem('isAuthenticated', 'true')
    localStorage.setItem('authTimestamp', Date.now().toString())

    ssoMessage.value = '登录成功！正在为您跳转主页...'
    
    // 6. 触发成功回调，通知父组件（如 App.vue）关闭弹窗并刷新主页状态
    emit('sso-success')

  } catch (err: any) {
    console.error(err)
    ssoMessage.value = '网络通信或脚本执行异常: ' + (err.message || '')
  } finally {
    ssoLoading.value = false
  }
}

// 🚀 挂载核心：侦听到 URL 存在 token 字段时，不需要人点，100% 全自动静默秒登
onMounted(() => {
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