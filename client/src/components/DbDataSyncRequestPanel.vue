<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'

const formatDate = (dateStr: string) => {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return dateStr
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
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
  urgencyLevel: '',
  operateType: '' as number | '',
})

const currentPage = ref(1)
const pageSize = ref(10)
const pageSizeOptions = [10, 20, 50, 100]

const dialogVisible = ref(false)
const dialogLoading = ref(false)
const dialogTitle = ref('新建数据库数据同步申请')
const message = ref('')
const isViewMode = ref(false)

const defaultForm = {
  id: 0,
  applicantTeam: '',
  environment: '',
  expectedFinishTime: '',
  urgencyLevel: '常规',
  urgencyReason: '',
  applyReason: '',
  operateType: 1,
  sourceDb: '',
  targetDbOrPerson: '',
  involvedDbSchemaTable: '',
  dataFilterCondition: '',
  estimatedDataVolume: '',
  containsSensitiveData: 0,
  desensitizationRule: '',
}

const editForm = ref({ ...defaultForm })

// --- 团队环境快速填入辅助逻辑 ---
interface TeamDbEnvItem {
  id: number
  teamName: string
  envName: string
  dbType: string
  testDbIp: string
  testDbName: string
  testDbSchema: string
  prodDbIp: string
  prodDbName: string
  prodDbSchema: string
}
const teamDbEnvs = ref<TeamDbEnvItem[]>([])

const loadTeamDbEnvs = async () => {
  try {
    const res = await fetch('/api/team-db-envs', { credentials: 'include' })
    const data = await res.json()
    if (data.ok) {
      teamDbEnvs.value = data.records || []
    }
  } catch (e) {
    console.error(e)
  }
}

const defaultTeams = ['交易开发', '运营开发', '后台开发', '增长开发']
const dynamicTeams = computed(() => {
  const teams = new Set(defaultTeams)
  teamDbEnvs.value.forEach(env => teams.add(env.teamName))
  return Array.from(teams)
})

const availableEnvs = computed(() => {
  return teamDbEnvs.value.filter(e => e.teamName === editForm.value.applicantTeam)
})

const handleEnvChange = () => {
  const env = teamDbEnvs.value.find(e => e.envName === editForm.value.environment && e.teamName === editForm.value.applicantTeam)
  if (env) {
    const isOracle = env.dbType.toLowerCase() === 'oracle'
    
    if (isOracle) {
      editForm.value.sourceDb = `IP: ${env.prodDbIp || '-'}, DB: ${env.prodDbName || '-'}, Schema: ${env.prodDbSchema || '-'}`
    } else {
      editForm.value.sourceDb = `IP: ${env.prodDbIp || '-'}, DB: ${env.prodDbName || '-'}`
    }

    if (editForm.value.operateType === 2) {
      if (isOracle) {
        editForm.value.targetDbOrPerson = `IP: ${env.testDbIp || '-'}, DB: ${env.testDbName || '-'}, Schema: ${env.testDbSchema || '-'}`
      } else {
        editForm.value.targetDbOrPerson = `IP: ${env.testDbIp || '-'}, DB: ${env.testDbName || '-'}`
      }
    }
  }
}

