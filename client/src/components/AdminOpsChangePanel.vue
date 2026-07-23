<script setup lang="ts">
/**
 * AdminOpsChangePanel.vue
 * ------------------------------------------------------------------
 * 该组件是「运维变更记录」页面。
 *
 * 布局模式：list 列表 ↔ view 查看 ↔ form 新增/编辑。
 *
 * 主要功能：
 * 1. 分页展示运维变更记录，按变更类型 / 变更级别过滤（下拉自动响应）。
 * 2. 申请人新增 / 编辑 / 删除自己的记录（含变更标题、内容、影响范围、变更 IP 列表、回滚计划）。
 * 3. 三态流转：待复核 → 待变更 → 变更结果（成功/失败）→ 回滚。
 * 4. 管理员指派复核人、登记变更结果、登记回滚状态；成功自动标记「无需回滚」。
 * 5. 已完结记录禁止再编辑/删除。
 *
 * 关键接口：
 * - GET    /api/ops-change-records                       用户 CRUD
 * - GET    /api/admin/ops-change-records                 管理员查询全部
 * - PUT    /api/admin/ops-change-records/reviewer        指派复核人
 * - PUT    /api/admin/ops-change-records/result          登记变更结果
 * - PUT    /api/admin/ops-change-records/rollback        登记回滚状态
 */

import { computed, onMounted, ref } from 'vue'
import { showToast, showConfirm } from '../utils/toast'

interface OpsChangeItem {
  id: number
  changeTitle: string
  changeType: string
  changeLevel: string
  changeContent: string
  impactScope: string
  changeIpList: string
  changeTime: string
  operator: string
  reviewer: string
  changeResult: string
  rollbackPlan: string
  rollbackStatus: string
  remark: string
  createTime: string
  updateTime: string
}

const loading = ref(false)
const message = ref('')

const viewMode = ref<'list' | 'view' | 'form'>('list')
const isEditMode = ref(false)

const records = ref<OpsChangeItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const filterChangeType = ref('')
const filterChangeLevel = ref('')
const searchKeyword = ref('')

const formatTime = (v: string): string => {
  if (!v) return ''
  return v.replace('T', ' ').slice(0, 16)
}

const truncate = (text: string, maxLen: number): string => {
  if (!text) return ''
  return text.length > maxLen ? text.slice(0, maxLen) + '...' : text
}

const handleFilterChange = () => {
  page.value = 1
  loadRecords()
}

const form = ref({
  id: 0,
  changeTitle: '',
  changeType: '',
  changeLevel: '常规',
  changeContent: '',
  impactScope: '',
  changeIpList: '',
  changeTime: '',
  rollbackPlan: '',
  remark: '',
})

const resetForm = () => {
  isEditMode.value = false
  form.value = {
    id: 0,
    changeTitle: '',
    changeType: '',
    changeLevel: '常规',
    changeContent: '',
    impactScope: '',
    changeIpList: '',
    changeTime: '',
    rollbackPlan: '',
    remark: '',
  }
}

const fillForm = (item: OpsChangeItem) => {
  form.value = {
    id: item.id,
    changeTitle: item.changeTitle,
    changeType: item.changeType,
    changeLevel: item.changeLevel || '常规',
    changeContent: item.changeContent,
    impactScope: item.impactScope,
    changeIpList: item.changeIpList,
    changeTime: item.changeTime ? item.changeTime.replace('T', ' ').slice(0, 16) : '',
    rollbackPlan: item.rollbackPlan,
    remark: item.remark,
  }
}

