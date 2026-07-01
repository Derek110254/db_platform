<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'

const formatDate = (dateStr: string) => {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return dateStr
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
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

const dialogVisible = ref(false)
const dialogLoading = ref(false)
const dialogTitle = ref('编辑发布验证信息')
const message = ref('')

const editForm = ref({
  id: 0,
  testPublisher: '',
  prodPublisher: '',
  releaseVerifier: ''
})

const currentRequest = ref<DBChangeRequest | null>(null)

const totalPages = computed(() => {
  if (totalCount.value === 0) return 1
  return Math.ceil(totalCount.value / pageSize.value)
})

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

const openEditDialog = (row: DBChangeRequest) => {
  currentRequest.value = row
  editForm.value = {
    id: row.id,
    testPublisher: row.testPublisher || '',
    prodPublisher: row.prodPublisher || '',
    releaseVerifier: row.releaseVerifier || ''
  }
  dialogVisible.value = true
  message.value = ''
}

const closeDialog = () => {
  dialogVisible.value = false
}

const saveRequest = async () => {
  dialogLoading.value = true
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
    
    dialogVisible.value = false
    loadData()
  } catch (err) {
    console.error(err)
    message.value = '保存失败，请检查网络或后端状态'
  } finally {
    dialogLoading.value = false
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
    const oldMsg = message.value
    message.value = '已复制到剪贴板'
    setTimeout(() => {
      if (message.value === '已复制到剪贴板') {
        message.value = oldMsg
      }
    }, 2000)
  } catch (err) {
    console.error('复制失败:', err)
    message.value = '复制失败'
  }
}

const currentUserName = ref('')

const isItemVerified = (item: DBChangeRequest | null) => {
  if (!item) return false
  return !!(item.testPublisher && item.prodPublisher && item.releaseVerifier)
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
    <div class="panel-header">
      <h2>数据库变更申请 (发布验证)</h2>
    </div>
    
    <div v-if="message && !dialogVisible" class="message-banner">{{ message }}</div>

    <div class="search-bar">
      <div class="search-item">
        <label>申请团队</label>
        <select v-model="searchForm.applicantTeam">
          <option value="">全部</option>
          <option value="交易开发">交易开发</option>
          <option value="运营开发">运营开发</option>
          <option value="后台开发">后台开发</option>
          <option value="增长开发">增长开发</option>
        </select>
      </div>
      <div class="search-item">
        <label>紧急程度</label>
        <select v-model="searchForm.urgencyLevel">
          <option value="">全部</option>
          <option value="常规">常规</option>
          <option value="紧急">紧急</option>
        </select>
      </div>
      <div class="search-item">
        <label>数据库类型</label>
        <select v-model="searchForm.dbType">
          <option value="">全部</option>
          <option value="oracle">oracle</option>
          <option value="mysql">mysql</option>
          <option value="redis">redis</option>
        </select>
      </div>
      <div class="search-item">
        <label>是否通过验证</label>
        <select v-model="searchForm.isVerified">
          <option value="">全部</option>
          <option value="1">已通过</option>
          <option value="0">未通过</option>
        </select>
      </div>
      <div class="search-actions">
        <button class="action-btn" @click="handleSearch" type="button">搜索</button>
        <button class="action-btn plain-btn" @click="resetSearch" type="button">重置</button>
      </div>
    </div>

    <div class="table-wrap" :class="{ loading: listLoading }">
      <table class="data-table">
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
            <td colspan="11" class="empty-cell">暂无数据</td>
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
                <button class="text-btn" @click="openEditDialog(item)" type="button">{{ isItemVerified(item) ? '查看' : '填写验证' }}</button>
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

    <!-- 弹窗 -->
    <div v-if="dialogVisible" class="dialog-overlay">
      <div class="dialog-content">
        <div class="dialog-header">
          <h3>{{ dialogTitle }}</h3>
          <button class="close-btn" @click="closeDialog" type="button">×</button>
        </div>
        <div class="dialog-body">
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
          </div>
          
          <hr class="divider" />

          <div class="form-grid">
            <div class="form-item">
              <label>测试线发布人</label>
              <div class="action-input-group">
                <input v-model="editForm.testPublisher" type="text" readonly placeholder="待填写" />
                <button v-if="!isItemVerified(currentRequest) && !editForm.testPublisher" class="action-btn" @click="editForm.testPublisher = currentUserName" type="button">由我发布</button>
              </div>
            </div>
            <div class="form-item">
              <label>生产线发布人</label>
              <div class="action-input-group">
                <input v-model="editForm.prodPublisher" type="text" readonly placeholder="待填写" />
                <button v-if="!isItemVerified(currentRequest) && !editForm.prodPublisher" class="action-btn" @click="editForm.prodPublisher = currentUserName" type="button">由我发布</button>
              </div>
            </div>
            <div class="form-item full-width">
              <label>发布验证确认人</label>
              <div class="action-input-group">
                <input v-model="editForm.releaseVerifier" type="text" readonly placeholder="待填写" />
                <button v-if="!isItemVerified(currentRequest) && !editForm.releaseVerifier" class="action-btn" @click="editForm.releaseVerifier = currentUserName" type="button">由我验证</button>
              </div>
            </div>
          </div>
        </div>
        <div class="dialog-footer">
          <span v-if="message" class="error-message">{{ message }}</span>
          <button class="action-btn plain-btn" @click="closeDialog" :disabled="dialogLoading" type="button">
            {{ isItemVerified(currentRequest) ? '关闭' : '取消' }}
          </button>
          <button v-if="!isItemVerified(currentRequest)" class="action-btn primary-btn" @click="saveRequest" :disabled="dialogLoading" type="button">
            {{ dialogLoading ? '保存中...' : '确定保存' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.panel-card {
  padding: 24px;
}
.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}
.panel-header h2 {
  margin: 0;
  font-size: 20px;
  color: #2c3e50;
}
.search-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  margin-bottom: 20px;
  padding: 16px;
  background: #f8f9fa;
  border-radius: 8px;
}
.search-item {
  display: flex;
  align-items: center;
  gap: 8px;
}
.search-item label {
  font-size: 14px;
  color: #606266;
}
.search-item input,
.search-item select {
  padding: 6px 10px;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  font-size: 14px;
}
.search-actions {
  display: flex;
  gap: 10px;
  align-items: center;
}
.action-btn {
  padding: 8px 16px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 14px;
  background: #fff;
  border: 1px solid #dcdfe6;
  color: #606266;
}
.primary-btn {
  background: #409eff;
  border-color: #409eff;
  color: #fff;
}
.plain-btn {
  background: #fff;
}
.text-btn {
  background: transparent;
  border: none;
  color: #409eff;
  cursor: pointer;
  padding: 4px 8px;
  font-size: 13px;
}
.table-wrap {
  width: 100%;
  overflow-x: auto;
  border: 1px solid #ebeef5;
  border-radius: 4px;
  margin-bottom: 20px;
}
.table-wrap.loading {
  opacity: 0.6;
  pointer-events: none;
}
.data-table {
  width: 100%;
  border-collapse: collapse;
}
.data-table th,
.data-table td {
  padding: 12px;
  text-align: left;
  border-bottom: 1px solid #ebeef5;
  font-size: 14px;
}
.data-table th {
  background: #f5f7fa;
  color: #909399;
  font-weight: bold;
}
.empty-cell {
  text-align: center !important;
  color: #909399;
  padding: 30px !important;
}

.pagination {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 14px;
  color: #606266;
}
.page-nav {
  display: flex;
  align-items: center;
  gap: 12px;
}
.pager-btn {
  padding: 6px 12px;
  border: 1px solid #dcdfe6;
  background: #fff;
  border-radius: 4px;
  cursor: pointer;
}
.pager-btn:disabled {
  background: #f5f7fa;
  color: #c0c4cc;
  cursor: not-allowed;
}

.dialog-overlay {
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,0.5);
  display: flex;
  justify-content: center;
  align-items: flex-start;
  padding-top: 10vh;
  z-index: 2000;
  overflow-y: auto;
}
.dialog-content {
  background: #fff;
  border-radius: 8px;
  width: 90%;
  max-width: 500px;
  box-shadow: 0 4px 16px rgba(0,0,0,0.1);
  margin-bottom: 5vh;
}
.dialog-header {
  padding: 16px 20px;
  border-bottom: 1px solid #ebeef5;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.dialog-header h3 {
  margin: 0;
  font-size: 18px;
  color: #303133;
}
.close-btn {
  background: transparent;
  border: none;
  font-size: 20px;
  cursor: pointer;
  color: #909399;
}
.dialog-body {
  padding: 20px;
}
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}
.form-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.form-item.full-width {
  grid-column: span 2;
}
.form-item label {
  font-size: 14px;
  color: #606266;
}
.form-item input[type="text"] {
  padding: 8px;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  font-size: 14px;
  width: 100%;
}
.dialog-footer {
  padding: 16px 20px;
  border-top: 1px solid #ebeef5;
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}
.error-message {
  color: #f56c6c;
  font-size: 14px;
  margin-right: auto;
  align-self: center;
}
.message-banner {
  background: #fdf6ec;
  color: #e6a23c;
  padding: 10px 16px;
  border-radius: 4px;
  margin-bottom: 16px;
  font-size: 14px;
}

