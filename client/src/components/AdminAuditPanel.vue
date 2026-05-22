<script setup lang="ts">
import { onMounted, ref } from 'vue'

interface AdminSqlAuditRecord {
  id: number
  userId: number
  username: string
  displayName: string
  connectionName: string
  sqlText: string
  aiSuggestion: string
  aiScore: number
  submitAudit: number
  auditPassed: number
  reviewer: string
  remark: string
  createTime: string
}

const loading = ref(false)
const auditList = ref<AdminSqlAuditRecord[]>([])
const message = ref('')

const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)

const filterStatus = ref('')
const filterSubmitter = ref('')
const filterConnection = ref('')

const toastMessage = ref('')
const showToast = (msg: string) => {
  toastMessage.value = msg
  setTimeout(() => {
    toastMessage.value = ''
  }, 2000)
}

const handleSizeChange = (val: number) => {
  pageSize.value = val
  currentPage.value = 1
  loadAudits()
}

const handleCurrentChange = (val: number) => {
  currentPage.value = val
  loadAudits()
}

const searchWithReset = () => {
  currentPage.value = 1
  loadAudits()
}

const resetSearch = () => {
  filterStatus.value = ''
  filterSubmitter.value = ''
  filterConnection.value = ''
  currentPage.value = 1
  loadAudits()
}

const loadAudits = async () => {
  loading.value = true
  message.value = '加载中...'
  try {
    const params = new URLSearchParams()
    params.append('page', currentPage.value.toString())
    params.append('pageSize', pageSize.value.toString())
    if (filterStatus.value) params.append('status', filterStatus.value)
    if (filterSubmitter.value) params.append('submitter', filterSubmitter.value.trim())
    if (filterConnection.value) params.append('connection', filterConnection.value.trim())

    const url = `/api/admin/audits?${params.toString()}`

    const res = await fetch(url, {
      credentials: 'include',
    })
    
    if (res.status === 401 || res.status === 403) {
      message.value = '无权限访问或登录已失效'
      auditList.value = []
      total.value = 0
      return
    }
    
    const data = await res.json()
    auditList.value = data.history || []
    total.value = data.total || 0
    if (auditList.value.length === 0) {
      message.value = '暂无已提交的审核记录'
    } else {
      message.value = ''
    }
  } catch (err) {
    console.error(err)
    message.value = '获取审核列表失败'
  } finally {
    loading.value = false
  }
}

const doReviewAudit = async (auditId: number, auditPassed: number) => {
  try {
    const res = await fetch('/api/admin/audits/review', {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        auditId: auditId,
        auditPassed: auditPassed
      }),
      credentials: 'include'
    })

    const data = await res.json()
    if (res.ok) {
      showToast(data.message || '审核操作成功')
      loadAudits() // 刷新列表
    } else {
      showToast(data.error || data.message || '审核操作失败')
    }
  } catch (err) {
    console.error(err)
    showToast('请求异常，请稍后重试')
  }
}

const getSubmitAuditReason = (submitAudit: number) => {
  switch (submitAudit) {
    case 1: return '面向交易'
    case 2: return '面向用户'
    case 3: return '后台配置'
    case 4: return '报表生成'
    case 5: return '其他'
    default: return '未知'
  }
}

onMounted(() => {
  loadAudits()
})

const getAuditStatus = (auditPassed: number, submitAudit: number) => {
  if (submitAudit === 0) return '-'
  if (auditPassed === 1) return '通过'
  if (auditPassed === -1) return '驳回'
  return '待审核'
}

const getAuditStatusClass = (auditPassed: number) => {
  if (auditPassed === 1) return 'status-passed'
  if (auditPassed === -1) return 'status-rejected'
  return 'status-pending'
}

</script>