const handleOperateTypeChange = () => {
  if (editForm.value.operateType === 2 && editForm.value.environment) {
    const env = teamDbEnvs.value.find(e => e.envName === editForm.value.environment && e.teamName === editForm.value.applicantTeam)
    if (env) {
      const isOracle = env.dbType.toLowerCase() === 'oracle'
      if (isOracle) {
        editForm.value.targetDbOrPerson = `IP: ${env.testDbIp || '-'}, DB: ${env.testDbName || '-'}, Schema: ${env.testDbSchema || '-'}`
      } else {
        editForm.value.targetDbOrPerson = `IP: ${env.testDbIp || '-'}, DB: ${env.testDbName || '-'}`
      }
    }
  }
}
// --------------------------------

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
    if (searchForm.value.urgencyLevel) params.append('urgencyLevel', searchForm.value.urgencyLevel)
    if (searchForm.value.operateType) params.append('operateType', String(searchForm.value.operateType))

    const res = await fetch(`/api/db-data-sync-requests?${params.toString()}`, {
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
  searchForm.value = { urgencyLevel: '', operateType: '' }
  currentPage.value = 1
  loadData()
}

const handleSearch = () => {
  currentPage.value = 1
  loadData()
}

const openCreateDialog = () => {
  dialogTitle.value = '新建数据库数据同步申请'
  editForm.value = { ...defaultForm }
  isViewMode.value = false
  dialogVisible.value = true
  message.value = ''
}

const openEditDialog = (row: DBDataSyncRequest) => {
  dialogTitle.value = '编辑数据库数据同步申请'
  isViewMode.value = false
  
  let formattedExpectedTime = row.expectedFinishTime
  if (formattedExpectedTime) {
    const d = new Date(formattedExpectedTime)
    if (!isNaN(d.getTime())) {
      const pad = (n: number) => String(n).padStart(2, '0')
      formattedExpectedTime = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
    }
  }

  editForm.value = { 
    ...defaultForm,
    ...row,
    expectedFinishTime: formattedExpectedTime,
  }
  dialogVisible.value = true
  message.value = ''
}

const openViewDialog = (row: DBDataSyncRequest) => {
  openEditDialog(row)
  dialogTitle.value = '查看数据库数据同步申请'
  isViewMode.value = true
}

const closeDialog = () => {
  dialogVisible.value = false
}

const saveRequest = async () => {
  if (!editForm.value.expectedFinishTime || !editForm.value.urgencyLevel || 
      !editForm.value.applyReason || !editForm.value.operateType || 
      !editForm.value.sourceDb || !editForm.value.targetDbOrPerson || 
      !editForm.value.involvedDbSchemaTable || !editForm.value.estimatedDataVolume) {
    message.value = '请填写所有必填项'
    return
  }

  if (editForm.value.urgencyLevel === '紧急' && !editForm.value.urgencyReason) {
    message.value = '紧急程度为“紧急”时，必须输入原因'
    return
  }

  if (editForm.value.containsSensitiveData === 1 && !editForm.value.desensitizationRule) {
    message.value = '包含敏感信息时，请填写脱敏规则说明'
    return
  }

  // 清理不需要的字段
  if (editForm.value.urgencyLevel !== '紧急') {
    editForm.value.urgencyReason = ''
  }
  if (editForm.value.containsSensitiveData === 0) {
    editForm.value.desensitizationRule = ''
  }

  dialogLoading.value = true
  message.value = ''
  try {
    const isEdit = editForm.value.id > 0
    const method = isEdit ? 'PUT' : 'POST'
    
    const payload = {
      ...editForm.value,
    }

    const res = await fetch('/api/db-data-sync-requests', {
      method,
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(payload),
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

const deleteRequest = async (id: number) => {
  if (!confirm('确定要删除该申请吗？')) return
  
  try {
    const res = await fetch('/api/db-data-sync-requests', {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ id }),
    })
    const data = await res.json()
    if (!res.ok || !data.ok) {
      alert(data.message || '删除失败')
      return
    }
    loadData()
  } catch (err) {
    console.error(err)
    alert('删除失败')
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

onMounted(() => {
  loadData()
  loadTeamDbEnvs()
})
</script>

<template>
  <div class="card panel-card">
    <div class="panel-header">
      <h2>数据库数据同步申请</h2>
      <div class="header-actions">
        <button class="action-btn primary-btn" @click="openCreateDialog" type="button">新建申请</button>
      </div>
    </div>
    
    <div v-if="message && !dialogVisible" class="message-banner">{{ message }}</div>

    <div class="search-bar">
      <div class="search-item">
        <label>操作类型</label>
        <select v-model="searchForm.operateType">
          <option value="">全部</option>
          <option :value="1">迁移到其他数据库</option>
          <option :value="2">迁移到测试库</option>
          <option :value="3">导出为文件</option>
        </select>
      </div>
      <div class="search-item">
        <label>紧急程度</label>
        <select v-model="searchForm.urgencyLevel">
          <option value="">全部</option>
          <option value="常规">常规</option>
          <option value="重要">重要</option>
          <option value="紧急">紧急</option>
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
            <td colspan="10" class="empty-cell">暂无数据</td>
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
            <td>{{ item.executeDba || '-' }}</td>
            <td>
              <div class="row-actions" v-if="!item.executeDba">
                <button class="text-btn" @click="openEditDialog(item)" type="button">编辑</button>
                <button class="text-btn danger-text" @click="deleteRequest(item.id)" type="button">删除</button>
              </div>
              <div class="row-actions" v-else>
                <button class="text-btn" @click="openViewDialog(item)" type="button">查看</button>
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
      <div class="dialog-content large-dialog">
        <div class="dialog-header">
          <h3>{{ dialogTitle }}</h3>
          <button class="close-btn" @click="closeDialog" type="button">×</button>
        </div>
        <div class="dialog-body">
          
          <fieldset :disabled="isViewMode" style="border: none; padding: 0; margin: 0; min-width: 0;">

          <div class="form-grid">
            <div class="form-item">
              <label>申请团队 <span v-if="!isViewMode" class="required">*</span></label>
              <select v-model="editForm.applicantTeam" @change="editForm.environment = ''">
                <option value="">请选择申请团队</option>
                <option v-for="team in dynamicTeams" :key="team" :value="team">{{ team }}</option>
              </select>
            </div>
            <div class="form-item">
              <label>可用环境 (选填，选择后自动填入下方信息)</label>
              <select v-model="editForm.environment" @change="handleEnvChange" :disabled="!editForm.applicantTeam">
                <option value="">请选择环境</option>
                <option v-for="env in availableEnvs" :key="env.id" :value="env.envName">{{ env.envName }}</option>
              </select>
            </div>

            <div class="form-item">
              <label>期望完成时间 <span v-if="!isViewMode" class="required">*</span></label>
              <input v-model="editForm.expectedFinishTime" type="datetime-local" />
            </div>
            <div class="form-item">
              <label>紧急程度 <span v-if="!isViewMode" class="required">*</span></label>
              <select v-model="editForm.urgencyLevel">
                <option value="常规">常规</option>
                <option value="重要">重要</option>
                <option value="紧急">紧急</option>
              </select>
            </div>
            
            <div class="form-item full-width" v-if="editForm.urgencyLevel === '紧急'">
              <label>紧急原因说明 <span v-if="!isViewMode" class="required">*</span></label>
              <input v-model="editForm.urgencyReason" type="text" placeholder="请填写紧急申请的原因" />
            </div>

            <div class="form-item full-width">
              <label>操作类型 <span v-if="!isViewMode" class="required">*</span></label>
              <div class="radio-group">
                <label><input type="radio" :value="1" v-model="editForm.operateType" @change="handleOperateTypeChange" /> 迁移到其他数据库</label>
                <label><input type="radio" :value="2" v-model="editForm.operateType" @change="handleOperateTypeChange" /> 迁移到测试库</label>
                <label><input type="radio" :value="3" v-model="editForm.operateType" @change="handleOperateTypeChange" /> 导出为文件</label>
              </div>
            </div>

            <div class="form-item">
              <label>源数据库 <span v-if="!isViewMode" class="required">*</span></label>
              <input v-model="editForm.sourceDb" type="text" placeholder="如：交易主库 Trade_DB (10.x.x.x)" />
            </div>
            
            <div class="form-item">
              <label>目标库/目标人 <span v-if="!isViewMode" class="required">*</span></label>
              <input v-model="editForm.targetDbOrPerson" type="text" placeholder="如：交易测试库 Trade_Test_DB (192.x.x.x) / 或接收文件的人员" />
            </div>

            <div class="form-item full-width">
              <label>涉及库名/Schema名/表名 <span v-if="!isViewMode" class="required">*</span></label>
              <textarea v-model="editForm.involvedDbSchemaTable" rows="2" placeholder="请详细列出涉及的库/Schema/表"></textarea>
            </div>

            <div class="form-item full-width">
              <label>申请原因与目的 <span v-if="!isViewMode" class="required">*</span></label>
              <textarea v-model="editForm.applyReason" rows="2" placeholder="(例如：为了复现线上 #12345 订单偶现的计算异常 Bug，需要同步该订单相关的流水数据到测试环境进行 Debug；或者：为了进行大促前的全链路压测，需要拉取近 3 天的真实订单量构造压测模型。)"></textarea>
            </div>

            <div class="form-item full-width">
              <label>数据过滤条件</label>
              <textarea v-model="editForm.dataFilterCondition" rows="2" placeholder="可选，如有需要请填写过滤条件(如 WHERE 语句等)"></textarea>
            </div>

            <div class="form-item">
              <label>预估数据量 <span v-if="!isViewMode" class="required">*</span></label>
              <input v-model="editForm.estimatedDataVolume" type="text" placeholder="如：约 1 万条 / 约 2GB" />
            </div>

            <div class="form-item">
              <label>是否包含敏感信息 <span v-if="!isViewMode" class="required">*</span></label>
              <select v-model="editForm.containsSensitiveData">
                <option :value="0">否</option>
                <option :value="1">是</option>
              </select>
            </div>

            <div class="form-item full-width" v-if="editForm.containsSensitiveData === 1">
              <label>脱敏规则说明 <span v-if="!isViewMode" class="required">*</span></label>
              <textarea v-model="editForm.desensitizationRule" rows="2" placeholder="请详细说明哪些字段需要脱敏，以及脱敏规则..."></textarea>
            </div>
          </div>
          </fieldset>
        </div>
        <div class="dialog-footer">
          <span v-if="message" class="error-message">{{ message }}</span>
          <button class="action-btn plain-btn" @click="closeDialog" :disabled="dialogLoading" type="button">{{ isViewMode ? '关闭' : '取消' }}</button>
          <button v-if="!isViewMode" class="action-btn primary-btn" @click="saveRequest" :disabled="dialogLoading" type="button">
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
  font-family: Consolas, Monaco, monospace;
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
  padding: 4px 4px;
  font-size: 13px;
}
.row-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-wrap: nowrap;
  white-space: nowrap;
}
.danger-text {
  color: #f56c6c;
}
.table-wrap {
  width: 100%;
  min-height: 520px;
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
.data-table th:nth-child(7),
.data-table td:nth-child(7),
.data-table th:nth-child(8),
.data-table td:nth-child(8) {
  word-break: break-all;
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
.tag {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
}
.tag-info { background: #e9e9eb; color: #909399; }
.tag-danger { background: #fef0f0; color: #f56c6c; border: 1px solid #fde2e2; }
.tag-success { background: #f0f9eb; color: #67c23a; border: 1px solid #e1f3d8; }
.tag-warning { background: #fdf6ec; color: #e6a23c; border: 1px solid #faecd8; }

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
  padding-top: 5vh;
  z-index: 2000;
  overflow-y: auto;
}
.dialog-content {
  background: #fff;
  border-radius: 8px;
  width: 90%;
  max-width: 600px;
  box-shadow: 0 4px 16px rgba(0,0,0,0.1);
  margin-bottom: 5vh;
}
.large-dialog {
  max-width: 800px;
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
.required {
  color: #f56c6c;
}
.form-item input[type="text"],
.form-item input[type="datetime-local"],
.form-item select,
.form-item textarea {
  padding: 8px;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  font-size: 14px;
  font-family: Consolas, Monaco, monospace;
  width: 100%;
}
.checkbox-group, .radio-group {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
  padding: 6px 0;
}
.checkbox-group label, .radio-group label {
  display: flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
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
</style>
