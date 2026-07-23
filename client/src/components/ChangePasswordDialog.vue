<script setup lang="ts">
/**
 * ChangePasswordDialog.vue
 * ------------------------------------------------------------------
 * 修改密码弹窗。
 *
 * 两种触发场景：
 * 1. 首次登录 / 被管理员重置密码后强制修改（isForce=true，不可取消）。
 * 2. 用户在菜单中主动修改密码（isForce=false，可取消）。
 *
 * 通过 props.visible 控制显隐；前端校验旧密码、新密码、确认密码后，
 * emit('submit', oldPassword, newPassword) 由父组件请求后端完成修改。
 */

import { ref, watch, onMounted, onBeforeUnmount } from 'vue'

const props = withDefaults(defineProps<{
  visible: boolean
  isForce?: boolean
}>(), {
  isForce: true
})

const emit = defineEmits<{
  (e: 'submit', oldPassword: string, newPassword: string): void
  (e: 'cancel'): void
}>()

const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const message = ref('')

watch(
  () => props.visible,
  (newVal) => {
    if (newVal) {
      oldPassword.value = ''
      newPassword.value = ''
      confirmPassword.value = ''
      message.value = ''
    }
  }
)

const handleSubmit = () => {
  if (!oldPassword.value || !newPassword.value || !confirmPassword.value) {
    message.value = '请填写所有字段'
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    message.value = '两次输入的新密码不一致'
    return
  }
  if (oldPassword.value === newPassword.value) {
    message.value = '新密码不能与原密码一致'
    return
  }
  emit('submit', oldPassword.value, newPassword.value)
}

const handleCancel = () => {
  if (!props.isForce) {
    emit('cancel')
  }
}

const handleEsc = (e: KeyboardEvent) => {
  if (e.key === 'Escape' && props.visible && !props.isForce) {
    handleCancel()
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleEsc)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleEsc)
})

const setMessage = (msg: string) => {
  message.value = msg
}

defineExpose({
  setMessage
})
</script>

<template>
  <div v-if="props.visible" class="login-mask" @click="handleCancel">
    <div class="login-dialog" @click.stop>
      <button v-if="!props.isForce" class="close-icon-btn" @click="handleCancel" title="关闭">&times;</button>
      <h2 v-if="props.isForce">首次登录需要修改密码</h2>
      <h2 v-else>修改密码</h2>
      
      <p v-if="props.isForce" style="margin-bottom: 20px; color: #666;">为了您的账号安全，请在继续操作前修改默认密码。</p>

      <div class="login-form-item">
        <label>原密码</label>
        <input
          class="login-input"
          type="password"
          v-model="oldPassword"
          placeholder="请输入原密码"
        />
      </div>

      <div class="login-form-item">
        <label>新密码</label>
        <input
          class="login-input"
          type="password"
          v-model="newPassword"
          placeholder="请输入新密码"
        />
      </div>

      <div class="login-form-item">
        <label>确认新密码</label>
        <input
          class="login-input"
          type="password"
          v-model="confirmPassword"
          placeholder="请再次输入新密码"
          @keyup.enter="handleSubmit"
        />
      </div>

      <div v-if="message" class="login-message">
        {{ message }}
      </div>

      <div class="login-btn-row">
        <button class="action-btn primary-btn" @click="handleSubmit" type="button">
          确认修改
        </button>
        <button v-if="!props.isForce" class="action-btn secondary-btn" @click="handleCancel" type="button">
          取消
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 组件特有样式（公共样式见 global.css） */

/* 弹窗 */
.login-mask { position: fixed; inset: 0; z-index: 3000; background: rgba(0,0,0,0.45); display: flex; align-items: center; justify-content: center; backdrop-filter: blur(2px); }
.login-dialog { width: 420px; max-width: calc(100vw - 32px); background: #fff; border-radius: 12px; padding: 30px; box-shadow: 0 16px 48px rgba(0,0,0,0.25); position: relative; }
.close-icon-btn { position: absolute; top: 16px; right: 16px; background: transparent; border: none; font-size: 24px; line-height: 1; color: #909399; cursor: pointer; transition: color 0.2s; padding: 0; width: 24px; height: 24px; display: flex; align-items: center; justify-content: center; }
.close-icon-btn:hover { color: #f56c6c; }
.login-dialog h2 { margin-top: 0; color: #2c3e50; margin-bottom: 10px; }

/* 表单 */
.login-form-item { margin-bottom: 16px; }
.login-form-item label { display: block; margin-bottom: 8px; color: #333; font-weight: 500; }
.login-input { transition: border-color 0.2s; }
.login-message { margin: 10px 0 14px; color: #f56c6c; font-size: 14px; }

/* 按钮 */
.login-btn-row { display: flex; gap: 12px; justify-content: center; margin-top: 20px; }
.action-btn { font-weight: 600; width: 100%; }
.primary-btn { transition: background 0.2s; }
.primary-btn:hover { background: #66b1ff; }
.secondary-btn:hover { background: #a6a9ad; }
</style>