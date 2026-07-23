<script setup lang="ts">
/**
 * DbDataSyncRequestPanel.vue
 * ------------------------------------------------------------------
 * 该组件是「数据库数据同步申请」页面（申请人视角）。
 *
 * 布局模式：list 列表 ↔ view 查看 ↔ form 新建/编辑。
 *
 * 主要功能：
 * 1. 分页展示自己提交的数据同步申请，按操作类型 / 紧急程度筛选（下拉自动响应）。
 * 2. 新建 / 编辑 / 删除申请；未由 DBA 登记执行的记录才可编辑/删除。
 * 3. 操作类型单选（迁移到其他库 / 迁移到测试库 / 导出为文件），选「迁移到测试库」时按环境自动填入目标库。
 * 4. 紧急程度为「紧急」时必填紧急原因；包含敏感信息时必填脱敏规则说明。
 * 5. 选择「团队 + 环境」后自动填入源库 / 目标库连接信息。
 *
 * 关键接口：
 * - GET/POST/PUT/DELETE /api/db-data-sync-requests
 * - GET /api/team-db-envs  团队环境配置（用于自动填入连接信息）
 */

import { ref, onMounted, computed } from 'vue'
import { showToast, showConfirm } from '../utils/toast'

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
  urgencyLevel: '',
  operateType: '' as number | '',
})

const currentPage = ref(1)
const pageSize = ref(10)
const pageSizeOptions = [10, 20, 50, 100]

const viewMode = ref<'list' | 'view' | 'form'>('list')
const isEditMode = ref(false)
const loading = ref(false)
const message = ref('')

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

// 选择团队+环境后，按数据库类型自动填入源库连接信息；
// 若操作类型为「迁移到测试库(2)」，同时填入目标测试库连接信息。
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

// 操作类型切换为「迁移到测试库(2)」时，若已选环境，补填目标测试库连接信息
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

