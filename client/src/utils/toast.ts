/**
 * toast.ts — 全局 Toast 通知 + Confirm 确认弹窗
 *
 * 用法：
 *   import { showToast, showConfirm } from '../utils/toast'
 *
 *   showToast('创建成功', 'success')
 *   showToast('删除失败', 'error')
 *   const ok = await showConfirm('确定要删除吗？')
 */

import { ref } from 'vue'

// ========== Toast ==========
interface ToastItem {
  id: number
  message: string
  type: 'success' | 'error' | 'warning' | 'info'
}

const toasts = ref<ToastItem[]>([])
let toastId = 0

export function showToast(message: string, type: ToastItem['type'] = 'info') {
  const id = ++toastId
  toasts.value.push({ id, message, type })
  setTimeout(() => {
    toasts.value = toasts.value.filter(t => t.id !== id)
  }, 3000)
}

export function useToasts() {
  return toasts
}

// ========== Confirm ==========
interface ConfirmState {
  visible: boolean
  title: string
  message: string
  resolve: ((value: boolean) => void) | null
}

const confirmState = ref<ConfirmState>({
  visible: false,
  title: '确认操作',
  message: '',
  resolve: null,
})

export function showConfirm(message: string, title = '确认操作'): Promise<boolean> {
  return new Promise(resolve => {
    confirmState.value = { visible: true, title, message, resolve }
  })
}

export function resolveConfirm(result: boolean) {
  if (confirmState.value.resolve) {
    confirmState.value.resolve(result)
  }
  confirmState.value = { visible: false, title: '确认操作', message: '', resolve: null }
}

export function useConfirm() {
  return confirmState
}
