<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { showToast, showConfirm } from '../utils/toast'

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
  otherChangeTypeReason: string
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
const selectedEnvId = ref<number | ''>('')

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
  const env = teamDbEnvs.value.find(e => e.id === selectedEnvId.value)
  if (env) {
    let mappedDbType = env.dbType
    if (mappedDbType.toLowerCase() === 'mysql') mappedDbType = 'MySQL'
    else if (mappedDbType.toLowerCase() === 'oracle') mappedDbType = 'Oracle'
    else if (mappedDbType.toLowerCase() === 'redis') mappedDbType = 'redis'

    const standardOptions = ['Oracle', 'MySQL', 'redis']
    if (!standardOptions.includes(mappedDbType)) {
      editForm.value.dbType = ['其他']
      editForm.value.otherDbTypeReason = mappedDbType
    } else {
      editForm.value.dbType = [mappedDbType]
      editForm.value.otherDbTypeReason = ''
    }

    editForm.value.environment = env.envName
    editForm.value.testDbIp = env.testDbIp || ''
    editForm.value.testDbName = env.testDbName || ''
    editForm.value.testDbSchema = env.testDbSchema || ''
    editForm.value.dbIp = env.prodDbIp || ''
    editForm.value.dbName = env.prodDbName || ''
    editForm.value.dbSchema = env.prodDbSchema || ''
  }
}