const showCreateForm = () => {
  editForm.value = { ...defaultForm }
  isEditMode.value = false
  message.value = ''
  viewMode.value = 'form'
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

const startEdit = (row: DBDataSyncRequest) => {
  isEditMode.value = true
  let formattedExpectedTime = row.expectedFinishTime
  if (formattedExpectedTime) {
    const d = new Date(formattedExpectedTime)
    if (!isNaN(d.getTime())) {
      const pad = (n: number) => String(n).padStart(2, '0')
      formattedExpectedTime = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
    }
  }
  editForm.value = { ...defaultForm, ...row, expectedFinishTime: formattedExpectedTime }
  message.value = ''
  viewMode.value = 'form'
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

const startView = (row: DBDataSyncRequest) => {
  startEdit(row)
  viewMode.value = 'view'
}

const backToList = () => {
  viewMode.value = 'list'
  message.value = ''
  loadData()
}

const saveRequest = async () => {
  if (!editForm.value.expectedFinishTime || !editForm.value.urgencyLevel ||
      !editForm.value.applyReason || !editForm.value.operateType ||
      !editForm.value.sourceDb || !editForm.value.targetDbOrPerson ||
      !editForm.value.involvedDbSchemaTable || !editForm.value.estimatedDataVolume) {
    message.value = '❌ 请填写所有必填项'
    return
  }
  if (editForm.value.urgencyLevel === '紧急' && !editForm.value.urgencyReason) {
    message.value = '❌ 紧急程度为"紧急"时，必须输入原因'
    return
  }
  if (editForm.value.containsSensitiveData === 1 && !editForm.value.desensitizationRule) {
    message.value = '❌ 包含敏感信息时，请填写脱敏规则说明'
    return
  }
  if (editForm.value.urgencyLevel !== '紧急') {
    editForm.value.urgencyReason = ''
  }
  if (editForm.value.containsSensitiveData === 0) {
    editForm.value.desensitizationRule = ''
  }

  loading.value = true
  message.value = ''
  try {
    const isEdit = editForm.value.id > 0
    const method = isEdit ? 'PUT' : 'POST'
    const res = await fetch('/api/db-data-sync-requests', {
      method,
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(editForm.value),
    })
    const data = await res.json()
    if (!res.ok || !data.ok) {
      message.value = data.message || '保存失败'
      return
    }
    showToast(data.message || (isEdit ? '更新成功' : '创建成功'), 'success')
    backToList()
  } catch (err) {
    console.error(err)
    message.value = '保存失败，请检查网络或后端状态'
  } finally {
    loading.value = false
  }
}

const deleteRequest = async (id: number) => {
  const ok = await showConfirm('确定要删除该申请吗？')
  if (!ok) return
  try {
    const res = await fetch('/api/db-data-sync-requests', {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ id }),
    })
    const data = await res.json()
    if (!res.ok || !data.ok) {
      showToast(data.message || '删除失败', 'error')
      return
    }
    showToast('删除成功', 'success')
    loadData()
  } catch (err) {
    console.error(err)
    showToast('删除失败', 'error')
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
    <!-- ============ 列表视图 ============ -->
    <template v-if="viewMode === 'list'">
      <div class="panel-header">
        <h2>数据库数据同步申请</h2>
        <button class="action-btn primary-btn" @click="showCreateForm" type="button">+ 新建申请</button>
      </div>

      <div v-if="message" class="message-banner">{{ message }}</div>

      <div class="search-bar">
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
              <td>{{ item.executeDba || '-' }}</td>
              <td>
                <div class="row-actions" v-if="!item.executeDba">
                  <button class="text-btn" @click="startEdit(item)" type="button">编辑</button>
                  <button class="text-btn danger-text" @click="deleteRequest(item.id)" type="button">删除</button>
                </div>
                <div class="row-actions" v-else>
                  <button class="text-btn" @click="startView(item)" type="button">查看</button>
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

    <!-- ============ 查看 / 登记 / 编辑视图 ============ -->
    <template v-else>
      <div class="form-header">
        <h2 v-if="viewMode === 'view'">查看数据库数据同步申请</h2>
        <h2 v-else-if="isEditMode">编辑数据库数据同步申请</h2>
        <h2 v-else>新建数据库数据同步申请</h2>
        <button class="action-btn secondary-btn back-btn" @click="backToList" type="button">← 返回列表</button>
      </div>
      <p v-if="message" class="inline-message">{{ message }}</p>

      <fieldset :disabled="viewMode === 'view'" style="border: none; padding: 0; margin: 0;">
      <div class="form-grid">
        <div class="form-item">
          <label>申请团队 <span v-if="viewMode !== 'view'" class="required">*</span></label>
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
          <label>期望完成时间 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <input v-model="editForm.expectedFinishTime" type="datetime-local" />
        </div>
        <div class="form-item">
          <label>紧急程度 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <select v-model="editForm.urgencyLevel">
            <option value="常规">常规</option>
            <option value="重要">重要</option>
            <option value="紧急">紧急</option>
          </select>
        </div>

        <div class="form-item full-width" v-if="editForm.urgencyLevel === '紧急'">
          <label>紧急原因说明 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <input v-model="editForm.urgencyReason" type="text" placeholder="请填写紧急申请的原因" />
        </div>

        <div class="form-item full-width">
          <label>操作类型 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <div class="radio-group">
            <label><input type="radio" :value="1" v-model="editForm.operateType" @change="handleOperateTypeChange" /> 迁移到其他数据库</label>
            <label><input type="radio" :value="2" v-model="editForm.operateType" @change="handleOperateTypeChange" /> 迁移到测试库</label>
            <label><input type="radio" :value="3" v-model="editForm.operateType" @change="handleOperateTypeChange" /> 导出为文件</label>
          </div>
        </div>

        <div class="form-item">
          <label>源数据库 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <input v-model="editForm.sourceDb" type="text" placeholder="如：交易主库 Trade_DB (10.x.x.x)" />
        </div>
        <div class="form-item">
          <label>目标库/目标人 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <input v-model="editForm.targetDbOrPerson" type="text" placeholder="如：交易测试库 Trade_Test_DB / 或接收文件的人员" />
        </div>

        <div class="form-item full-width">
          <label>涉及库名/Schema名/表名 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <textarea v-model="editForm.involvedDbSchemaTable" rows="2" placeholder="请详细列出涉及的库/Schema/表"></textarea>
        </div>

        <div class="form-item full-width">
          <label>申请原因与目的 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <textarea v-model="editForm.applyReason" rows="2" placeholder="请填写申请原因与目的"></textarea>
        </div>

        <div class="form-item full-width">
          <label>数据过滤条件</label>
          <textarea v-model="editForm.dataFilterCondition" rows="2" placeholder="可选，如有需要请填写过滤条件"></textarea>
        </div>

        <div class="form-item">
          <label>预估数据量 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <input v-model="editForm.estimatedDataVolume" type="text" placeholder="如：约 1 万条 / 约 2GB" />
        </div>
        <div class="form-item">
          <label>是否包含敏感信息 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <select v-model="editForm.containsSensitiveData">
            <option :value="0">否</option>
            <option :value="1">是</option>
          </select>
        </div>

        <div class="form-item full-width" v-if="editForm.containsSensitiveData === 1">
          <label>脱敏规则说明 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <textarea v-model="editForm.desensitizationRule" rows="2" placeholder="请详细说明哪些字段需要脱敏"></textarea>
        </div>
      </div>
      </fieldset>

      <div class="btn-row">
        <template v-if="viewMode === 'view'">
          <button class="action-btn warning-btn" @click="backToList" type="button">返回列表</button>
        </template>
        <template v-else>
          <button class="action-btn primary-btn" :disabled="loading" @click="saveRequest" type="button">
            {{ loading ? '保存中...' : (isEditMode ? '保存修改' : '创建申请') }}
          </button>
          <button class="action-btn warning-btn" :disabled="loading" @click="backToList" type="button">取消</button>
        </template>
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
.text-btn { background: transparent; border: none; color: #409eff; cursor: pointer; padding: 4px 4px; font-size: 13px; }
.danger-text { color: #f56c6c; }

/* 行操作 */
.row-actions { display: flex; align-items: center; gap: 2px; flex-wrap: nowrap; white-space: nowrap; }

/* 表格 */
.table-wrap { min-height: 520px; border: 1px solid #ebeef5; border-radius: 4px; margin-bottom: 20px; }
.table-wrap.loading { opacity: 0.6; pointer-events: none; }

/* 分页（两栏布局） */
.pagination { display: flex; justify-content: space-between; align-items: center; font-size: 14px; color: #606266; }
.page-nav { display: flex; align-items: center; gap: 12px; }

/* 表单头部 */
.form-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.back-btn { white-space: nowrap; }
.inline-message { color: #f56c6c; font-size: 14px; margin-bottom: 12px; }

/* 按钮行 */
.btn-row { display: flex; gap: 12px; flex-wrap: wrap; align-items: center; margin-top: 16px; }

/* 表单 */
.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-bottom: 16px; }
.form-item { display: flex; flex-direction: column; gap: 6px; }
.form-item.full-width { grid-column: span 2; }
.form-item label { font-size: 14px; color: #606266; }

/* 单选框组 */
.radio-group { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; padding: 6px 0; }
.radio-group label { display: flex; align-items: center; gap: 4px; cursor: pointer; }

.message-banner { background: #fdf6ec; color: #e6a23c; padding: 10px 16px; border-radius: 4px; margin-bottom: 16px; font-size: 14px; }
</style>
