<script setup lang="ts">
/**
 * AdminDbAlertHandlePanel.vue
 * ------------------------------------------------------------------
 * 该组件是「数据库告警处理（管理员）」页面。
 *
 * 布局模式：list 列表 ↔ form 新增/编辑，列表头部下拉筛选自动响应。
 *
 * 主要功能：
 * 1. 分页展示告警处理记录，按数据库类型 / 告警等级 / 告警分类自动筛选。
 * 2. 新增告警处理记录（普通用户也可提交）。
 * 3. 管理员补录处理结果：处理人、处理开始/结束时间、处理结果。
 * 4. 编辑 / 删除（按 handler 归属区分权限）。
 *
 * 关键接口：
 * - GET    /api/db-alert-handles              用户查询/CRUD
 * - GET    /api/admin/db-alert-handles         管理员查询全部
 * - PUT    /api/admin/db-alert-handles/result  补录处理结果
 */

import { onMounted, ref } from 'vue'

interface AlertHandleItem {
  id: number
  dbType: string
  alertLevel: string
  alertCategory: string
  alertContent: string
  impactScope: string
  alertTime: string
  handler: string
  handleStartTime: string
  handleEndTime: string
  handleResult: string
  createTime: string
  updateTime: string
}

const loading = ref(false)
const message = ref('')

/**
 * 视图模式：list 列表 / view 查看（只读）/ form 登记（新增/编辑）
 */
const viewMode = ref<'list' | 'view' | 'form'>('list')

/**
 * 是否为编辑模式（form 视图下区分新增与编辑）
 */
const isEditMode = ref(false)

const records = ref<AlertHandleItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const filterDbType = ref('')
const filterAlertLevel = ref('')
const filterAlertCategory = ref('')

/**
 * 将 datetime-local 控件值（2026-07-08T12:00）转为后端格式（2026-07-08 12:00:00）
 */
const toBackendTime = (v: string): string => {
  if (!v) return ''
  return v.replace('T', ' ') + (v.length === 16 ? ':00' : '')
}

/**
 * 将后端时间字符串（2026-07-08 12:00:00）转为 datetime-local 控件值（2026-07-08T12:00）
 */
const toLocalTime = (v: string): string => {
  if (!v) return ''
  return v.replace(' ', 'T').slice(0, 16)
}

/**
 * 将后端时间字符串（如 2026-07-08T08:07:00+08:00 或 2026-07-08 08:07:00）
 * 统一格式化为 2026-07-08 08:07
 */
const formatTime = (v: string): string => {
  if (!v) return ''
  const s = v.replace('T', ' ')
  return s.slice(0, 16)
}

/**
 * 截断文本，超过 maxLen 字符用省略号表示
 */
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
  dbType: '',
  alertLevel: '一般',
  alertCategory: '',
  alertContent: '',
  impactScope: '',
  alertTime: '',
  handler: '',
  handleStartTime: '',
  handleEndTime: '',
  handleResult: '',
})

const resetForm = () => {
  isEditMode.value = false
  form.value = {
    id: 0,
    dbType: '',
    alertLevel: '一般',
    alertCategory: '',
    alertContent: '',
    impactScope: '',
    alertTime: '',
    handler: '',
    handleStartTime: '',
    handleEndTime: '',
    handleResult: '',
  }
}

const fillForm = (item: AlertHandleItem) => {
  form.value = {
    id: item.id,
    dbType: item.dbType,
    alertLevel: item.alertLevel || '一般',
    alertCategory: item.alertCategory || '',
    alertContent: item.alertContent,
    impactScope: item.impactScope,
    alertTime: toLocalTime(item.alertTime),
    handler: item.handler,
    handleStartTime: toLocalTime(item.handleStartTime),
    handleEndTime: toLocalTime(item.handleEndTime),
    handleResult: item.handleResult,
  }
}

/**
 * 进入新增视图
 */
