<script setup lang="ts">
import { onMounted, ref } from 'vue'

interface SqlAuditHistoryRecord {
  id: number
  connectionName: string
  sqlText: string
  executionPlan: string
  aiSuggestion: string
  aiScore: number
  submitAudit: number
  auditPassed: number
  reviewer: string
  remark: string
  createTime: string
}

const loading = ref(false)
const historyList = ref<SqlAuditHistoryRecord[]>([])
const message = ref('')
const searchSql = ref('')

const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)

import { computed } from 'vue'

const totalPages = computed(() => {
  return Math.ceil(total.value / pageSize.value) || 1
})

const handleSizeChange = (val: number) => {
  pageSize.value = val
  currentPage.value = 1
  loadHistory()
}

const handleCurrentChange = (val: number) => {
  currentPage.value = val
  loadHistory()
}

const searchWithReset = () => {
  currentPage.value = 1
  loadHistory()
}

const resetSearch = () => {
  searchSql.value = ''
  currentPage.value = 1
  loadHistory()
}

const loadHistory = async () => {
  loading.value = true
  message.value = '加载中...'
  try {
    const params = new URLSearchParams()
    if (searchSql.value.trim() !== '') {
      params.append('sql', searchSql.value.trim())
    }
    params.append('page', currentPage.value.toString())
    params.append('pageSize', pageSize.value.toString())

    const url = `/api/audit-history?${params.toString()}`

    const res = await fetch(url, {
      credentials: 'include',
    })
    
    if (res.status === 401) {
      message.value = '登录已失效，请重新登录'
      historyList.value = []
      total.value = 0
      return
    }
    
    const data = await res.json()
    historyList.value = data.history || []
    total.value = data.total || 0
    if (historyList.value.length === 0) {
      message.value = '暂无审核历史记录'
    } else {
      message.value = ''
    }
  } catch (err) {
    console.error(err)
    message.value = '获取审核历史失败'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadHistory()
})

const getSubmitStatus = (submitAudit: number) => {
  if (submitAudit > 0) return '已提交审核'
  return '未提交'
}

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

const getSubmitStatusClass = (submitAudit: number) => {
  if (submitAudit > 0) return 'status-submitted'
  return 'status-unsubmitted'
}
</script>

<template>
  <div class="admin-page">
    <div class="card center-card wide-center-card">
      <h2>SQL AI 审核历史</h2>

      <div class="module-tip">
        展示您过去提交进行 AI 性能检测的全部 SQL 记录及其评分与建议。
      </div>

      <div class="search-row">
        <input 
          type="text" 
          class="search-input" 
          v-model="searchSql" 
          placeholder="输入要搜索的 SQL 进行指纹匹配..." 
          @keyup.enter="searchWithReset"
        />
        <button class="action-btn primary-btn small-btn" @click="searchWithReset" :disabled="loading" type="button">
          搜索
        </button>
        <button class="action-btn secondary-btn small-btn" @click="resetSearch" :disabled="loading" type="button">
          刷新 / 重置
        </button>
      </div>

      <div v-if="message" class="status-message">
        {{ message }}
      </div>

      <div v-if="historyList.length > 0" class="history-list-wrapper">
        <div v-for="item in historyList" :key="item.id" class="history-item-card">
          <div class="history-header">
            <div class="history-meta">
              <span class="meta-label">数据库连接：</span>
              <span class="meta-value">{{ item.connectionName }}</span>
              <span class="meta-label margin-left">审核时间：</span>
              <span class="meta-value">{{ item.createTime }}</span>
              
              <span class="meta-label margin-left">提交状态：</span>
              <span class="status-badge" :class="getSubmitStatusClass(item.submitAudit)">{{ getSubmitStatus(item.submitAudit) }}</span>
              
              <span class="meta-label margin-left" v-if="item.submitAudit > 0">审核结果：</span>
              <span class="status-badge" :class="getAuditStatusClass(item.auditPassed)" v-if="item.submitAudit > 0">{{ getAuditStatus(item.auditPassed, item.submitAudit) }}</span>

              <span class="meta-label margin-left" v-if="item.auditPassed !== 0">审核人员：</span>
              <span class="meta-value" v-if="item.auditPassed !== 0">{{ item.reviewer || '-' }}</span>
            </div>
            <div class="history-score" :class="{ 'score-high': item.aiScore >= 80, 'score-medium': item.aiScore >= 60 && item.aiScore < 80, 'score-low': item.aiScore < 60 }">
              评分：<strong>{{ item.aiScore }}</strong>
            </div>
          </div>
          
          <div class="history-body">
            <div class="history-sql-label">SQL 内容：</div>
            <pre class="history-sql-pre">{{ item.sqlText }}</pre>

            <div v-if="item.executionPlan" class="history-sql-label" style="margin-top: 12px;">执行计划：</div>
            <pre v-if="item.executionPlan" class="history-sql-pre">{{ item.executionPlan }}</pre>
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
  </div>