<template>
  <div class="admin-page">
    <div class="card center-card wide-center-card">
      <h2>SQL 审核管理</h2>

      <div class="module-tip">
        管理员可以在此查看并审核所有用户提交的 SQL 检测记录。
      </div>

      <div class="search-row">
        <select v-model="filterStatus" class="search-select" @change="searchWithReset">
          <option value="">全部状态</option>
          <option value="pending">待审核</option>
          <option value="passed">已通过</option>
          <option value="rejected">已驳回</option>
        </select>
        
        <input 
          type="text" 
          class="search-input" 
          v-model="filterSubmitter" 
          placeholder="提交人 (用户名或昵称)" 
          @keyup.enter="searchWithReset"
        />
        
        <input 
          type="text" 
          class="search-input" 
          v-model="filterConnection" 
          placeholder="数据库连接" 
          @keyup.enter="searchWithReset"
        />

        <button class="action-btn primary-btn small-btn" @click="searchWithReset" :disabled="loading" type="button">
          搜索
        </button>
        <button class="action-btn secondary-btn small-btn" @click="resetSearch" :disabled="loading" type="button">
          重置 / 刷新
        </button>
      </div>

      <div v-if="message" class="status-message">
        {{ message }}
      </div>

      <div v-if="auditList.length > 0" class="history-list-wrapper">
        <div v-for="item in auditList" :key="item.id" class="history-item-card">
          <div class="history-header">
            <div class="history-meta">
              <span class="meta-label">提交人：</span>
              <span class="meta-value">{{ item.displayName || item.username }} ({{ item.username }})</span>
              
              <span class="meta-label margin-left">连接：</span>
              <span class="meta-value">{{ item.connectionName }}</span>
              
              <span class="meta-label margin-left">审核时间：</span>
              <span class="meta-value">{{ item.createTime }}</span>
              
              <span class="meta-label margin-left">审核理由：</span>
              <span class="meta-value">{{ getSubmitAuditReason(item.submitAudit) }}</span>
              
              <span class="meta-label margin-left">状态：</span>
              <span class="status-badge" :class="getAuditStatusClass(item.auditPassed)">{{ getAuditStatus(item.auditPassed, item.submitAudit) }}</span>
            </div>
            <div class="history-score" :class="{ 'score-high': item.aiScore >= 80, 'score-medium': item.aiScore >= 60 && item.aiScore < 80, 'score-low': item.aiScore < 60 }">
              评分：<strong>{{ item.aiScore }}</strong>
            </div>
          </div>
          
          <div class="history-body">
            <div v-if="item.remark" class="history-remark">
              <span class="meta-label">备注：</span> {{ item.remark }}
            </div>
            <div class="history-sql-label">SQL 内容：</div>
            <pre class="history-sql-pre">{{ item.sqlText }}</pre>
            
            <div class="review-actions" v-if="item.auditPassed === 0">
              <button class="action-btn success-btn small-btn" @click="doReviewAudit(item.id, 1)">通过</button>
              <button class="action-btn danger-btn small-btn" @click="doReviewAudit(item.id, -1)">驳回</button>
            </div>
          </div>
          
          <div class="history-ai-section">
            <div class="ai-header">
              <span class="ai-icon">🤖</span> AI 审核建议
            </div>
            <div class="ai-content">
              {{ item.aiSuggestion || '无建议' }}
            </div>
          </div>
        </div>
      </div>

      <!-- Pagination Component -->
      <div v-if="total > 0" class="pagination-container">
        <div class="pagination-info">
          共 {{ total }} 条记录，第 {{ currentPage }} / {{ Math.ceil(total / pageSize) }} 页
        </div>
        <div class="pagination-actions">
          <select v-model="pageSize" @change="handleSizeChange(pageSize)" class="page-size-select">
            <option :value="10">10 条/页</option>
            <option :value="20">20 条/页</option>
            <option :value="50">50 条/页</option>
          </select>
          <button 
            class="action-btn secondary-btn small-btn" 
            :disabled="currentPage <= 1 || loading" 
            @click="handleCurrentChange(currentPage - 1)">
            上一页
          </button>
          <button 
            class="action-btn secondary-btn small-btn" 
            :disabled="currentPage >= Math.ceil(total / pageSize) || loading" 
            @click="handleCurrentChange(currentPage + 1)">
            下一页
          </button>
        </div>
      </div>
    </div>
    
    <!-- Toast 提示框 -->
    <div v-if="toastMessage" class="toast-message">
      {{ toastMessage }}
    </div>
  </div>
</template>

<style scoped>
.toast-message {
  position: fixed;
  top: 40px;
  left: 50%;
  transform: translateX(-50%);
  background: rgba(0, 0, 0, 0.75);
  color: #fff;
  padding: 12px 24px;
  border-radius: 6px;
  z-index: 5000;
  font-size: 15px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  animation: fadeIn 0.3s ease;
}

@keyframes fadeIn {
  from { opacity: 0; top: 20px; }
  to { opacity: 1; top: 40px; }
}

.admin-page {
  width: 100%;
}

.card {
  width: 100%;
  border: 1px solid #ddd;
  border-radius: 10px;
  padding: 24px;
  margin-bottom: 24px;
  background: #fff;
}

.center-card,
.wide-center-card {
  min-height: 600px;
  height: auto;
  display: flex;
  flex-direction: column;
}

h2 {
  margin-top: 0;
  color: #2c3e50;
  flex: 0 0 auto;
}

.module-tip {
  margin-bottom: 12px;
  padding: 12px 14px;
  line-height: 1.8;
  background: #f5f7fa;
  border-radius: 8px;
  color: #666;
  flex: 0 0 auto;
}

.search-row {
  display: flex;
  gap: 12px;
  margin-bottom: 15px;
  align-items: center;
}