.request-details {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin-bottom: 16px;
  font-size: 14px;
}
.detail-row {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.detail-row.full-width {
  grid-column: span 2;
}
.detail-label {
  color: #909399;
}
.detail-value {
  color: #303133;
}
.detail-value a {
  color: #409eff;
  text-decoration: none;
}
.detail-value a:hover {
  text-decoration: underline;
}
.sql-content-box {
  background: #f5f7fa;
  border: 1px solid #ebeef5;
  border-radius: 4px;
  padding: 10px;
  max-height: 200px;
  overflow-y: auto;
}
.sql-content-box pre {
  margin: 0;
  white-space: pre-wrap;
  word-wrap: break-word;
  font-family: Consolas, Monaco, monospace;
  font-size: 13px;
  color: #333;
}
.divider {
  border: none;
  border-top: 1px dashed #ebeef5;
  margin: 16px 0;
}
.tag {
  display: inline-block;
  padding: 2px 6px;
  font-size: 12px;
  border-radius: 4px;
  background: #f4f4f5;
  color: #909399;
  border: 1px solid #e9e9eb;
}
.tag-danger {
  background: #fef0f0;
  color: #f56c6c;
  border-color: #fde2e2;
}
.tag-info {
  background: #f4f4f5;
  color: #909399;
  border-color: #e9e9eb;
}
.dialog-content {
  background: #fff;
  border-radius: 8px;
  width: 90%;
  max-width: 650px;
  box-shadow: 0 4px 16px rgba(0,0,0,0.1);
  margin-bottom: 5vh;
}
.action-input-group {
  display: flex;
  gap: 8px;
  align-items: center;
}
.action-input-group input {
  flex: 1;
  background-color: #f5f7fa !important;
  color: #606266;
  cursor: not-allowed;
}
</style>
