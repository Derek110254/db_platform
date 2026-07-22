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

interface DBDataSyncRequest {
  id: number
  applicant: string
  applicantTeam: string
  environment: string
  expectedFinishTime: string
  urgencyLevel: string
  urgencyReason: string
  executeDba: string
  applyReason: string
  operateType: number
  sourceDb: string
  targetDbOrPerson: string
  involvedDbSchemaTable: string
  dataFilterCondition: string
  estimatedDataVolume: string
  containsSensitiveData: number
  desensitizationRule: string
  createTime: string
  updateTime: string
}

const listLoading = ref(false)
const listData = ref<DBDataSyncRequest[]>([])
const totalCount = ref(0)

const searchForm = ref({
  applicant: '',
  urgencyLevel: '',
  operateType: '' as number | '',
})

const currentPage = ref(1)
const pageSize = ref(10)
const pageSizeOptions = [10, 20, 50, 100]

const viewMode = ref<'list' | 'form'>('list')
const loading = ref(false)
const message = ref('')

const editForm = ref({
  id: 0,
  executeDba: '',
})

const currentRequest = ref<DBDataSyncRequest | null>(null)
const currentUserName = ref('')

const totalPages = computed(() => {
  if (totalCount.value === 0) return 1
  return Math.ceil(totalCount.value / pageSize.value)
})

const getOperateTypeName = (type: number) => {
  switch (type) {
    case 1: return '迁移到其他数据库'
    case 2: return '迁移到测试库'
    case 3: return '导出为文件'
    default: return '未知'
  }
}

