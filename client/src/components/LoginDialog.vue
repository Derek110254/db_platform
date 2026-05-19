<script setup lang="ts">
/**
 * LoginDialog.vue
 * ------------------------------------------------------------------
 * 该组件只负责登录弹窗 UI：
 * 1. 输入用户名和密码
 * 2. 显示登录消息
 * 3. 提交或关闭弹窗
 *
 * 业务逻辑不在这里处理，统一交由父组件 App.vue 管理。
 */

const props = defineProps<{
  visible: boolean
  loading: boolean
  message: string
  username: string
  password: string
}>()

const emit = defineEmits<{
  (e: 'update:username', value: string): void
  (e: 'update:password', value: string): void
  (e: 'submit'): void
  (e: 'close'): void
}>()

/**
 * 用户名输入变化
 */
const handleUsernameInput = (event: Event) => {
  const value = (event.target as HTMLInputElement).value
  emit('update:username', value)
}

/**
 * 密码输入变化
 */
const handlePasswordInput = (event: Event) => {
  const value = (event.target as HTMLInputElement).value
  emit('update:password', value)
}
</script>

<template>
  <div v-if="props.visible" class="login-mask" @click="emit('close')">
    <div class="login-dialog" @click.stop>
      <h2>用户登录</h2>

      <div class="login-form-item">
        <label>用户名</label>
        <input
          class="login-input"
          :value="props.username"
          placeholder="请输入用户名"
          @input="handleUsernameInput"
        />
      </div>

      <div class="login-form-item">
        <label>密码</label>
        <input
          class="login-input"
          type="password"
          :value="props.password"
          placeholder="请输入密码"
          @input="handlePasswordInput"
          @keyup.enter="emit('submit')"
        />
      </div>

      <div v-if="props.message" class="login-message">
        {{ props.message }}
      </div>

      <div class="login-btn-row">
        <button class="action-btn primary-btn" :disabled="props.loading" @click="emit('submit')" type="button">
          {{ props.loading ? '登录中...' : '登录' }}
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
}

.login-form-item {
  margin-bottom: 14px;
}

.login-form-item label {
  display: block;
  margin-bottom: 8px;
  color: #333;
}

.login-input {
  width: 100%;
  padding: 10px 12px;
  font-size: 16px;
  border: 1px solid #ccc;
  border-radius: 6px;
  background: #fff;
}

.login-message {
  margin: 10px 0 14px;
  color: #f56c6c;
  font-size: 14px;
}

.login-btn-row {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
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