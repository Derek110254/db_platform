<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { showToast } from '../utils/toast'

const formatDate = (dateStr: string) => {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return dateStr
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

interface DBChangeRequest {
  id: number
  applicant: string
  applicantTeam: string
  environment: string
  plannedChangeTime: string
  urgencyLevel: string
  testPublisher: string
  prodPublisher: string
  releaseVerifier: string
  changeType: string
  changeReason: string
  requirementUrl: string
  impactScope: string
  dbType: string
  testDbIp: string
  testDbName: string
  testDbSchema: string
  dbIp: string
  dbName: string
  dbSchema: string
  changeContent: string
  backupTable: string
  createTime: string
  updateTime: string
}

const listLoading = ref(false)
const listData = ref<DBChangeRequest[]>([])
const totalCount = ref(0)

const searchForm = ref({
  applicantTeam: '',
  urgencyLevel: '',
  dbType: '',
  isVerified: '0',
})

const currentPage = ref(1)
const pageSize = ref(10)
const pageSizeOptions = [10, 20, 50, 100]

const viewMode = ref<'list' | 'form'>('list')
const loading = ref(false)
const message = ref('')

const editForm = ref({
  id: 0,
  testPublisher: '',
  prodPublisher: '',
})

const currentRequest = ref<DBChangeRequest | null>(null)
const currentUserName = ref('')

const totalPages = computed(() => {
  if (totalCount.value === 0) return 1
  return Math.ceil(totalCount.value / pageSize.value)
})

const isItemVerified = (item: DBChangeRequest | null) => {
  if (!item) return false
  return !!(item.testPublisher && item.prodPublisher)
}

const loadData = async () => {
  listLoading.value = true
  message.value = ''
  try {
    const params = new URLSearchParams()
    params.append('page', String(currentPage.value))
    params.append('pageSize', String(pageSize.value))
    if (searchForm.value.applicantTeam) params.append('applicantTeam', searchForm.value.applicantTeam)
    if (searchForm.value.urgencyLevel) params.append('urgencyLevel', searchForm.value.urgencyLevel)
    if (searchForm.value.dbType) params.append('dbType', searchForm.value.dbType)
    if (searchForm.value.isVerified !== '') params.append('isVerified', searchForm.value.isVerified)

    const res = await fetch(`/api/admin/db-change-requests/release?${params.toString()}`, {
      credentials: 'include',
    })
    const data = await res.json()
    if (!res.ok || !data.ok) {
      message.value = data.message || '加载列表失败'
      return
    }
    listData.value = data.records || []
    totalCount.value = data.total || 0
  } catch (err) {
    console.error(err)
    message.value = '加载失败，请检查网络或后端状态'
  } finally {
    listLoading.value = false
  }
}

const resetSearch = () => {
  searchForm.value = { applicantTeam: '', urgencyLevel: '', dbType: '', isVerified: '0' }
  currentPage.value = 1
  loadData()
}

const handleSearch = () => {
  currentPage.value = 1
  loadData()
}

const openEdit = (row: DBChangeRequest) => {
  currentRequest.value = row
  editForm.value = {
    id: row.id,
    testPublisher: row.testPublisher || '',
    prodPublisher: row.prodPublisher || '',
  }
  message.value = ''
  viewMode.value = 'form'
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

const backToList = () => {
  viewMode.value = 'list'
  message.value = ''
  loadData()
}

const saveRequest = async () => {
  loading.value = true
  message.value = ''
  try {
    const res = await fetch('/api/admin/db-change-requests/release', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(editForm.value),
    })
    const data = await res.json()
    if (!res.ok || !data.ok) {
      message.value = data.message || '保存失败'
      return
    }
    showToast(data.message || '保存成功', 'success')
    backToList()
  } catch (err) {
    console.error(err)
    message.value = '保存失败，请检查网络或后端状态'
  } finally {
    loading.value = false
  }
}

const goPrevPage = () => {
  if (currentPage.value > 1) {
    currentPage.value--
    loadData()
  }
}

const goNextPage = () => {
  if (currentPage.value < totalPages.value) {
    currentPage.value++
    loadData()
  }
}

const copyChangeContent = async () => {
  if (!currentRequest.value || !currentRequest.value.changeContent) {
    message.value = '变更内容为空，无法复制'
    return
  }
  try {
    await navigator.clipboard.writeText(currentRequest.value.changeContent)
    showToast('已复制到剪贴板', 'info')
  } catch (err) {
    console.error('复制失败:', err)
    message.value = '复制失败'
  }
}

onMounted(async () => {
  loadData()
  try {
    const res = await fetch('/api/auth/me', { credentials: 'include' })
    if (res.ok) {
      const data = await res.json()
      currentUserName.value = data.displayName || data.username || 'admin'
    }
  } catch (e) {}
})
</script>

<template>
  <div class="card panel-card">
    <!-- ============ 列表视图 ============ -->
    <template v-if="viewMode === 'list'">
      <div class="panel-header">
        <h2>数据库变更申请 (发布验证)</h2>
      </div>

      <div v-if="message" class="message-banner">{{ message }}</div>

      <div class="search-bar">
        <div class="search-item">
          <label>申请团队</label>
          <select v-model="searchForm.applicantTeam" @change="handleSearch">
            <option value="">全部</option>
            <option value="交易开发">交易开发</option>
            <option value="运营开发">运营开发</option>
            <option value="后台开发">后台开发</option>
            <option value="增长开发">增长开发</option>
          </select>
        </div>
        <div class="search-item">
          <label>紧急程度</label>
          <select v-model="searchForm.urgencyLevel" @change="handleSearch">
            <option value="">全部</option>
            <option value="常规">常规</option>
            <option value="紧急">紧急</option>
          </select>
        </div>
        <div class="search-item">
          <label>数据库类型</label>
          <select v-model="searchForm.dbType" @change="handleSearch">
            <option value="">全部</option>
            <option value="oracle">oracle</option>
            <option value="mysql">mysql</option>
            <option value="redis">redis</option>
          </select>
        </div>
        <div class="search-item">
          <label>是否通过验证</label>
          <select v-model="searchForm.isVerified" @change="handleSearch">
            <option value="">全部</option>
            <option value="1">已通过</option>
            <option value="0">未通过</option>
          </select>
        </div>
        <div class="search-actions">
          <button class="action-btn plain-btn" @click="resetSearch" type="button">重置</button>
        </div>
      </div>

      <div class="table-wrap" :class="{ loading: listLoading }">
        <table class="result-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>申请人</th>
              <th>申请团队</th>
              <th>环境</th>
              <th>计划变更时间</th>
              <th>测试线发布人</th>
              <th>生产线发布人</th>
              <th>发布验证确认</th>
              <th>测试线 IP</th>
              <th>生产线 IP</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="listData.length === 0">
              <td colspan="11" class="empty-text">暂无数据</td>
            </tr>
            <tr v-for="item in listData" :key="item.id">
              <td>{{ item.id }}</td>
              <td>{{ item.applicant }}</td>
              <td>{{ item.applicantTeam }}</td>
              <td>{{ item.environment || '-' }}</td>
              <td>{{ formatDate(item.plannedChangeTime) }}</td>
              <td>{{ item.testPublisher || '-' }}</td>
              <td>{{ item.prodPublisher || '-' }}</td>
              <td>{{ item.releaseVerifier || '-' }}</td>
              <td>{{ item.testDbIp }}</td>
              <td>{{ item.dbIp }}</td>
              <td>
                <div class="row-actions">
                  <button class="text-btn" @click="openEdit(item)" type="button">{{ isItemVerified(item) ? '查看' : '填写发布' }}</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="pagination">
        <div class="page-size">
          共 {{ totalCount }} 条，每页
          <select v-model="pageSize" @change="handleSearch">
            <option v-for="s in pageSizeOptions" :key="s" :value="s">{{ s }}</option>
          </select>
          条
        </div>
        <div class="page-nav">
          <button class="pager-btn" :disabled="currentPage <= 1" @click="goPrevPage" type="button">上一页</button>
          <span>第 {{ currentPage }} / {{ totalPages }} 页</span>
          <button class="pager-btn" :disabled="currentPage >= totalPages" @click="goNextPage" type="button">下一页</button>
        </div>
      </div>
    </template>

    <!-- ============ 详情/发布视图 ============ -->
    <template v-else>
      <div class="form-header">
        <h2>编辑发布信息</h2>
        <button class="action-btn secondary-btn back-btn" @click="backToList" type="button">← 返回列表</button>
      </div>
      <p v-if="message" class="inline-message">{{ message }}</p>

      <div class="request-details" v-if="currentRequest">
        <div class="detail-row">
          <span class="detail-label">申请人：</span>
          <span class="detail-value">{{ currentRequest.applicant }} ({{ currentRequest.applicantTeam }})</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">环境：</span>
          <span class="detail-value">{{ currentRequest.environment || '-' }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">计划时间：</span>
          <span class="detail-value">{{ formatDate(currentRequest.plannedChangeTime) }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">变更类型：</span>
          <span class="detail-value">{{ currentRequest.changeType }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">紧急程度：</span>
          <span class="detail-value">
            <span :class="['tag', currentRequest.urgencyLevel === '紧急' ? 'tag-danger' : 'tag-info']">
              {{ currentRequest.urgencyLevel }}
            </span>
          </span>
        </div>
        <div class="detail-row">
          <span class="detail-label">测试线数据库：</span>
          <span class="detail-value">{{ currentRequest.dbType }} / {{ currentRequest.testDbIp }} / {{ currentRequest.testDbName }} / {{ currentRequest.testDbSchema }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">生产线数据库：</span>
          <span class="detail-value">{{ currentRequest.dbType }} / {{ currentRequest.dbIp }} / {{ currentRequest.dbName }} / {{ currentRequest.dbSchema }}</span>
        </div>
        <div class="detail-row full-width">
          <span class="detail-label">变更原因：</span>
          <span class="detail-value">{{ currentRequest.changeReason }}</span>
        </div>
        <div class="detail-row full-width" v-if="currentRequest.requirementUrl">
          <span class="detail-label">需求链接：</span>
          <span class="detail-value"><a :href="currentRequest.requirementUrl" target="_blank">{{ currentRequest.requirementUrl }}</a></span>
        </div>
        <div class="detail-row full-width">
          <div style="display: flex; align-items: center; gap: 10px;">
            <span class="detail-label">变更内容：</span>
            <button class="action-btn plain-btn" @click="copyChangeContent" type="button" style="padding: 2px 8px; font-size: 12px; height: 24px;">复制全部</button>
          </div>
          <div class="sql-content-box">
            <pre>{{ currentRequest.changeContent }}</pre>
          </div>
        </div>
        <div class="detail-row full-width" v-if="currentRequest.backupTable">
          <span class="detail-label">备份表：</span>
          <span class="detail-value">{{ currentRequest.backupTable }}</span>
        </div>
      </div>

      <hr class="divider" />

      <div class="form-grid">
        <div class="form-item">
          <label>测试线发布人</label>
          <div class="action-input-group">
            <input v-model="editForm.testPublisher" type="text" readonly placeholder="待发布" />
            <button v-if="!isItemVerified(currentRequest) && !editForm.testPublisher" class="action-btn success-btn" @click="editForm.testPublisher = currentUserName" type="button">发布完成</button>
          </div>
        </div>
        <div class="form-item">
          <label>生产线发布人</label>
          <div class="action-input-group">
            <input v-model="editForm.prodPublisher" type="text" readonly placeholder="待发布" />
            <button v-if="!isItemVerified(currentRequest) && !editForm.prodPublisher" class="action-btn success-btn" @click="editForm.prodPublisher = currentUserName" type="button">发布完成</button>
          </div>
        </div>
      </div>

      <div class="btn-row">
        <button class="action-btn warning-btn" @click="backToList" type="button">取消</button>
        <button v-if="!isItemVerified(currentRequest)" class="action-btn primary-btn" :disabled="loading" @click="saveRequest" type="button">
          {{ loading ? '保存中...' : '保存发布信息' }}
        </button>
      </div>
    </template>
  </div>
</template>

<style scoped>
/* 组件特有样式（公共样式见 global.css） */
.panel-card { padding: 24px; }
.panel-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.panel-header h2 { margin: 0; font-size: 20px; color: #c3e50; }

/* 搜索栏 */
.search-bar { display: flex; flex-wrap: wrap; gap: 16px; margin-bottom: 20px; padding: 16px; background: #f8f9fa; border-radius: 8px; }
.search-item { display: flex; align-items: center; gap: 8px; }
.search-item label { font-size: 14px; color: #606266; white-space: nowrap; flex-shrink: 0; }
.search-item input, .search-item select { padding: 6px 10px; }
.search-actions { display: flex; gap: 10px; align-items: center; }

/* 按钮 */
.plain-btn { background: #fff; }
.text-btn { background: transparent; border: none; color: #409eff; cursor: pointer; padding: 4px 8px; font-size: 13px; }

/* 表格 */
.table-wrap { border: 1px solid #ebeef5; border-radius: 4px; margin-bottom: 20px; }
.table-wrap.loading { opacity: 0.6; pointer-events: none; }

/* 分页 */
.pagination { display: flex; justify-content: space-between; align-items: center; font-size: 14px; color: #606266; }
.page-nav { display: flex; align-items: center; gap: 12px; }

/* 表单头部 */
.form-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.back-btn { white-space: nowrap; }
.inline-message { color: #f56c6c; font-size: 14px; margin-bottom: 12px; }

/* 按钮行 */
.btn-row { display: flex; gap: 12px; flex-wrap: wrap; align-items: center; margin-top: 16px; }

/* 详情 */
.request-details { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-bottom: 16px; font-size: 14px; }
.detail-row { display: flex; flex-direction: column; gap: 4px; }
.detail-row.full-width { grid-column: span 2; }
.detail-label { color: #909399; }
.detail-value { color: #303133; }
.detail-value a { color: #409eff; text-decoration: none; }
.detail-value a:hover { text-decoration: underline; }
.sql-content-box { background: #f5f7fa; border: 1px solid #ebeef5; border-radius: 4px; padding: 10px; max-height: 200px; overflow-y: auto; }
.sql-content-box pre { margin: 0; white-space: pre-wrap; word-wrap: break-word; font-family: Consolas, Monaco, monospace; font-size: 13px; color: #333; }
.divider { border: none; border-top: 1px dashed #ebeef5; margin: 16px 0; }

/* 表单 */
.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-bottom: 16px; }
.form-item { display: flex; flex-direction: column; gap: 6px; }
.form-item label { font-size: 14px; color: #606266; }

/* 行操作 */
.row-actions { display: flex; align-items: center; gap: 2px; flex-wrap: nowrap; white-space: nowrap; }

/* 输入组 */
.action-input-group { display: flex; gap: 8px; align-items: center; }
.action-input-group input { flex: 1; background: #f5f7fa !important; color: #606266; cursor: not-allowed; }

.message-banner { background: #fdf6ec; color: #e6a23c; padding: 10px 16px; border-radius: 4px; margin-bottom: 16px; font-size: 14px; }
</style>