const loadData = async () => {
  listLoading.value = true
  message.value = ''
  try {
    const params = new URLSearchParams()
    params.append('page', String(currentPage.value))
    params.append('pageSize', String(pageSize.value))
    if (searchForm.value.applicant) params.append('applicant', searchForm.value.applicant)
    if (searchForm.value.urgencyLevel) params.append('urgencyLevel', searchForm.value.urgencyLevel)
    if (searchForm.value.operateType) params.append('operateType', String(searchForm.value.operateType))

    const res = await fetch(`/api/admin/db-data-sync-requests/dba?${params.toString()}`, {
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
  searchForm.value = { applicant: '', urgencyLevel: '', operateType: '' }
  currentPage.value = 1
  loadData()
}

const handleSearch = () => {
  currentPage.value = 1
  loadData()
}

const openExecute = (row: DBDataSyncRequest) => {
  currentRequest.value = row
  editForm.value = {
    id: row.id,
    executeDba: row.executeDba || '',
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

const submitExecution = async () => {
  if (!editForm.value.executeDba) {
    message.value = '❌ 请输入执行 DBA 的姓名或标识'
    return
  }

  loading.value = true
  message.value = ''
  try {
    const res = await fetch('/api/admin/db-data-sync-requests/dba', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(editForm.value),
    })
    const data = await res.json()
    if (!res.ok || !data.ok) {
      message.value = data.message || '更新失败'
      return
    }
    showToast(data.message || '登记成功', 'success')
    backToList()
  } catch (err) {
    console.error(err)
    message.value = '请求失败，请检查网络或后端状态'
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
        <h2>数据同步 DBA 执行管理</h2>
      </div>

      <div v-if="message" class="message-banner">{{ message }}</div>

      <div class="search-bar">
        <div class="search-item">
          <label>申请人</label>
          <input v-model="searchForm.applicant" type="text" placeholder="申请人标识（回车搜索）" @keyup.enter="handleSearch" />
        </div>
        <div class="search-item">
          <label>操作类型</label>
          <select v-model="searchForm.operateType" @change="handleSearch">
            <option value="">全部</option>
            <option :value="1">迁移到其他数据库</option>
            <option :value="2">迁移到测试库</option>
            <option :value="3">导出为文件</option>
          </select>
        </div>
        <div class="search-item">
          <label>紧急程度</label>
          <select v-model="searchForm.urgencyLevel" @change="handleSearch">
            <option value="">全部</option>
            <option value="常规">常规</option>
            <option value="重要">重要</option>
            <option value="紧急">紧急</option>
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
              <th>申请时间</th>
              <th>期望完成时间</th>
              <th>紧急程度</th>
              <th>操作类型</th>
              <th>源数据库</th>
              <th>目标库/人</th>
              <th>执行DBA</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="listData.length === 0">
              <td colspan="10" class="empty-text">暂无数据</td>
            </tr>
            <tr v-for="item in listData" :key="item.id">
              <td>{{ item.id }}</td>
              <td>{{ item.applicant }}</td>
              <td>{{ formatDate(item.createTime) }}</td>
              <td>{{ formatDate(item.expectedFinishTime) }}</td>
              <td>
                <span :class="['tag', item.urgencyLevel === '紧急' ? 'tag-danger' : (item.urgencyLevel === '重要' ? 'tag-warning' : 'tag-info')]">
                  {{ item.urgencyLevel }}
                </span>
              </td>
              <td>{{ getOperateTypeName(item.operateType) }}</td>
              <td>{{ item.sourceDb }}</td>
              <td>{{ item.targetDbOrPerson }}</td>
              <td>
                <span v-if="item.executeDba" class="tag tag-success">{{ item.executeDba }}</span>
                <span v-else class="tag tag-warning">待执行</span>
              </td>
              <td>
                <div class="row-actions">
                  <button class="text-btn" @click="openExecute(item)" type="button">
                    {{ item.executeDba ? '查看' : '执行登记' }}
                  </button>
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

    <!-- ============ 详情/执行登记视图 ============ -->
    <template v-else>
      <div class="form-header">
        <h2>数据同步执行登记 (DBA)</h2>
        <button class="action-btn secondary-btn back-btn" @click="backToList" type="button">← 返回列表</button>
      </div>
      <p v-if="message" class="inline-message">{{ message }}</p>

      <div class="request-details" v-if="currentRequest">
        <div class="detail-row">
          <span class="detail-label">申请人：</span>
          <span class="detail-value">{{ currentRequest.applicant }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">申请团队：</span>
          <span class="detail-value">{{ currentRequest.applicantTeam || '-' }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">申请环境：</span>
          <span class="detail-value">{{ currentRequest.environment || '-' }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">申请时间：</span>
          <span class="detail-value">{{ formatDate(currentRequest.createTime) }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">期望完成：</span>
          <span class="detail-value">{{ formatDate(currentRequest.expectedFinishTime) }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">操作类型：</span>
          <span class="detail-value highlight-text">{{ getOperateTypeName(currentRequest.operateType) }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">紧急程度：</span>
          <span class="detail-value">
            <span :class="['tag', currentRequest.urgencyLevel === '紧急' ? 'tag-danger' : (currentRequest.urgencyLevel === '重要' ? 'tag-warning' : 'tag-info')]">
              {{ currentRequest.urgencyLevel }}
            </span>
            <span v-if="currentRequest.urgencyReason" class="reason-text"> ({{ currentRequest.urgencyReason }})</span>
          </span>
        </div>
        <div class="detail-row full-width">
          <span class="detail-label">源数据库：</span>
          <span class="detail-value">{{ currentRequest.sourceDb }}</span>
        </div>
        <div class="detail-row full-width">
          <span class="detail-label">目标库/人：</span>
          <span class="detail-value">{{ currentRequest.targetDbOrPerson }}</span>
        </div>
        <div class="detail-row full-width">
          <span class="detail-label">申请原因与目的：</span>
          <div class="text-box">{{ currentRequest.applyReason }}</div>
        </div>
        <div class="detail-row full-width">
          <span class="detail-label">涉及库/表：</span>
          <div class="text-box">{{ currentRequest.involvedDbSchemaTable }}</div>
        </div>
        <div class="detail-row full-width" v-if="currentRequest.dataFilterCondition">
          <span class="detail-label">过滤条件：</span>
          <div class="text-box code-font">{{ currentRequest.dataFilterCondition }}</div>
        </div>
        <div class="detail-row">
          <span class="detail-label">预估数据量：</span>
          <span class="detail-value">{{ currentRequest.estimatedDataVolume }}</span>
        </div>
        <div class="detail-row full-width" v-if="currentRequest.containsSensitiveData === 1">
          <span class="detail-label" style="color: #f56c6c;">脱敏规则说明：</span>
          <div class="text-box">{{ currentRequest.desensitizationRule }}</div>
        </div>
      </div>

      <hr class="divider" />

      <div class="form-grid">
        <div class="form-item">
          <label>执行 DBA</label>
          <div class="action-input-group">
            <input v-model="editForm.executeDba" type="text" readonly placeholder="待执行" />
            <button v-if="!currentRequest?.executeDba && !editForm.executeDba" class="action-btn success-btn" @click="editForm.executeDba = currentUserName" type="button">执行完成</button>
          </div>
        </div>
      </div>

      <div class="btn-row">
        <button class="action-btn warning-btn" @click="backToList" type="button">
          {{ currentRequest?.executeDba ? '关闭' : '取消' }}
        </button>
        <button v-if="!currentRequest?.executeDba" class="action-btn primary-btn" :disabled="loading" @click="submitExecution" type="button">
          {{ loading ? '提交中...' : '登记执行完成' }}
        </button>
      </div>
    </template>
  </div>
</template>

<style scoped>
/* 组件特有样式（公共样式见 global.css） */
.panel-card { padding: 24px; }
.panel-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.panel-header h2 { margin: 0; font-size: 20px; color: #2c3e50; }

/* 搜索栏 */
.search-bar { display: flex; flex-wrap: wrap; gap: 16px; margin-bottom: 20px; padding: 16px; background: #f8f9fa; border-radius: 8px; }
.search-item { display: flex; align-items: center; gap: 8px; }
.search-item label { font-size: 14px; color: #606266; white-space: nowrap; flex-shrink: 0; }
.search-item input, .search-item select { padding: 6px 10px; }
.search-actions { display: flex; gap: 10px; align-items: center; }

/* 按钮 */
.plain-btn { background: #fff; }
.text-btn { background: transparent; border: none; color: #409eff; cursor: pointer; padding: 4px 8px; font-size: 13px; }

/* 行操作 */
.row-actions { display: flex; align-items: center; gap: 2px; flex-wrap: nowrap; white-space: nowrap; }

/* 表格 */
.table-wrap { min-height: 520px; border: 1px solid #ebeef5; border-radius: 4px; margin-bottom: 20px; }
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
.highlight-text { font-weight: bold; color: #409eff; }
.reason-text { color: #f56c6c; font-size: 12px; }
.text-box { background: #fff; border: 1px solid #dcdfe6; padding: 8px; border-radius: 4px; width: 100%; min-height: 40px; white-space: pre-wrap; word-break: break-all; }
.code-font { font-family: Consolas, Monaco, monospace; }
.divider { border: none; border-top: 1px dashed #ebeef5; margin: 16px 0; }

/* 表单 */
.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-bottom: 16px; }
.form-item { display: flex; flex-direction: column; gap: 6px; }
.form-item label { font-size: 14px; color: #606266; }

/* 输入组 */
.action-input-group { display: flex; gap: 8px; align-items: center; }
.action-input-group input { flex: 1; background: #f5f7fa !important; color: #606266; cursor: not-allowed; }

.message-banner { background: #fdf6ec; color: #e6a23c; padding: 10px 16px; border-radius: 4px; margin-bottom: 16px; font-size: 14px; }
</style>