.search-input {
  flex: 1;
  padding: 8px 12px;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  font-size: 14px;
  outline: none;
}

.search-input:focus {
  border-color: #409eff;
}

.search-select {
  padding: 8px 12px;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  font-size: 14px;
  outline: none;
  background-color: #fff;
  cursor: pointer;
}

.search-select:focus {
  border-color: #409eff;
}

.action-btn {
  padding: 10px 20px;
  font-size: 16px;
  cursor: pointer;
  border: none;
  border-radius: 6px;
  color: #fff;
  white-space: nowrap;
}

.small-btn {
  padding: 8px 14px;
  font-size: 14px;
}

.primary-btn {
  background: #409eff;
}

.primary-btn:hover {
  background: #66b1ff;
}

.secondary-btn {
  background: #909399;
}

.success-btn {
  background: #67c23a;
}

.success-btn:hover {
  background: #85ce61;
}

.danger-btn {
  background: #f56c6c;
}

.danger-btn:hover {
  background: #f78989;
}

.status-message {
  margin-bottom: 15px;
  padding: 12px 16px;
  border-radius: 6px;
  background-color: #fdf6ec;
  color: #e6a23c;
  font-size: 14px;
  font-weight: 500;
  border-left: 4px solid #e6a23c;
}

.history-list-wrapper {
  display: flex;
  flex-direction: column;
  gap: 20px;
  flex: 1 1 auto;
}

.history-item-card {
  border: 1px solid #ebeef5;
  border-radius: 8px;
  background-color: #fafafa;
  overflow: hidden;
  box-shadow: 0 2px 4px rgba(0,0,0,0.02);
}

.history-header {
  padding: 12px 16px;
  background-color: #f5f7fa;
  border-bottom: 1px solid #ebeef5;
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.history-meta {
  font-size: 14px;
  color: #333;
}

.meta-label {
  color: #909399;
}

.meta-value {
  font-weight: 500;
  color: #303133;
}

.margin-left {
  margin-left: 20px;
}

.history-score {
  font-size: 14px;
  padding: 4px 10px;
  border-radius: 12px;
  font-weight: bold;
  background: #f4f4f5;
  color: #909399;
}

.score-high {
  background: #f0f9eb;
  color: #67c23a;
}

.score-medium {
  background: #fdf6ec;
  color: #e6a23c;
}

.score-low {
  background: #fef0f0;
  color: #f56c6c;
}

.status-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: bold;
}

.status-passed {
  background-color: #f0f9eb;
  color: #67c23a;
  border: 1px solid #e1f3d8;
}

.status-rejected {
  background-color: #fef0f0;
  color: #f56c6c;
  border: 1px solid #fde2e2;
}

.status-pending {
  background-color: #fdf6ec;
  color: #e6a23c;
  border: 1px solid #faecd8;
}

.history-body {
  padding: 16px;
  background: #fff;
  position: relative;
}

.history-remark {
  margin-bottom: 12px;
  font-size: 14px;
  color: #606266;
  background: #fdf6ec;
  padding: 8px 12px;
  border-radius: 4px;
}

.history-sql-label {
  font-size: 13px;
  color: #909399;
  margin-bottom: 8px;
}

.history-sql-pre {
  margin: 0;
  padding: 12px;
  background-color: #f8f9fb;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
  font-family: Consolas, Monaco, "Courier New", monospace;
  font-size: 13px;
  color: #303133;
  white-space: pre-wrap;
  word-break: break-all;
}

.review-actions {
  margin-top: 15px;
  display: flex;
  gap: 10px;
  justify-content: flex-end;
}

.history-ai-section {
  border-top: 1px solid #e4e7ed;
  background-color: #ecf5ff;
}

.ai-header {
  background-color: #d9ecff;
  padding: 10px 16px;
  font-weight: bold;
  color: #409eff;
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
}

.ai-icon {
  font-size: 16px;
}

.ai-content {
  padding: 16px;
  line-height: 1.8;
  color: #303133;
  white-space: pre-wrap; 
  word-break: break-all;
  font-size: 14px;
  background-color: #fff;
}

.pagination-container {
  margin-top: 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-top: 15px;
  border-top: 1px solid #ebeef5;
}

.pagination-info {
  font-size: 14px;
  color: #606266;
}

.pagination-actions {
  display: flex;
  gap: 10px;
  align-items: center;
}

.page-size-select {
  padding: 6px 10px;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  font-size: 14px;
  outline: none;
  background-color: #fff;
  color: #606266;
  cursor: pointer;
}

.page-size-select:focus {
  border-color: #409eff;
}

.action-btn:disabled {
  background-color: #f3f4f6;
  color: #c0c4cc;
  cursor: not-allowed;
  border-color: #ebeef5;
}
</style>