</template>

<style scoped>
/* 组件特有样式（公共样式见 global.css） */
.admin-page { width: 100%; }

/* 卡片布局 */
.center-card, .wide-center-card { min-height: 600px; height: auto; display: flex; flex-direction: column; }
h2 { flex: 0 0 auto; }
.module-tip { margin-bottom: 12px; padding: 12px 14px; line-height: 1.8; background: #f5f7fa; border-radius: 8px; color: #666; flex: 0 0 auto; }

/* 搜索栏 */
.search-row { display: flex; gap: 12px; margin-bottom: 15px; align-items: center; }
.search-input { flex: 1; }

/* 按钮特有 */
.action-btn { white-space: nowrap; }
.primary-btn:hover { background: #66b1ff; }

/* 状态消息 */
.status-message { margin-bottom: 15px; padding: 12px 16px; border-radius: 6px; background: #fdf6ec; color: #e6a23c; font-size: 14px; font-weight: 500; border-left: 4px solid #e6a23c; }

/* 历史记录卡片 */
.history-list-wrapper { display: flex; flex-direction: column; gap: 20px; flex: 1 1 auto; }
.history-item-card { border: 1px solid #ebeef5; border-radius: 8px; background: #fafafa; overflow: hidden; box-shadow: 0 2px 4px rgba(0,0,0,0.02); }
.history-header { padding: 12px 16px; background: #f5f7fa; border-bottom: 1px solid #ebeef5; display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 10px; }
.history-meta { font-size: 14px; color: #333; }
.meta-label { color: #909399; }
.meta-value { font-weight: 500; color: #303133; }
.margin-left { margin-left: 20px; }
.history-score { font-size: 14px; padding: 4px 10px; border-radius: 12px; font-weight: bold; background: #f4f4f5; color: #909399; }
.score-high { background: #f0f9eb; color: #67c23a; }
.score-medium { background: #fdf6ec; color: #e6a23c; }
.score-low { background: #fef0f0; color: #f56c6c; }

/* 状态徽章 */
.status-badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 12px; font-weight: bold; }
.status-submitted { background: #e1f3d8; color: #67c23a; border: 1px solid #c2e7b0; }
.status-unsubmitted { background: #f4f4f5; color: #909399; border: 1px solid #d3d4d6; }
.status-passed { background: #f0f9eb; color: #67c23a; border: 1px solid #e1f3d8; }
.status-rejected { background: #fef0f0; color: #f56c6c; border: 1px solid #fde2e2; }
.status-pending { background: #fdf6ec; color: #e6a23c; border: 1px solid #faecd8; }

/* 记录内容 */
.history-body { padding: 16px; background: #fff; }
.history-sql-label { font-size: 13px; color: #909399; margin-bottom: 8px; }
.history-sql-pre { margin: 0; padding: 12px; background: #f8f9fb; border: 1px solid #e4e7ed; border-radius: 4px; font-family: Consolas, Monaco, "Courier New", monospace; font-size: 13px; color: #303133; white-space: pre-wrap; word-break: break-all; }

/* AI 区域 */
.history-ai-section { border-top: 1px solid #e4e7ed; background: #ecf5ff; }
.ai-header { background: #d9ecff; padding: 10px 16px; font-weight: bold; color: #409eff; display: flex; align-items: center; gap: 8px; font-size: 14px; }
.ai-icon { font-size: 16px; }
.ai-content { padding: 16px; line-height: 1.8; color: #303133; white-space: pre-wrap; word-break: break-all; font-size: 14px; background: #fff; }

/* 分页（两栏布局） */
.pagination-container { margin-top: 20px; display: flex; justify-content: space-between; align-items: center; padding-top: 15px; border-top: 1px solid #ebeef5; }
.pagination-info { font-size: 14px; color: #606266; }
.pagination-actions { display: flex; gap: 10px; align-items: center; }
.page-size-select { padding: 6px 10px; background: #fff; color: #606266; cursor: pointer; }
.page-size-select:focus { border-color: #409eff; }
.action-btn:disabled { background: #f3f4f6; color: #c0c4cc; cursor: not-allowed; border-color: #ebeef5; }
</style>