const searchForm = ref({
  applicantTeam: '',
  urgencyLevel: '',
  dbType: '',
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
  plannedChangeTime: '',
  urgencyLevel: '常规',
  changeType: [] as string[],
  otherChangeTypeReason: '',
  changeReason: '',
  requirementUrl: '',
  impactScope: '',
  dbType: [] as string[],
  otherDbTypeReason: '',
  testDbIp: '',
  testDbName: '',
  testDbSchema: '',
  dbIp: '',
  dbName: '',
  dbSchema: '',
  changeContent: '',
  backupTable: '',
}

const editForm = ref({ ...defaultForm })

const totalPages = computed(() => {
  if (totalCount.value === 0) return 1
  return Math.ceil(totalCount.value / pageSize.value)
})

const isReleased = (item: DBChangeRequest) => {
  return !!(item.testPublisher && item.prodPublisher)
}

const isVerified = (item: DBChangeRequest) => {
  return isReleased(item) && !!item.releaseVerifier
}

const isDataChange = computed(() => editForm.value.changeType.includes('数据修改'))

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

    const res = await fetch(`/api/db-change-requests?${params.toString()}`, {
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
  searchForm.value = { applicantTeam: '', urgencyLevel: '', dbType: '' }
  currentPage.value = 1
  loadData()
}

const handleSearch = () => {
  currentPage.value = 1
  loadData()
}

const showCreateForm = () => {
  editForm.value = { ...defaultForm, changeType: [], dbType: [], otherChangeTypeReason: '', otherDbTypeReason: '' }
  selectedEnvId.value = ''
  isEditMode.value = false
  message.value = ''
  viewMode.value = 'form'
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

const fillFormFromRow = (row: DBChangeRequest) => {
  selectedEnvId.value = ''

  let changeTypeArr: string[] = []
  let otherReason = ''
  if (row.changeType) {
    const parts = row.changeType.split(',')
    for (const part of parts) {
      if (part.startsWith('其他(') && part.endsWith(')')) {
        changeTypeArr.push('其他')
        otherReason = part.substring(3, part.length - 1)
      } else if (part === '其他') {
        changeTypeArr.push('其他')
      } else {
        changeTypeArr.push(part)
      }
    }
  }

  let dbTypeArr: string[] = []
  let otherDbReason = ''
  if (row.dbType) {
    const parts = row.dbType.split(',').map(p => {
      if (p === 'oracle') return 'Oracle'
      if (p === 'mysql') return 'MySQL'
      return p
    })
    for (const part of parts) {
      if (part.startsWith('其他(') && part.endsWith(')')) {
        dbTypeArr.push('其他')
        otherDbReason = part.substring(3, part.length - 1)
      } else if (part === '其他') {
        dbTypeArr.push('其他')
      } else {
        dbTypeArr.push(part)
      }
    }
  }

  let formattedPlannedTime = row.plannedChangeTime
  if (formattedPlannedTime) {
    const d = new Date(formattedPlannedTime)
    if (!isNaN(d.getTime())) {
      const pad = (n: number) => String(n).padStart(2, '0')
      formattedPlannedTime = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
    }
  }

  editForm.value = {
    ...defaultForm,
    ...row,
    plannedChangeTime: formattedPlannedTime,
    changeType: changeTypeArr,
    otherChangeTypeReason: otherReason,
    dbType: dbTypeArr,
    otherDbTypeReason: otherDbReason
  }
}

const startEdit = (row: DBChangeRequest) => {
  fillFormFromRow(row)
  isEditMode.value = true
  message.value = ''
  viewMode.value = 'form'
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

const startView = (row: DBChangeRequest) => {
  fillFormFromRow(row)
  isEditMode.value = true
  message.value = ''
  viewMode.value = 'view'
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

const startClone = (row: DBChangeRequest) => {
  fillFormFromRow(row)
  isEditMode.value = false
  editForm.value.id = 0
  editForm.value.changeType = []
  editForm.value.otherChangeTypeReason = ''
  editForm.value.changeContent = ''
  message.value = '正在克隆配置，请修改后点击"创建申请"'
  viewMode.value = 'form'
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

const backToList = () => {
  viewMode.value = 'list'
  message.value = ''
  loadData()
}

const saveRequest = async () => {
  if (!editForm.value.applicantTeam || !editForm.value.plannedChangeTime || !editForm.value.urgencyLevel || editForm.value.changeType.length === 0 ||
      !editForm.value.changeReason || editForm.value.dbType.length === 0 || !editForm.value.dbIp || !editForm.value.testDbIp ||
      !editForm.value.changeContent) {
    message.value = '❌ 请填写所有必填项'
    return
  }

  if (isDataChange.value && !editForm.value.backupTable) {
    message.value = '❌ 变更类型包含「数据修改」时，备份表为必填项'
    return
  }

  const getArr = (val: any) => {
    if (!val) return []
    if (Array.isArray(val)) return val
    if (typeof val === 'string') return val.split(',')
    return []
  }

  const dbTypeArr = getArr(editForm.value.dbType)
  const changeTypeArr = getArr(editForm.value.changeType)

  const hasOracle = dbTypeArr.includes('Oracle')
  const hasMySQL = dbTypeArr.includes('MySQL')
  const hasOtherDB = dbTypeArr.includes('其他')

  if ((hasOracle || hasMySQL || hasOtherDB) && (!editForm.value.dbName || !editForm.value.testDbName)) {
    message.value = '❌ 请填写测试线及生产线数据库实例/数据库名'
    return
  }
  if (hasOracle && (!editForm.value.dbSchema || !editForm.value.testDbSchema)) {
    message.value = '❌ 请填写测试线及生产线数据库 Schema'
    return
  }
  if (changeTypeArr.includes('其他') && !editForm.value.otherChangeTypeReason) {
    message.value = '❌ 请填写其他变更类型理由'
    return
  }
  if (hasOtherDB && !editForm.value.otherDbTypeReason) {
    message.value = '❌ 请填写其他数据库类型'
    return
  }

  if (!hasOracle && !hasMySQL && !hasOtherDB) {
    editForm.value.dbName = ''
    editForm.value.testDbName = ''
  }
  if (!hasOracle) {
    editForm.value.dbSchema = ''
    editForm.value.testDbSchema = ''
  }
  if (!changeTypeArr.includes('其他')) {
    editForm.value.otherChangeTypeReason = ''
  }
  if (!hasOtherDB) {
    editForm.value.otherDbTypeReason = ''
  }

  loading.value = true
  message.value = ''
  try {
    const isEdit = editForm.value.id > 0
    const method = isEdit ? 'PUT' : 'POST'

    let finalChangeType = editForm.value.changeType.map(item =>
      item === '其他' && editForm.value.otherChangeTypeReason ? `其他(${editForm.value.otherChangeTypeReason})` : item
    ).join(',')

    let finalDbType = editForm.value.dbType.map(item =>
      item === '其他' && editForm.value.otherDbTypeReason ? `其他(${editForm.value.otherDbTypeReason})` : item
    ).join(',')

    const payload = {
      ...editForm.value,
      changeType: finalChangeType,
      dbType: finalDbType,
    }

    const res = await fetch('/api/db-change-requests', {
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
    const res = await fetch('/api/db-change-requests', {
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

const verifyRequest = async (id: number) => {
  const ok = await showConfirm('确认该变更申请的发布结果正常？\n验证后该申请将彻底完结，不可再修改。')
  if (!ok) return
  try {
    const res = await fetch('/api/db-change-requests/verify', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ id }),
    })
    const data = await res.json()
    if (!res.ok || !data.ok) {
      showToast(data.message || '验证失败', 'error')
      return
    }
    showToast('验证成功', 'success')
    loadData()
  } catch (err) {
    console.error(err)
    showToast('验证失败', 'error')
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

const handleFileUpload = (event: Event) => {
  const target = event.target as HTMLInputElement
  if (target.files && target.files.length > 0) {
    const file = target.files[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = (e) => {
      if (e.target && typeof e.target.result === 'string') {
        editForm.value.changeContent = e.target.result
      }
    }
    reader.readAsText(file)
  }
}

const copyChangeContent = async () => {
  if (!editForm.value.changeContent) {
    message.value = '变更内容为空，无法复制'
    return
  }
  try {
    await navigator.clipboard.writeText(editForm.value.changeContent)
    showToast('已复制到剪贴板', 'info')
  } catch (err) {
    console.error('复制失败:', err)
    message.value = '复制失败'
  }
}

onMounted(() => {
  loadTeamDbEnvs()
  loadData()
})
</script>

<template>
  <div class="card panel-card">
    <!-- ============ 列表视图 ============ -->
    <template v-if="viewMode === 'list'">
      <div class="panel-header">
        <h2>数据库变更申请</h2>
        <button class="action-btn primary-btn" @click="showCreateForm" type="button">+ 新建申请</button>
      </div>

      <div v-if="message" class="message-banner">{{ message }}</div>

      <div class="search-bar">
        <div class="search-item">
          <label>申请团队</label>
          <select v-model="searchForm.applicantTeam" @change="handleSearch">
            <option value="">全部</option>
            <option v-for="team in dynamicTeams" :key="team" :value="team">{{ team }}</option>
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
            <option value="Oracle">Oracle</option>
            <option value="MySQL">MySQL</option>
            <option value="redis">redis</option>
          </select>
        </div>
        <div class="search-actions">
          <button class="action-btn plain-btn" @click="resetSearch" type="button">重置</button>
        </div>
      </div>

      <div class="table-wrap" :class="{ loading: listLoading }">
        <table class="result-table">
          <colgroup>
            <col style="width: 9%">
            <col style="width: 6%">
            <col style="width: 9%">
            <col style="width: 6%">
            <col style="width: 7%">
            <col style="width: 8%">
            <col style="width: 7%">
            <col style="width: 7%">
            <col style="width: 6%">
            <col style="width: 5%">
            <col style="width: 7%">
            <col style="width: 9%">
            <col style="width: 6%">
            <col style="width: 8%">
          </colgroup>
          <thead>
            <tr>
              <th>申请团队</th>
              <th>数据库环境</th>
              <th>计划变更时间</th>
              <th>紧急程度</th>
              <th>数据库类型</th>
              <th>变更类型</th>
              <th>测试线 IP</th>
              <th>生产线 IP</th>
              <th>实例/库名</th>
              <th>Schema</th>
              <th>备份表</th>
              <th>创建时间</th>
              <th>发布状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="listData.length === 0">
              <td colspan="14" class="empty-text">暂无数据</td>
            </tr>
            <tr v-for="item in listData" :key="item.id">
              <td>{{ item.applicantTeam }}</td>
              <td>{{ item.environment }}</td>
              <td>{{ formatDate(item.plannedChangeTime) }}</td>
              <td>
                <span :class="['tag', item.urgencyLevel === '紧急' ? 'tag-danger' : 'tag-info']">
                  {{ item.urgencyLevel }}
                </span>
              </td>
              <td>{{ item.dbType }}</td>
              <td>{{ item.changeType }}</td>
              <td>{{ item.testDbIp }}</td>
              <td>{{ item.dbIp }}</td>
              <td>{{ item.dbName }}</td>
              <td>{{ item.dbSchema }}</td>
              <td>{{ item.backupTable }}</td>
              <td>{{ formatDate(item.createTime) }}</td>
              <td>
                <span v-if="!isReleased(item)" class="tag tag-info">待发布</span>
                <span v-else-if="!isVerified(item)" class="tag tag-warning">去验证</span>
                <span v-else class="tag tag-success">已验证</span>
              </td>
              <td>
                <div class="row-actions" v-if="!isReleased(item)">
                  <button class="text-btn" @click="startEdit(item)" type="button">编辑</button>
                  <button class="text-btn danger-text" @click="deleteRequest(item.id)" type="button">删除</button>
                </div>
                <div class="row-actions" v-else-if="!isVerified(item)">
                  <button class="text-btn" @click="startView(item)" type="button">查看</button>
                  <button class="text-btn" @click="startClone(item)" type="button">克隆</button>
                  <button class="text-btn success-text" @click="verifyRequest(item.id)" type="button">验证</button>
                </div>
                <div class="row-actions" v-else>
                  <button class="text-btn" @click="startView(item)" type="button">查看</button>
                  <button class="text-btn" @click="startClone(item)" type="button">克隆</button>
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
        <h2 v-if="viewMode === 'view'">查看数据库变更申请</h2>
        <h2 v-else-if="isEditMode">编辑数据库变更申请</h2>
        <h2 v-else>新建数据库变更申请</h2>
        <button class="action-btn secondary-btn back-btn" @click="backToList" type="button">← 返回列表</button>
      </div>
      <p v-if="message" class="inline-message">{{ message }}</p>

      <fieldset :disabled="viewMode === 'view'" style="border: none; padding: 0; margin: 0; min-width: 0;">
      <div class="form-grid">
        <div class="form-item">
          <label>申请团队 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <select v-model="editForm.applicantTeam" @change="selectedEnvId = ''">
            <option value="">请选择申请团队</option>
            <option v-for="team in dynamicTeams" :key="team" :value="team">{{ team }}</option>
          </select>
        </div>
        <div class="form-item">
          <label>可用环境 (选填，选择后自动填入下方信息)</label>
          <select v-model="selectedEnvId" @change="handleEnvChange" :disabled="!editForm.applicantTeam">
            <option value="">请选择环境</option>
            <option v-for="env in availableEnvs" :key="env.id" :value="env.id">{{ env.envName }}</option>
          </select>
        </div>
        <div class="form-item">
          <label>计划变更时间 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <input v-model="editForm.plannedChangeTime" type="datetime-local" />
        </div>
        <div class="form-item">
          <label>紧急程度 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <select v-model="editForm.urgencyLevel">
            <option value="常规">常规</option>
            <option value="紧急">紧急</option>
          </select>
        </div>

        <div class="form-item full-width">
          <label>变更类型 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <div class="checkbox-group">
            <label><input type="checkbox" value="新建表" v-model="editForm.changeType" /> 新建表</label>
            <label><input type="checkbox" value="修改表结构" v-model="editForm.changeType" /> 修改表结构</label>
            <label><input type="checkbox" value="数据修改" v-model="editForm.changeType" /> 数据修改</label>
            <label><input type="checkbox" value="数据同步" v-model="editForm.changeType" /> 数据同步</label>
            <label><input type="checkbox" value="其他" v-model="editForm.changeType" /> 其他</label>
          </div>
        </div>
        <div class="form-item full-width" v-if="editForm.changeType.includes('其他')">
          <label>其他变更类型理由 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <input v-model="editForm.otherChangeTypeReason" type="text" placeholder="请填写其他变更类型理由" />
        </div>

        <div class="form-item full-width">
          <label>数据库类型 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <div class="checkbox-group">
            <label><input type="checkbox" value="Oracle" v-model="editForm.dbType" /> Oracle</label>
            <label><input type="checkbox" value="MySQL" v-model="editForm.dbType" /> MySQL</label>
            <label><input type="checkbox" value="redis" v-model="editForm.dbType" /> redis</label>
            <label><input type="checkbox" value="其他" v-model="editForm.dbType" /> 其他</label>
          </div>
        </div>
        <div class="form-item full-width" v-if="editForm.dbType.includes('其他')">
          <label>其他数据库类型 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <input v-model="editForm.otherDbTypeReason" type="text" placeholder="请填写其他数据库类型" />
        </div>

        <div class="form-item">
          <label>测试线数据库 IP <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <input v-model="editForm.testDbIp" type="text" placeholder="如 127.0.0.1" />
        </div>
        <div class="form-item" v-if="editForm.dbType.includes('Oracle') || editForm.dbType.includes('MySQL') || editForm.dbType.includes('其他')">
          <label>测试线实例/数据库名 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <input v-model="editForm.testDbName" type="text" :placeholder="editForm.dbType.includes('Oracle') && !editForm.dbType.includes('MySQL') && !editForm.dbType.includes('其他') ? '实例名称' : '库名'" />
        </div>
        <div class="form-item" v-if="editForm.dbType.includes('Oracle')">
          <label>测试线数据库 Schema <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <input v-model="editForm.testDbSchema" type="text" placeholder="Schema名" />
        </div>

        <div class="form-item">
          <label>生产线数据库 IP <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <input v-model="editForm.dbIp" type="text" placeholder="如 127.0.0.1" />
        </div>
        <div class="form-item" v-if="editForm.dbType.includes('Oracle') || editForm.dbType.includes('MySQL') || editForm.dbType.includes('其他')">
          <label>生产线实例/数据库名 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <input v-model="editForm.dbName" type="text" :placeholder="editForm.dbType.includes('Oracle') && !editForm.dbType.includes('MySQL') && !editForm.dbType.includes('其他') ? '实例名称' : '库名'" />
        </div>
        <div class="form-item" v-if="editForm.dbType.includes('Oracle')">
          <label>生产线数据库 Schema <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <input v-model="editForm.dbSchema" type="text" placeholder="Schema名" />
        </div>

        <div class="form-item">
          <label>需求 URL</label>
          <input v-model="editForm.requirementUrl" type="text" placeholder="http://..." />
        </div>
        <div class="form-item full-width">
          <label>影响范围</label>
          <textarea v-model="editForm.impactScope" rows="2" placeholder="简述影响范围..."></textarea>
        </div>
        <div class="form-item full-width">
          <label>变更原因 <span v-if="viewMode !== 'view'" class="required">*</span></label>
          <textarea v-model="editForm.changeReason" rows="2" placeholder="简述变更原因..."></textarea>
        </div>
        <div class="form-item full-width">
          <label style="display: flex; align-items: center;">
            <span>变更内容 (SQL 等) <span v-if="viewMode !== 'view'" class="required">*</span></span>
            <input v-if="viewMode !== 'view'" type="file" @change="handleFileUpload" accept=".sql,.txt" style="display:inline-block; width:auto; margin-left: 10px; font-size: 13px;" />
            <button v-if="viewMode !== 'view'" class="action-btn plain-btn" @click="copyChangeContent" type="button" style="margin-left: 10px; padding: 2px 8px; font-size: 12px; height: 24px;">复制全部</button>
          </label>
          <textarea v-model="editForm.changeContent" rows="6" class="code-font" placeholder="填入具体的 SQL 或变更脚本..."></textarea>
        </div>
        <div class="form-item full-width">
          <label>备份表 <span v-if="isDataChange && viewMode !== 'view'" class="required">*</span></label>
          <textarea v-model="editForm.backupTable" rows="2" :placeholder="isDataChange ? '变更类型包含数据修改，备份表为必填项，多个表名可用逗号分隔...' : '填写备份表名，方便后续清理备份数据，多个表名可用逗号分隔...'"></textarea>
        </div>
      </div>
      </fieldset>

      <div v-if="viewMode === 'view'" style="margin-top: 10px; text-align: right;">
        <button class="action-btn plain-btn" @click="copyChangeContent" type="button" style="padding: 4px 12px; font-size: 13px;">复制变更内容</button>
      </div>

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
.text-btn { background: transparent; border: none; color: #409eff; cursor: pointer; padding: 4px 8px; font-size: 13px; }
.danger-text { color: #f56c6c; }
.success-text { color: #67c23a; }

/* 表格 */
.table-wrap { border: 1px solid #ebeef5; border-radius: 4px; margin-bottom: 20px; }
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

/* 复选框组 */
.checkbox-group { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; padding: 6px 0; }
.checkbox-group label { display: flex; align-items: center; gap: 4px; cursor: pointer; }

/* 行操作 */
.row-actions { display: flex; align-items: center; gap: 2px; flex-wrap: nowrap; white-space: nowrap; }

/* 代码字体 */
.code-font { font-family: Consolas, Monaco, monospace; }

.message-banner { background: #fdf6ec; color: #e6a23c; padding: 10px 16px; border-radius: 4px; margin-bottom: 16px; font-size: 14px; }
</style>