const showCreateForm = () => {
  resetForm()
  message.value = ''
  viewMode.value = 'form'
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

const startView = (item: OpsChangeItem) => {
  fillForm(item)
  message.value = `查看记录：ID=${item.id}`
  viewMode.value = 'view'
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

const startEdit = (item: OpsChangeItem) => {
  fillForm(item)
  isEditMode.value = true
  message.value = `正在编辑记录：ID=${item.id}`
  viewMode.value = 'form'
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

const backToList = () => {
  viewMode.value = 'list'
  message.value = ''
  loadRecords()
}

const loadRecords = async () => {
  loading.value = true
  message.value = ''

  try {
    const params = new URLSearchParams({
      page: String(page.value),
      pageSize: String(pageSize.value),
      changeType: filterChangeType.value,
      changeLevel: filterChangeLevel.value,
    })
    const res = await fetch(`/api/admin/ops-change-records?${params.toString()}`, {
      credentials: 'include',
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || '加载列表失败'
      records.value = []
      return
    }

    records.value = data.records || []
    total.value = data.total || 0
  } catch (err) {
    console.error(err)
    message.value = '加载列表失败，请检查后端是否启动'
    records.value = []
  } finally {
    loading.value = false
  }
}

/**
 * 确认弹窗：根据当前状态自适应显示
 * 待复核 → 填复核人，复核通过后状态变为待变更
 * 待变更 → 选择变更结果（成功/失败/部分成功）
 * 失败/部分成功 + 回滚待确认 → 选择回滚状态
 */
const confirmDialogVisible = ref(false)
const confirmTargetId = ref(0)
const confirmTargetItem = ref<OpsChangeItem | null>(null)
const confirmReviewer = ref('')
const confirmResult = ref('成功')
const confirmRollback = ref('无需回滚')

// 根据当前记录所处状态，决定确认弹窗的步骤：
// 待复核 → 指派复核人；待变更 → 登记变更结果；失败/部分成功且回滚待确认 → 登记回滚状态
const confirmStep = computed(() => {
  const item = confirmTargetItem.value
  if (!item) return ''
  if (item.changeResult === '待复核') return 'review'
  if (item.changeResult === '待变更') return 'result'
  if ((item.changeResult === '失败' || item.changeResult === '部分成功') && item.rollbackStatus === '待确认') return 'rollback'
  return ''
})

// 判断当前编辑中的记录是否已完结（已登记结果且回滚状态非待确认），完结后禁止编辑/删除
const isCurrentCompleted = computed(() => {
  const item = records.value.find(r => r.id === form.value.id)
  if (!item) return false
  if (item.changeResult === '待复核' || item.changeResult === '待变更') return false
  if ((item.changeResult === '失败' || item.changeResult === '部分成功') && item.rollbackStatus === '待确认') return false
  return true
})

const openConfirmDialog = (item: OpsChangeItem) => {
  confirmTargetItem.value = item
  confirmTargetId.value = item.id
  confirmReviewer.value = item.reviewer || ''
  confirmResult.value = '成功'
  confirmRollback.value = '无需回滚'
  confirmDialogVisible.value = true
}

const closeConfirmDialog = () => {
  confirmDialogVisible.value = false
  confirmTargetItem.value = null
}

const doConfirm = async () => {
  const id = confirmTargetId.value
  if (!id) return

  loading.value = true
  message.value = ''

  try {
    const step = confirmStep.value

    if (step === 'review') {
      const res = await fetch('/api/admin/ops-change-records/reviewer', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ id }),
      })
      const data = await res.json()
      if (!res.ok || !data.ok) {
        message.value = data.message || '复核失败'
        return
      }
    } else if (step === 'result') {
      const res = await fetch('/api/admin/ops-change-records/result', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ id, changeResult: confirmResult.value }),
      })
      const data = await res.json()
      if (!res.ok || !data.ok) {
        message.value = data.message || '确认变更结果失败'
        return
      }
    } else if (step === 'rollback') {
      const res = await fetch('/api/admin/ops-change-records/rollback', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ id, rollbackStatus: confirmRollback.value }),
      })
      const data = await res.json()
      if (!res.ok || !data.ok) {
        message.value = data.message || '确认回滚状态失败'
        return
      }
    }

    showToast('操作成功', 'success')
    confirmDialogVisible.value = false
    await loadRecords()
  } catch (err) {
    console.error(err)
    message.value = '操作失败，请检查后端是否启动'
  } finally {
    loading.value = false
  }
}

const createRecord = async () => {
  if (!form.value.changeTitle.trim()) {
    message.value = '❌ 请填写变更标题'
    return
  }
  if (!form.value.changeType) {
    message.value = '❌ 请选择变更类型'
    return
  }
  if (!form.value.changeContent.trim()) {
    message.value = '❌ 请填写变更内容'
    return
  }
  if (!form.value.changeTime) {
    message.value = '❌ 请选择变更时间'
    return
  }

  loading.value = true
  message.value = ''

  try {
    const res = await fetch('/api/ops-change-records', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({
        ...form.value,
        changeTime: form.value.changeTime ? form.value.changeTime.replace('T', ' ') + ':00' : '',
      }),
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || '创建记录失败'
      return
    }

    message.value = ''
    showToast(data.message || '创建记录成功', 'success')
    backToList()
  } catch (err) {
    console.error(err)
    message.value = '创建记录失败，请检查后端是否启动'
  } finally {
    loading.value = false
  }
}