const showCreateForm = () => {
  resetForm()
  message.value = '请填写告警处理记录信息'
  viewMode.value = 'form'
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

/**
 * 进入查看视图（只读）
 */
const startView = (item: AlertHandleItem) => {
  fillForm(item)
  message.value = `查看记录：ID=${item.id}`
  viewMode.value = 'view'
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

/**
 * 从查看视图切换到编辑视图
 */
const editFromView = () => {
  isEditMode.value = true
  message.value = `正在编辑记录：ID=${form.value.id}`
  viewMode.value = 'form'
}

/**
 * 进入编辑视图
 */
const startEdit = (item: AlertHandleItem) => {
  fillForm(item)
  isEditMode.value = true
  message.value = `正在编辑记录：ID=${item.id}`
  viewMode.value = 'form'
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

/**
 * 返回列表视图
 */
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
      dbType: filterDbType.value,
      alertLevel: filterAlertLevel.value,
      alertCategory: filterAlertCategory.value,
    })
    const res = await fetch(`/api/admin/db-alert-handles?${params.toString()}`, {
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
    message.value = '加载列表成功'
  } catch (err) {
    console.error(err)
    message.value = '加载列表失败，请检查后端是否启动'
    records.value = []
  } finally {
    loading.value = false
  }
}

const createRecord = async () => {
  loading.value = true
  message.value = ''

  try {
    const res = await fetch('/api/db-alert-handles', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({
        ...form.value,
        alertTime: toBackendTime(form.value.alertTime),
        handleStartTime: toBackendTime(form.value.handleStartTime),
        handleEndTime: toBackendTime(form.value.handleEndTime),
      }),
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || '创建记录失败'
      return
    }

    message.value = data.message || '创建记录成功'
    backToList()
  } catch (err) {
    console.error(err)
    message.value = '创建记录失败，请检查后端是否启动'
  } finally {
    loading.value = false
  }
}

const updateRecord = async () => {
  loading.value = true
  message.value = ''

  try {
    const res = await fetch('/api/db-alert-handles', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({
        ...form.value,
        alertTime: toBackendTime(form.value.alertTime),
        handleStartTime: toBackendTime(form.value.handleStartTime),
        handleEndTime: toBackendTime(form.value.handleEndTime),
      }),
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || '更新记录失败'
      return
    }

    message.value = data.message || '更新记录成功'
    backToList()
  } catch (err) {
    console.error(err)
    message.value = '更新记录失败，请检查后端是否启动'
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
        <h2>告警处理记录列表 (总数: {{ total }})</h2>
        <div class="header-actions">
          <div class="filter-row">
            <select v-model="filterDbType" @change="handleFilterChange" class="filter-select">
              <option value="">全部类型</option>
              <option value="MySQL">MySQL</option>
              <option value="Oracle">Oracle</option>
              <option value="redis">redis</option>
              <option value="其他">其他</option>
            </select>
            <select v-model="filterAlertLevel" @change="handleFilterChange" class="filter-select">
              <option value="">全部等级</option>
              <option value="一般">一般</option>
              <option value="重要">重要</option>
              <option value="紧急">紧急</option>
            </select>
            <select v-model="filterAlertCategory" @change="handleFilterChange" class="filter-select">
              <option value="">全部分类</option>
              <option value="SQL性能">SQL性能</option>
              <option value="空间扩容">空间扩容</option>
              <option value="配置优化">配置优化</option>
              <option value="可用性故障">可用性故障</option>
              <option value="锁与阻塞">锁与阻塞</option>
              <option value="备份恢复">备份恢复</option>
              <option value="硬件不足">硬件不足</option>
            </select>
          </div>
          <button class="action-btn primary-btn" :disabled="loading" @click="showCreateForm" type="button">
            + 新增告警
          </button>
        </div>
      </div>

      <p class="result">{{ message }}</p>

      <div class="table-wrap">
        <table class="result-table">
          <colgroup>
            <col style="width: 7%" />
            <col style="width: 6%" />
            <col style="width: 6%" />
            <col style="width: 20%" />
            <col style="width: 7%" />
            <col style="width: 9%" />
            <col style="width: 9%" />
            <col style="width: 9%" />
            <col style="width: 5%" />
            <col style="width: 15%" />
            <col style="width: 7%" />
          </colgroup>
          <thead>
            <tr>
              <th>数据库类型</th>
              <th>告警等级</th>
              <th>告警分类</th>
              <th>告警内容</th>
              <th>影响范围</th>
              <th>告警时间</th>
              <th>处理开始时间</th>
              <th>处理结束时间</th>
              <th>处理人</th>
              <th>处理结果</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in records" :key="item.id" class="data-row" @dblclick="startView(item)">
              <td>{{ item.dbType }}</td>
              <td><span :class="['level-tag', 'level-' + item.alertLevel]">{{ item.alertLevel }}</span></td>
              <td>{{ item.alertCategory }}</td>
              <td class="cell-wrap" :title="item.alertContent">{{ truncate(item.alertContent, 50) }}</td>
              <td class="cell-wrap">{{ item.impactScope }}</td>
              <td>{{ formatTime(item.alertTime) }}</td>
              <td>{{ formatTime(item.handleStartTime) }}</td>
              <td>{{ formatTime(item.handleEndTime) }}</td>
              <td>{{ item.handler }}</td>
              <td class="cell-wrap" :title="item.handleResult">{{ truncate(item.handleResult, 50) }}</td>
              <td>
                <div class="row-btns">
                  <button @click="startView(item)" class="mini-btn view-btn">查看</button>
                  <button @click="startEdit(item)" class="mini-btn edit-btn">编辑</button>
                </div>
              </td>
            </tr>
            <tr v-if="records.length === 0">
              <td colspan="11" class="empty-text">暂无数据</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="pagination">
        <button
          class="action-btn secondary-btn"
          :disabled="page <= 1 || loading"
          @click="handlePageChange(-1)"
        >
          上一页
        </button>
        <span class="page-info">第 {{ page }} 页</span>
        <button
          class="action-btn secondary-btn"
          :disabled="records.length < pageSize || loading"
          @click="handlePageChange(1)"
        >
          下一页
        </button>
      </div>
    </div>

    <!-- ============ 查看 / 登记 / 编辑视图 ============ -->
    <div v-else class="card form-card">
      <div class="form-header">
        <h2 v-if="viewMode === 'view'">查看数据库告警处理记录</h2>
        <h2 v-else-if="isEditMode">编辑数据库告警处理记录</h2>
        <h2 v-else>登记数据库告警处理记录</h2>
        <button class="action-btn secondary-btn back-btn" @click="backToList" type="button">
          ← 返回列表
        </button>
      </div>
      <p class="result">{{ message }}</p>

      <div class="form-grid">
        <!-- 第一行：数据库类型 / 告警等级 / 告警分类 -->
        <div class="form-item">
          <label>数据库类型</label>
          <select v-model="form.dbType" :disabled="viewMode === 'view'">
            <option value="">请选择数据库类型</option>
            <option value="MySQL">MySQL</option>
            <option value="Oracle">Oracle</option>
            <option value="redis">redis</option>
            <option value="其他">其他</option>
          </select>
        </div>
        <div class="form-item">
          <label>告警等级</label>
          <select v-model="form.alertLevel" :disabled="viewMode === 'view'">
            <option value="一般">一般</option>
            <option value="重要">重要</option>
            <option value="紧急">紧急</option>
          </select>
        </div>
        <div class="form-item">
          <label>告警分类</label>
          <select v-model="form.alertCategory" :disabled="viewMode === 'view'">
            <option value="">请选择告警分类</option>
            <option value="SQL性能">SQL性能</option>
            <option value="空间扩容">空间扩容</option>
            <option value="配置优化">配置优化</option>
            <option value="可用性故障">可用性故障</option>
            <option value="锁与阻塞">锁与阻塞</option>
            <option value="备份恢复">备份恢复</option>
            <option value="硬件不足">硬件不足</option>
          </select>
        </div>

        <!-- 告警内容（整行） -->
        <div class="form-item form-item-full">
          <label>告警内容</label>
          <textarea v-model="form.alertContent" rows="6" :disabled="viewMode === 'view'" placeholder="请输入告警内容"></textarea>
        </div>

        <!-- 第二行：告警时间 / 处理开始时间 / 处理结束时间 -->
        <div class="form-item">
          <label>告警时间</label>
          <input type="datetime-local" v-model="form.alertTime" :disabled="viewMode === 'view'" />
        </div>
        <div class="form-item">
          <label>处理开始时间</label>
          <input type="datetime-local" v-model="form.handleStartTime" :disabled="viewMode === 'view'" />
        </div>
        <div class="form-item">
          <label>处理结束时间</label>
          <input type="datetime-local" v-model="form.handleEndTime" :disabled="viewMode === 'view'" />
        </div>

        <!-- 影响范围（整行） -->
        <div class="form-item form-item-full">
          <label>影响范围</label>
          <input v-model="form.impactScope" :disabled="viewMode === 'view'" placeholder="请输入影响范围" />
        </div>

        <!-- 处理结果（整行） -->
        <div class="form-item form-item-full">
          <label>处理结果</label>
          <textarea v-model="form.handleResult" rows="3" :disabled="viewMode === 'view'" placeholder="请输入处理结果"></textarea>
        </div>

        <!-- 处理人（仅查看时展示，整行只读） -->
        <div class="form-item form-item-full" v-if="viewMode === 'view'">
          <label>处理人</label>
          <input v-model="form.handler" disabled />
        </div>
      </div>

      <div class="btn-row">
        <!-- 查看模式：可切换为编辑 -->
        <template v-if="viewMode === 'view'">
          <button
            @click="editFromView"
            class="action-btn primary-btn"
            :disabled="loading"
            type="button"
          >
            编辑
          </button>
          <button
            class="action-btn warning-btn"
            :disabled="loading"
            @click="backToList"
            type="button"
          >
            返回列表
          </button>
        </template>

        <!-- 新增模式 -->
        <template v-else-if="!isEditMode">
          <button
            @click="createRecord"
            class="action-btn primary-btn"
            :disabled="loading"
            type="button"
          >
            {{ loading ? '处理中...' : '创建记录' }}
          </button>
          <button
            class="action-btn warning-btn"
            :disabled="loading"
            @click="backToList"
            type="button"
          >
            取消
          </button>
        </template>

        <!-- 编辑模式 -->
        <template v-else>
          <button
            @click="updateRecord"
            class="action-btn primary-btn"
            :disabled="loading"
            type="button"
          >
            {{ loading ? '保存中...' : '保存修改' }}
          </button>
          <button
            class="action-btn warning-btn"
            :disabled="loading"
            @click="backToList"
            type="button"
          >
            取消
          </button>
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 组件特有样式（公共样式见 global.css） */
.admin-page { width: 100%; }

/* 告警等级标签颜色 */
.level-一般 { background: #909399; }
.level-重要 { background: #e6a23c; }
.level-紧急 { background: #f56c6c; }

/* 按钮 */
.view-btn { background: #909399; }
.edit-btn { background: #409eff; }

/* 筛选行 */
.filter-row { display: flex; gap: 12px; }
</style>