const updateRecord = async () => {
  if (!form.value.changeTitle.trim()) {
    message.value = '❌ 请填写变更标题'
    return
  }
  if (!form.value.changeType) {
    message.value = '❌ 请选择变更类型'
    return
  }
  if (!form.value.changeContent.trim()) {
    message.value = '❌ 请填写变更内容'
    return
  }
  if (!form.value.changeTime) {
    message.value = '❌ 请选择变更时间'
    return
  }

  loading.value = true
  message.value = ''

  try {
    const res = await fetch('/api/ops-change-records', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({
        ...form.value,
        changeTime: form.value.changeTime ? form.value.changeTime.replace('T', ' ') + ':00' : '',
      }),
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || '更新记录失败'
      return
    }

    message.value = ''
    showToast(data.message || '更新记录成功', 'success')
    backToList()
  } catch (err) {
    console.error(err)
    message.value = '更新记录失败，请检查后端是否启动'
  } finally {
    loading.value = false
  }
}

const deleteRecord = async (item: OpsChangeItem) => {
  const ok = await showConfirm(`确定要删除记录【${item.changeTitle}】吗？`)
  if (!ok) return

  loading.value = true
  message.value = ''

  try {
    const res = await fetch('/api/ops-change-records', {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ id: item.id }),
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || '删除记录失败'
      return
    }

    showToast(data.message || '删除记录成功', 'success')
    await loadRecords()
  } catch (err) {
    console.error(err)
    message.value = '删除记录失败，请检查后端是否启动'
  } finally {
    loading.value = false
  }
}

const handlePageChange = (delta: number) => {
  const newPage = page.value + delta
  if (newPage < 1) return
  page.value = newPage
  loadRecords()
}

onMounted(() => {
  loadRecords()
})
</script>

<template>
  <div class="admin-page">
    <!-- ============ 列表视图 ============ -->
    <div v-if="viewMode === 'list'" class="card table-card">
      <div class="table-header">
        <h2>运维变更记录列表 (总数: {{ total }})</h2>
        <div class="header-actions">
          <select v-model="filterChangeType" @change="handleFilterChange" class="filter-select">
            <option value="">全部类型</option>
            <option value="安装部署">安装部署</option>
            <option value="配置变更">配置变更</option>
            <option value="服务重启">服务重启</option>
            <option value="版本升级">版本升级</option>
            <option value="数据修复">数据修复</option>
            <option value="性能优化">性能优化</option>
            <option value="容量变更">容量变更</option>
            <option value="应急变更">应急变更</option>
            <option value="其他">其他</option>
          </select>
          <select v-model="filterChangeLevel" @change="handleFilterChange" class="filter-select">
            <option value="">全部等级</option>
            <option value="常规">常规</option>
            <option value="重要">重要</option>
            <option value="紧急">紧急</option>
          </select>
          <button class="action-btn primary-btn" :disabled="loading" @click="showCreateForm" type="button">
            + 新增记录
          </button>
        </div>
      </div>

      <p class="result">{{ message }}</p>

      <div class="table-wrap">
        <table class="result-table">
          <colgroup>
            <col style="width: 7%" />
            <col style="width: 5%" />
            <col style="width: 14%" />
            <col style="width: 10%" />
            <col style="width: 10%" />
            <col style="width: 9%" />
            <col style="width: 7%" />
            <col style="width: 7%" />
            <col style="width: 9%" />
            <col style="width: 14%" />
          </colgroup>
          <thead>
            <tr>
              <th>变更类型</th>
              <th>变更等级</th>
              <th>变更标题</th>
              <th>影响范围</th>
              <th>变更IP列表</th>
              <th>变更时间</th>
              <th>操作人</th>
              <th>复核人</th>
              <th>变更结果</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in records" :key="item.id" class="data-row" @dblclick="startView(item)">
              <td>{{ item.changeType }}</td>
              <td><span :class="['level-tag', 'level-' + item.changeLevel]">{{ item.changeLevel }}</span></td>
              <td class="cell-wrap" :title="item.changeTitle">{{ truncate(item.changeTitle, 30) }}</td>
              <td class="cell-wrap" :title="item.impactScope">{{ truncate(item.impactScope, 30) }}</td>
              <td class="cell-wrap" :title="item.changeIpList">{{ truncate(item.changeIpList, 30) }}</td>
              <td>{{ formatTime(item.changeTime) }}</td>
              <td>{{ item.operator }}</td>
              <td>{{ item.reviewer || '-' }}</td>
              <td>
                <span :class="['result-tag', 'result-' + item.changeResult]">
                  {{ item.changeResult }}<template v-if="(item.changeResult === '失败' || item.changeResult === '部分成功') && item.rollbackStatus !== '待确认'">/{{ item.rollbackStatus }}</template>
                </span>
              </td>
              <td>
                <div class="row-btns">
                  <button @click="startView(item)" class="mini-btn view-btn">详情</button>
                  <button v-if="item.changeResult === '待复核'" @click="openConfirmDialog(item)" class="mini-btn confirm-btn">复核</button>
                  <button v-else-if="item.changeResult === '待变更'" @click="openConfirmDialog(item)" class="mini-btn confirm-btn">确认结果</button>
                  <button v-else-if="(item.changeResult === '失败' || item.changeResult === '部分成功') && item.rollbackStatus === '待确认'" @click="openConfirmDialog(item)" class="mini-btn rollback-btn">确认回滚</button>
                  <span v-else class="done-text">已完成</span>
                </div>
              </td>
            </tr>
            <tr v-if="records.length === 0">
              <td colspan="10" class="empty-text">暂无数据</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="pagination">
        <button class="action-btn secondary-btn" :disabled="page <= 1 || loading" @click="handlePageChange(-1)">上一页</button>
        <span class="page-info">第 {{ page }} 页</span>
        <button class="action-btn secondary-btn" :disabled="records.length < pageSize || loading" @click="handlePageChange(1)">下一页</button>
      </div>
    </div>

    <!-- ============ 查看 / 登记 / 编辑视图 ============ -->
    <div v-else class="card form-card">
      <div class="form-header">
        <h2 v-if="viewMode === 'view'">查看运维变更记录</h2>
        <h2 v-else-if="isEditMode">编辑运维变更记录</h2>
        <h2 v-else>登记运维变更记录</h2>
        <button class="action-btn secondary-btn back-btn" @click="backToList" type="button">← 返回列表</button>
      </div>

      <div class="form-grid">
        <div class="form-item">
          <label>变更标题 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <input v-model="form.changeTitle" :disabled="viewMode === 'view'" placeholder="简述变更内容" />
        </div>
        <div class="form-item">
          <label>变更类型 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <select v-model="form.changeType" :disabled="viewMode === 'view'">
            <option value="">请选择变更类型</option>
            <option value="安装部署">安装部署</option>
            <option value="配置变更">配置变更</option>
            <option value="服务重启">服务重启</option>
            <option value="版本升级">版本升级</option>
            <option value="数据修复">数据修复</option>
            <option value="性能优化">性能优化</option>
            <option value="容量变更">容量变更</option>
            <option value="应急变更">应急变更</option>
            <option value="监控配置">监控配置</option>
            <option value="其他">其他</option>
          </select>
        </div>
        <div class="form-item">
          <label>变更等级 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <select v-model="form.changeLevel" :disabled="viewMode === 'view'">
            <option value="常规">常规</option>
            <option value="重要">重要</option>
            <option value="紧急">紧急</option>
          </select>
        </div>

        <div class="form-item" style="grid-column: span 1;">
          <label>影响范围</label>
          <input v-model="form.impactScope" :disabled="viewMode === 'view'" placeholder="受影响的系统/服务" />
        </div>
        <div class="form-item" style="grid-column: span 2;">
          <label>变更IP列表</label>
          <input v-model="form.changeIpList" :disabled="viewMode === 'view'" placeholder="涉及的IP地址，多个用逗号分隔" />
        </div>

        <div class="form-item form-item-full">
          <label>变更内容 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <textarea v-model="form.changeContent" rows="6" :disabled="viewMode === 'view'" placeholder="详细描述具体操作"></textarea>
        </div>

        <div class="form-item">
          <label>变更时间 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <input type="datetime-local" v-model="form.changeTime" :disabled="viewMode === 'view'" />
        </div>

        <div class="form-item form-item-full">
          <label>回滚方案</label>
          <textarea v-model="form.rollbackPlan" rows="3" :disabled="viewMode === 'view'" placeholder="回滚方案（可选）"></textarea>
        </div>

        <div class="form-item form-item-full">
          <label>备注</label>
          <input v-model="form.remark" :disabled="viewMode === 'view'" placeholder="备注" />
        </div>

        <div class="form-item" v-if="viewMode === 'view'">
          <label>操作人</label>
          <input :value="form.id ? records.find(r => r.id === form.id)?.operator || '' : ''" disabled />
        </div>
        <div class="form-item" v-if="viewMode === 'view'">
          <label>复核人</label>
          <input :value="records.find(r => r.id === form.id)?.reviewer || ''" disabled />
        </div>
      </div>

      <div class="btn-row">
        <template v-if="viewMode === 'view'">
          <button v-if="!isCurrentCompleted" class="action-btn primary-btn" @click="startEdit(records.find(r => r.id === form.id)!)" type="button">编辑</button>
          <button v-if="!isCurrentCompleted" class="action-btn danger-btn" @click="deleteRecord(records.find(r => r.id === form.id)!)" type="button">删除</button>
          <button class="action-btn warning-btn" @click="backToList" type="button">返回列表</button>
        </template>
        <template v-else-if="!isEditMode">
          <button class="action-btn primary-btn" :disabled="loading" @click="createRecord" type="button">{{ loading ? '处理中...' : '创建记录' }}</button>
          <button class="action-btn warning-btn" :disabled="loading" @click="backToList" type="button">取消</button>
        </template>
        <template v-else>
          <button class="action-btn primary-btn" :disabled="loading" @click="updateRecord" type="button">{{ loading ? '保存中...' : '保存修改' }}</button>
          <button class="action-btn warning-btn" :disabled="loading" @click="backToList" type="button">取消</button>
        </template>
        <span v-if="message" class="inline-message">{{ message }}</span>
      </div>
    </div>

    <!-- 确认弹窗 -->
    <div v-if="confirmDialogVisible" class="confirm-overlay" @click.self="closeConfirmDialog">
      <div class="confirm-dialog" :class="{ 'large-dialog': confirmStep === 'review' }">
        <div class="confirm-header">
          <h3 v-if="confirmStep === 'review'">复核变更</h3>
          <h3 v-else-if="confirmStep === 'result'">确认变更结果</h3>
          <h3 v-else-if="confirmStep === 'rollback'">确认回滚状态</h3>
          <h3 v-else>变更已完成</h3>
          <button class="close-btn" @click="closeConfirmDialog" type="button">×</button>
        </div>
        <div class="confirm-body">
          <!-- 步骤1：待复核 → 展示全部内容，点击复核确认 -->
          <template v-if="confirmStep === 'review' && confirmTargetItem">
            <div class="review-detail-grid">
              <div class="review-detail-item"><span class="review-label">变更标题：</span>{{ confirmTargetItem.changeTitle }}</div>
              <div class="review-detail-item"><span class="review-label">变更类型：</span>{{ confirmTargetItem.changeType }}</div>
              <div class="review-detail-item"><span class="review-label">变更等级：</span><span :class="['level-tag', 'level-' + confirmTargetItem.changeLevel]">{{ confirmTargetItem.changeLevel }}</span></div>
              <div class="review-detail-item"><span class="review-label">变更时间：</span>{{ formatTime(confirmTargetItem.changeTime) }}</div>
              <div class="review-detail-item"><span class="review-label">操作人：</span>{{ confirmTargetItem.operator }}</div>
              <div class="review-detail-item full"><span class="review-label">影响范围：</span>{{ confirmTargetItem.impactScope || '-' }}</div>
              <div class="review-detail-item full"><span class="review-label">变更IP列表：</span>{{ confirmTargetItem.changeIpList || '-' }}</div>
              <div class="review-detail-item full"><span class="review-label">变更内容：</span><pre class="review-pre">{{ confirmTargetItem.changeContent }}</pre></div>
              <div class="review-detail-item full" v-if="confirmTargetItem.rollbackPlan"><span class="review-label">回滚方案：</span><pre class="review-pre">{{ confirmTargetItem.rollbackPlan }}</pre></div>
              <div class="review-detail-item full" v-if="confirmTargetItem.remark"><span class="review-label">备注：</span>{{ confirmTargetItem.remark }}</div>
            </div>
            <p class="confirm-tip">点击「复核确认」后，您将成为该变更的复核人，变更状态将变为「待变更」</p>
          </template>
          <!-- 步骤2：待变更 → 选择变更结果 -->
          <div v-else-if="confirmStep === 'result'" class="confirm-item">
            <label>变更结果</label>
            <select v-model="confirmResult">
              <option value="成功">成功</option>
              <option value="失败">失败</option>
              <option value="部分成功">部分成功</option>
            </select>
            <p class="confirm-tip" v-if="confirmResult === '失败' || confirmResult === '部分成功'">结果为失败/部分成功时，需后续确认回滚状态</p>
          </div>
          <!-- 步骤3：失败 → 确认回滚状态 -->
          <div v-else-if="confirmStep === 'rollback'" class="confirm-item">
            <label>回滚状态</label>
            <select v-model="confirmRollback">
              <option value="无需回滚">无需回滚</option>
              <option value="已回滚">已回滚</option>
              <option value="已失败">已失败</option>
            </select>
          </div>
          <!-- 已完成 -->
          <div v-else class="confirm-item">
            <p class="confirm-tip">该变更记录已完成所有流程</p>
          </div>
        </div>
        <div class="confirm-footer">
          <button class="action-btn warning-btn" @click="closeConfirmDialog" type="button">关闭</button>
          <button v-if="confirmStep === 'review'" class="action-btn primary-btn" :disabled="loading" @click="doConfirm" type="button">{{ loading ? '处理中...' : '复核确认' }}</button>
          <button v-else-if="confirmStep" class="action-btn primary-btn" :disabled="loading" @click="doConfirm" type="button">{{ loading ? '处理中...' : '确认' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 组件特有样式（公共样式见 global.css） */
.admin-page { width: 100%; }
.inline-message { color: #f56c6c; font-size: 14px; margin-left: 8px; }

/* 等级/结果标签颜色 */
.level-常规 { background: #909399; }
.level-重要 { background: #e6a23c; }
.level-紧急 { background: #f56c6c; }
.result-成功 { background: #67c23a; }
.result-失败 { background: #f56c6c; }
.result-部分成功 { background: #e6a23c; }
.rollback-sub { font-size: 12px; color: #909399; }

/* 按钮 */
.view-btn { background: #909399; }
.confirm-btn { background: #67c23a; }
.rollback-btn { background: #e6a23c; }
.delete-btn { background: #f56c6c; }
.done-text { font-size: 13px; color: #c0c4cc; }

/* 确认弹窗 */
.confirm-overlay { position: fixed; inset: 0; z-index: 3000; background: rgba(0,0,0,0.45); display: flex; align-items: center; justify-content: center; }
.confirm-dialog { background: #fff; border-radius: 8px; width: 90%; max-width: 480px; box-shadow: 0 4px 16px rgba(0,0,0,0.1); }
.confirm-header { padding: 16px 20px; border-bottom: 1px solid #ebeef5; display: flex; justify-content: space-between; align-items: center; }
.confirm-header h3 { margin: 0; font-size: 18px; color: #303133; }
.close-btn { background: transparent; border: none; font-size: 20px; cursor: pointer; color: #909399; }
.confirm-body { padding: 20px; display: flex; flex-direction: column; gap: 16px; }
.confirm-item { display: flex; flex-direction: column; gap: 8px; }
.confirm-item label { font-size: 14px; color: #333; }
.confirm-item select, .confirm-item input { width: 100%; padding: 8px 12px; border: 1px solid #dcdfe6; border-radius: 4px; font-size: 14px; font-family: Consolas, Monaco, monospace; }
.confirm-tip { margin: 8px 0 0; font-size: 13px; color: #909399; }
.large-dialog { max-width: 700px; }
.review-detail-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.review-detail-item { font-size: 14px; color: #303133; line-height: 1.6; }
.review-detail-item.full { grid-column: 1 / -1; }
.review-label { color: #909399; font-weight: bold; }
.review-pre { margin: 4px 0 0; padding: 10px; background: #f8f9fb; border: 1px solid #e4e7ed; border-radius: 4px; font-family: Consolas, Monaco, monospace; font-size: 13px; white-space: pre-wrap; word-break: break-all; }
.confirm-footer { padding: 16px 20px; border-top: 1px solid #ebeef5; display: flex; justify-content: flex-end; gap: 12px; }
</style>
