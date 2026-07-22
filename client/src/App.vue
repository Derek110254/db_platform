<script setup lang="ts">
/**
 * App.vue
 * ------------------------------------------------------------------
 * 这是整个前端应用的总入口组件。
 *
 * 当前包含的主要功能：
 * 1. 首页（SQL 检测 / DDL 检测）
 * 2. 数据库查询页
 * 3. 管理员页面：
 *    - 用户管理
 *    - 查询数据库管理
 *    - SQL 审核管理
 *    - 数据库变更管理
 *    - 团队数据库环境
 * 4. 登录弹窗
 * 5. 查询历史
 * 6. SQL 收藏弹窗
 *
 * 权限规则：
 * 1. 未登录用户：只能访问首页
 * 2. 普通用户：可以访问查询页，但只能看到自己被授权的数据库连接
 * 3. admin：可以访问全部页面
 *
 * 说明：
 * 1. 这版不改变你原有查询逻辑
 * 2. 只把“SQL 收藏”这条链路接通
 * 3. SQL 收藏弹窗本身负责收藏列表加载、新增、编辑、删除
 * 4. App.vue 只负责：
 *    - 打开 / 关闭弹窗
 *    - 把当前查询页状态传给弹窗
 *    - 接收“使用收藏”后的回填结果
 */

import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import AdminConnectionPanel from './components/AdminConnectionPanel.vue'
import { useToasts, useConfirm, resolveConfirm } from './utils/toast'
import DashboardPanel from './components/DashboardPanel.vue'
import PersonalDashboardPanel from './components/PersonalDashboardPanel.vue'
import AdminUserPanel from './components/AdminUserPanel.vue'
import AdminAuditPanel from './components/AdminAuditPanel.vue'
import HomePanel from './components/HomePanel.vue'
import LoginDialog from './components/LoginDialog.vue'
import MetadataPanel from './components/MetadataPanel.vue'
import QueryHistoryPanel from './components/QueryHistoryPanel.vue'
import QueryPanel from './components/QueryPanel.vue'
import QueryPlanPanel from './components/QueryPlanPanel.vue'
import SqlFavoritePanel from './components/SqlFavoritePanel.vue'
import AuditHistoryPanel from './components/AuditHistoryPanel.vue'
import ChangePasswordDialog from './components/ChangePasswordDialog.vue'
import DbChangeRequestPanel from './components/DbChangeRequestPanel.vue'
import AdminDbChangeReleasePanel from './components/AdminDbChangeReleasePanel.vue'
import AdminTeamDbEnvPanel from './components/AdminTeamDbEnvPanel.vue'
import DbDataSyncRequestPanel from './components/DbDataSyncRequestPanel.vue'
import AdminDbDataSyncRequestPanel from './components/AdminDbDataSyncRequestPanel.vue'
import AdminDbAlertHandlePanel from './components/AdminDbAlertHandlePanel.vue'
import AdminOpsChangePanel from './components/AdminOpsChangePanel.vue'

/**
 * 数据库类型
 */
type DBType = 'mysql' | 'oracle'

/**
 * 页面类型
 */
type PageType = 'home' | 'dashboard' | 'personal-dashboard' | 'query' | 'query-plan' | 'audit-history' | 'admin-users' | 'admin-connections' | 'admin-audits' | 'db-change-requests' | 'db-data-sync-requests' | 'admin-db-change-release' | 'admin-db-data-sync-requests' | 'admin-team-db-envs' | 'admin-db-alert-handles' | 'admin-ops-change-records'

/**
 * 当前登录用户信息
 */
interface CurrentUserInfo {
  userId: number
  username: string
  displayName: string
  role: string
  canQueryData: number
  canQueryPlan: number
  needChangePwd: number
}

/**
 * 查询连接配置
 */
interface QueryConnectionInfo {
  name: string
  dbType: string
  host: string
  port: number
  database: string
  serviceName: string
  canConnect: number
}

/**
 * 表元数据
 */
interface QueryMetadataTable {
  name: string
  comment: string
}

/**
 * 字段元数据
 */
interface QueryMetadataColumn {
  tableName: string
  columnName: string
  columnType: string
  comment: string
}

/**
 * 查询历史项
 */
interface QueryHistoryItem {
  id: string
  connectionName: string
  dbType: DBType
  sql: string
  createdAt: string
}

/**
 * SQL 收藏“使用”时回填的数据结构
 */
interface FavoriteApplyPayload {
  dbType: DBType
  connectionName: string
  sqlText: string
}

/**
 * 查询历史在 localStorage 中的 key
 */
const HISTORY_STORAGE_KEY = 'db_query_history_v1'

/**
 * 根据浏览器地址解析当前页面
 */
const resolvePageByPath = (): PageType => {
  const path = window.location.pathname
  if (path === '/dashboard') return 'dashboard'
  if (path === '/personal-dashboard') return 'personal-dashboard'
  if (path === '/query') return 'query'
  if (path === '/query-plan') return 'query-plan'
  if (path === '/audit-history') return 'audit-history'
  if (path === '/admin/users') return 'admin-users'
  if (path === '/admin/connections') return 'admin-connections'
  if (path === '/admin/audits') return 'admin-audits'
  if (path === '/admin/db-change-release') return 'admin-db-change-release'
  if (path === '/admin/team-db-envs') return 'admin-team-db-envs'
  if (path === '/admin/db-alert-handles') return 'admin-db-alert-handles'
  if (path === '/admin/ops-change-records') return 'admin-ops-change-records'
  return 'home'
}

/**
 * 当前页面
 */
const currentPage = ref<PageType>(resolvePageByPath())

/**
 * 面包屑：根据当前页面生成标题
 */
const pageTitles: Record<string, string> = {
  'home': 'SQL 自查工具',
  'dashboard': '工作看板',
  'personal-dashboard': '个人看板',
  'query': '数据库查询',
  'query-plan': 'SQL 性能检测',
  'audit-history': '审核历史',
  'db-change-requests': '数据库变更申请',
  'db-data-sync-requests': '数据同步申请',
  'admin-users': '用户管理',
  'admin-connections': '查询数据库管理',
  'admin-audits': 'SQL 审核管理',
  'admin-db-change-release': '数据库变更管理',
  'admin-db-data-sync-requests': '数据同步管理',
  'admin-team-db-envs': '团队数据库环境',
  'admin-db-alert-handles': '数据库告警处理',
  'admin-ops-change-records': '运维变更记录',
}
const breadcrumbTitle = computed(() => pageTitles[currentPage.value] || '首页')
const sidebarCollapsed = ref(false)

// 个人看板待办数（用于侧边栏数字提醒）
const personalPendingCount = ref(0)
const loadPersonalPending = async () => {
  if (!isAuthenticated.value) return
  try {
    const res = await fetch('/api/dashboard/personal', { credentials: 'include' })
    const data = await res.json()
    if (data.ok) {
      personalPendingCount.value =
        (data.pendingAudit || 0) +
        (data.pendingChange || 0) +
        (data.pendingSync || 0) +
        (data.pendingAlert || 0) +
        (data.pendingOpsReview || 0)
    }
  } catch {
    // 静默失败
  }
}
const toggleSidebar = () => { sidebarCollapsed.value = !sidebarCollapsed.value }

/**
 * 登录相关状态
 */
const isAuthenticated = ref(false)
const currentUser = ref<CurrentUserInfo | null>(null)
const loginDialogVisible = ref(false)
const loginLoading = ref(false)
const loginMessage = ref('')

const changePasswordDialogVisible = ref(false)
const changePasswordDialogRef = ref<InstanceType<typeof ChangePasswordDialog> | null>(null)
const isForceChangePwd = ref(true)

const openChangePasswordDialog = () => {
  isForceChangePwd.value = false
  changePasswordDialogVisible.value = true
}

/**
 * 登录表单
 */
const loginForm = ref({
  username: '',
  password: '',
})

/**
 * 当前用户是否管理员
 */
const isAdmin = computed(() => currentUser.value?.role === 'admin')

/**
 * 用户是否有权限访问查询页
 */
const hasQueryDataPermission = computed(() => {
  return isAdmin.value || currentUser.value?.canQueryData === 1
})

/**
 * 用户是否有权限访问执行计划页
 */
const hasQueryPlanPermission = computed(() => {
  return isAdmin.value || currentUser.value?.canQueryPlan === 1
})

/**
 * 新增：保存 AI 打分
 */
const queryScore = ref(0)
const queryAuditId = ref(0) 

/**
 * 查询页状态
 */
const queryLoading = ref(false)
const exportLoading = ref(false)
const metadataLoading = ref(false)
const queryMessage = ref('还没有执行查询')

/**
 * 当前用户可见的数据库连接列表
 */
const queryConnections = ref<QueryConnectionInfo[]>([])

/**
 * 查询页当前选中的数据库类型
 */
const queryDBType = ref<DBType>('mysql')

/**
 * 当前选中的连接名称
 */
const selectedConnectionName = ref('')

/**
 * 查询 SQL 文本
 */
const queryText = ref('')

/**
 * 查询结果列名
 */
const queryColumns = ref<string[]>([])

/**
 * 查询结果数据
 */
const queryRows = ref<Record<string, unknown>[]>([])

/**
 * 元数据查询关键字
 */
const metadataKeyword = ref('')

/**
 * 元数据表列表
 */
const metadataTables = ref<QueryMetadataTable[]>([])

/**
 * 元数据字段列表
 */
const metadataColumns = ref<QueryMetadataColumn[]>([])

/**
 * 当前选中的元数据表
 */
const selectedMetadataTable = ref('')

/**
 * 查询历史
 */
const queryHistory = ref<QueryHistoryItem[]>([])

/**
 * 查询结果分页状态
 */
const currentPageNo = ref(1)
const pageSize = ref(10)
const pageSizeOptions = [10, 20, 50, 100]

/**
 * 当前查询请求的中止控制器
 */
const queryAbortController = ref<AbortController | null>(null)

/**
 * 元数据请求序号
 * 用于防止旧请求覆盖新请求
 */
const metadataRequestId = ref(0)

/* ------------------------------------------------------------------ */
/* SQL 收藏弹窗状态                                                    */
/* ------------------------------------------------------------------ */

/**
 * SQL 收藏弹窗是否显示
 */
const sqlFavoriteVisible = ref(false)

/* ------------------------------------------------------------------ */
/* 计算属性                                                            */
/* ------------------------------------------------------------------ */

/**
 * 当前数据库类型下可用的连接列表
 */
const filteredConnections = computed(() =>
  queryConnections.value.filter((item) => item.dbType === queryDBType.value),
)

/**
 * 当前选中的连接对象
 */
const selectedConnection = computed(() =>
  filteredConnections.value.find((item) => item.name === selectedConnectionName.value),
)

/**
 * 查询结果总行数
 */
const totalQueryRows = computed(() => queryRows.value.length)

/**
 * 查询结果总页数
 */
const totalPages = computed(() => {
  if (totalQueryRows.value === 0) return 1
  return Math.ceil(totalQueryRows.value / pageSize.value)
})

/**
 * 当前页展示的数据
 */
const pagedQueryRows = computed(() => {
  const start = (currentPageNo.value - 1) * pageSize.value
  const end = start + pageSize.value
  return queryRows.value.slice(start, end)
})

/**
 * 当前选中表对应的字段列表
 */
const currentTableColumns = computed(() => {
  if (!selectedMetadataTable.value) return []
  return metadataColumns.value.filter((item) => item.tableName === selectedMetadataTable.value)
})

/**
 * 按关键字实时过滤表列表（客户端过滤，与团队数据库环境搜索一致）
 */
const filteredMetadataTables = computed(() => {
  const kw = metadataKeyword.value.trim().toLowerCase()
  if (!kw) return metadataTables.value
  return metadataTables.value.filter((item) =>
    item.name.toLowerCase().includes(kw) || (item.comment && item.comment.toLowerCase().includes(kw))
  )
})

/**
 * 按关键字实时过滤字段列表（同时过滤表名和字段名）
 */
const filteredMetadataColumns = computed(() => {
  const kw = metadataKeyword.value.trim().toLowerCase()
  const base = selectedMetadataTable.value
    ? metadataColumns.value.filter((item) => item.tableName === selectedMetadataTable.value)
    : metadataColumns.value
  if (!kw) return base
  return base.filter((item) =>
    item.columnName.toLowerCase().includes(kw) || (item.tableName && item.tableName.toLowerCase().includes(kw))
  )
})

/* ------------------------------------------------------------------ */
/* 监听器                                                              */
/* ------------------------------------------------------------------ */

/**
 * 每页条数变化时回到第一页
 */
watch(pageSize, () => {
  currentPageNo.value = 1
})

/**
 * 切换数据库类型时：
 * 1. 清空当前连接
 * 2. 清空元数据
 */
watch(queryDBType, () => {
  selectedConnectionName.value = ''
  clearMetadata()
})

/**
 * 当前数据库类型下的连接列表变化时：
 * 若当前选中连接不存在，则自动切到第一个
 */
watch(filteredConnections, (items) => {
  if (items.length === 0) {
    selectedConnectionName.value = ''
    return
  }

  const found = items.some((item) => item.name === selectedConnectionName.value)
  if (!found) {
    const first = items[0]
    if (first) {
      selectedConnectionName.value = first.name
    }
  }
})

/**
 * 切换连接时自动重新加载元数据
 */
watch(selectedConnectionName, async (connName) => {
  clearMetadata()
  metadataKeyword.value = ''
  if (currentPage.value === 'query' && connName && isAuthenticated.value) {
    const conn = queryConnections.value.find(c => c.name === connName)
    if (conn && conn.canConnect === 0) {
      queryMessage.value = '该数据库不可查询'
      return
    }
    await loadQueryMetadata()
  }
})

/**
 * 根据页面动态设置浏览器标题
 */
watch(
  currentPage,
  (page) => {
    if (page === 'query') {
      document.title = '数据库查询平台'
    } else if (page === 'admin-users') {
      document.title = '用户管理'
    } else if (page === 'admin-connections') {
      document.title = '查询数据库管理'
    } else if (page === 'admin-audits') {
      document.title = 'SQL 审核管理'
    } else if (page === 'db-change-requests') {
      document.title = '数据库变更申请'
    } else if (page === 'admin-db-change-release') {
      document.title = '数据库变更管理'
    } else if (page === 'admin-team-db-envs') {
      document.title = '团队数据库环境'
    } else {
      document.title = 'SQL 综合管理平台'
    }
  },
  { immediate: true },
)

/* ------------------------------------------------------------------ */
/* 页面导航                                                            */
/* ------------------------------------------------------------------ */

/**
 * 根据页面类型生成浏览器路径
 */
const getPathByPage = (page: PageType): string => {
  if (page === 'dashboard') return '/dashboard'
  if (page === 'personal-dashboard') return '/personal-dashboard'
  if (page === 'query') return '/query'
  if (page === 'query-plan') return '/query-plan'
  if (page === 'audit-history') return '/audit-history'
  if (page === 'db-change-requests') return '/db-change-requests'
  if (page === 'admin-users') return '/admin/users'
  if (page === 'admin-connections') return '/admin/connections'
  if (page === 'admin-audits') return '/admin/audits'
  if (page === 'admin-db-change-release') return '/admin/db-change-release'
  if (page === 'admin-team-db-envs') return '/admin/team-db-envs'
  if (page === 'admin-db-alert-handles') return '/admin/db-alert-handles'
  if (page === 'admin-ops-change-records') return '/admin/ops-change-records'
  return '/'
}

/**
 * 页面跳转
 */
const navigateTo = async (page: PageType) => {
  if (page === 'query' || page === 'query-plan' || page === 'db-change-requests') {
    const ok = await checkAuthStatus(false)
    if (!ok) {
      loginDialogVisible.value = true
      loginMessage.value = '请先登录后再访问该页面'
      currentPage.value = 'home'
      window.history.pushState({}, '', '/')
      return
    }
  }

  if (page === 'admin-users' || page === 'admin-connections' || page === 'admin-audits' || page === 'admin-db-change-release' || page === 'admin-team-db-envs' || page === 'admin-db-alert-handles' || page === 'admin-ops-change-records' || page === 'personal-dashboard') {
    const ok = await checkAuthStatus(false)
    if (!ok) {
      loginDialogVisible.value = true
      loginMessage.value = '请先登录后再访问管理员页面'
      currentPage.value = 'home'
      window.history.pushState({}, '', '/')
      return
    }

    if (!isAdmin.value) {
      currentPage.value = 'home'
      window.history.pushState({}, '', '/')
      return
    }
  }

  if (currentPage.value !== page && (page === 'query' || page === 'query-plan' || currentPage.value === 'query' || currentPage.value === 'query-plan')) {
    clearQueryResult()
  }

  currentPage.value = page
  window.history.pushState({}, '', getPathByPage(page))

  if (page === 'query' || page === 'query-plan') {
    await loadQueryConnections()
    loadQueryHistory()
    if (selectedConnectionName.value) {
      await loadQueryMetadata()
    }
  }
}

/**
 * 浏览器前进 / 后退处理
 */
const handlePopState = async () => {
  const targetPage = resolvePageByPath()

  if (targetPage === 'query' || targetPage === 'query-plan') {
    const ok = await checkAuthStatus(false)
    if (!ok) {
      currentPage.value = 'home'
      window.history.replaceState({}, '', '/' + window.location.search)
      loginDialogVisible.value = true
      loginMessage.value = '请先登录后再访问该页面'
      return
    }
  }

  if (targetPage === 'admin-users' || targetPage === 'admin-connections' || targetPage === 'admin-audits' || targetPage === 'admin-db-change-release' || targetPage === 'admin-team-db-envs' || targetPage === 'admin-db-alert-handles' || targetPage === 'admin-ops-change-records' || targetPage === 'personal-dashboard') {
    const ok = await checkAuthStatus(false)
    if (!ok || !isAdmin.value) {
      currentPage.value = 'home'
      window.history.replaceState({}, '', '/' + window.location.search)
      return
    }
  }

  if (currentPage.value !== targetPage && (targetPage === 'query' || targetPage === 'query-plan' || currentPage.value === 'query' || currentPage.value === 'query-plan')) {
    clearQueryResult()
  }

  currentPage.value = targetPage
}

/* ------------------------------------------------------------------ */
/* 生命周期                                                            */
/* ------------------------------------------------------------------ */

onMounted(async () => {
  window.addEventListener('popstate', handlePopState)

  if (window.location.search.includes('token=') || window.location.hash.includes('token=')) {
    loginDialogVisible.value = true
  }

  await checkAuthStatus(false)
  loadPersonalPending()

  // 已登录且访问首页时，自动跳转到工作看板
  if (isAuthenticated.value && (currentPage.value === 'home')) {
    currentPage.value = 'dashboard'
    window.history.replaceState({}, '', '/dashboard')
  }

  // 未登录时访问看板，跳回 SQL 自查工具
  if (!isAuthenticated.value && currentPage.value === 'dashboard') {
    currentPage.value = 'home'
    window.history.replaceState({}, '', '/')
  }

  if (currentPage.value === 'query' || currentPage.value === 'query-plan') {
    const ok = await checkAuthStatus(false)
    if (!ok) {
      currentPage.value = 'home'
      window.history.replaceState({}, '', '/' + window.location.search)
      loginDialogVisible.value = true
      loginMessage.value = '请先登录后再访问该页面'
      return
    }

    await loadQueryConnections()
    loadQueryHistory()
    if (selectedConnectionName.value) {
      await loadQueryMetadata()
    }
  }

  if (currentPage.value === 'admin-users' || currentPage.value === 'admin-connections' || currentPage.value === 'admin-audits' || currentPage.value === 'admin-db-change-release' || currentPage.value === 'admin-team-db-envs') {
    const ok = await checkAuthStatus(false)
    if (!ok || !isAdmin.value) {
      currentPage.value = 'home'
      window.history.replaceState({}, '', '/')
    }
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('popstate', handlePopState)

  if (queryAbortController.value) {
    queryAbortController.value.abort()
    queryAbortController.value = null
  }
})

/* ------------------------------------------------------------------ */
/* 登录相关                                                            */
/* ------------------------------------------------------------------ */

/**
 * 打开登录弹窗
 */
const openLoginDialog = () => {
  loginDialogVisible.value = true
  loginMessage.value = ''
}

/**
 * 关闭登录弹窗
 */
const closeLoginDialog = () => {
  loginDialogVisible.value = false
  loginMessage.value = ''
}

/**
 * 检查当前登录状态
 */
const checkAuthStatus = async (showError = true): Promise<boolean> => {
  try {
    const res = await fetch('/api/auth/me', {
      credentials: 'include',
    })

    if (!res.ok) {
      isAuthenticated.value = false
      currentUser.value = null
      if (showError) {
        loginMessage.value = '当前未登录'
      }
      return false
    }

    const data = await res.json()
    isAuthenticated.value = !!data.ok
    currentUser.value = {
      userId: data.userId,
      username: data.username,
      displayName: data.displayName || '',
      role: data.role || '',
      canQueryData: data.canQueryData ?? 1,
      canQueryPlan: data.canQueryPlan ?? 1,
      needChangePwd: data.needChangePwd ?? 0,
    }
    
    if (currentUser.value.needChangePwd === 1) {
      isForceChangePwd.value = true
      changePasswordDialogVisible.value = true
    }
    
    return true
  } catch (err) {
    console.error(err)
    isAuthenticated.value = false
    currentUser.value = null
    if (showError) {
      loginMessage.value = '登录状态校验失败'
    }
    return false
  }
}

/**
 * 提交登录
 */
const submitLogin = async () => {
  loginLoading.value = true
  loginMessage.value = ''

  try {
    const res = await fetch('/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({
        username: loginForm.value.username,
        password: loginForm.value.password,
      }),
    })

    const data = await res.json()

    if (!res.ok || !data.ok) {
      loginMessage.value = data.message || '登录失败'
      return
    }

    await checkAuthStatus(false)
    loginDialogVisible.value = false
    loginForm.value.password = ''
    loginMessage.value = ''

    if (currentUser.value?.needChangePwd !== 1) {
      currentPage.value = 'dashboard'
      window.history.pushState({}, '', '/dashboard')
    }
  } catch (err) {
    console.error(err)
    loginMessage.value = '登录失败，请检查后端是否启动'
  } finally {
    loginLoading.value = false
  }
}

/**
 * 处理 SSO 登录成功
 */
const handleSsoSuccess = async () => {
  await checkAuthStatus(false)
  loginDialogVisible.value = false
  loginMessage.value = ''

  if (currentUser.value?.needChangePwd !== 1) {
    navigateTo('dashboard')
  }
}

/**
 * 提交修改密码
 */
const handleChangePasswordSubmit = async (oldPassword: string, newPassword: string) => {
  if (changePasswordDialogRef.value) {
    changePasswordDialogRef.value.setMessage('')
  }
  try {
    const res = await fetch('/api/user/change-password', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ oldPassword, newPassword }),
    })
    const data = await res.json()
    if (!res.ok || !data.ok) {
      if (changePasswordDialogRef.value) {
        changePasswordDialogRef.value.setMessage(data.message || '修改密码失败')
      }
      return
    }
    
    // 修改成功，关闭弹窗并重新加载数据
    changePasswordDialogVisible.value = false
    if (currentUser.value) {
      currentUser.value.needChangePwd = 0
    }
    
    // 登录后的正常跳转流程
    if (currentPage.value === 'home') {
      currentPage.value = 'query'
      window.history.pushState({}, '', '/query')
    }
    await loadQueryConnections()
    loadQueryHistory()
    if (selectedConnectionName.value) {
      await loadQueryMetadata()
    }
  } catch (err) {
    console.error(err)
    if (changePasswordDialogRef.value) {
      changePasswordDialogRef.value.setMessage('修改密码请求失败')
    }
  }
}

/**
 * 退出登录
 */
const logout = async () => {
  try {
    await fetch('/api/logout', {
      method: 'POST',
      credentials: 'include',
    })
  } catch (err) {
    console.error(err)
  }

  isAuthenticated.value = false
  currentUser.value = null
  currentPage.value = 'home'
  window.history.pushState({}, '', '/')
  clearQuery()
  clearMetadata()
  sqlFavoriteVisible.value = false
}

/**
 * 统一处理登录失效
 */
const handleUnauthorized = (message: string) => {
  isAuthenticated.value = false
  currentUser.value = null
  currentPage.value = 'home'
  window.history.replaceState({}, '', '/')
  loginDialogVisible.value = true
  loginMessage.value = message
}

/* ------------------------------------------------------------------ */
/* 查询页逻辑                                                          */
/* ------------------------------------------------------------------ */

/**
 * 清空元数据
 */
const clearMetadata = () => {
  metadataTables.value = []
  metadataColumns.value = []
  selectedMetadataTable.value = ''
}

/**
 * 清空查询内容
 */
const clearQuery = () => {
  queryText.value = ''
  queryColumns.value = []
  queryRows.value = []
  queryMessage.value = '已清空查询内容'
  currentPageNo.value = 1
}

/**
 * 仅清空查询结果（不清除SQL文本），用于页面切换
 */
const clearQueryResult = () => {
  queryColumns.value = []
  queryRows.value = []
  queryMessage.value = '还没有执行查询'
  currentPageNo.value = 1
}

/**
 * 选中某张元数据表
 */
const selectMetadataTable = (tableName: string) => {
  selectedMetadataTable.value = tableName
}

/**
 * 加载当前用户可见的连接列表
 */
const loadQueryConnections = async () => {
  try {
    const res = await fetch('/api/query-connections', {
      credentials: 'include',
    })

    if (res.status === 401) {
      handleUnauthorized('登录已失效，请重新登录')
      queryConnections.value = []
      return
    }

    const data = await res.json()
    queryConnections.value = data.connections || []

    if (queryConnections.value.length === 0) {
      queryMessage.value = '当前用户未分配可查询数据库连接'
    }
  } catch (err) {
    console.error(err)
    queryConnections.value = []
  }
}

/**
 * 加载元数据
 */
const loadQueryMetadata = async () => {
  if (!selectedConnectionName.value) {
    clearMetadata()
    return
  }

  const conn = queryConnections.value.find(c => c.name === selectedConnectionName.value)
  if (conn && conn.canConnect === 0) {
    clearMetadata()
    return
  }

  const currentRequestId = ++metadataRequestId.value
  metadataLoading.value = true

  try {
    const res = await fetch('/api/query-metadata', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({
        connectionName: selectedConnectionName.value,
        keyword: '',
      }),
    })

    if (res.status === 401) {
      handleUnauthorized('登录已失效，请重新登录')
      return
    }

    const data = await res.json()

    if (currentRequestId !== metadataRequestId.value) return

    metadataTables.value = data.tables || []
    metadataColumns.value = data.columns || []
    selectedMetadataTable.value = ''

    if (!data.ok) {
      queryMessage.value = data.message || '元数据加载失败'
    }
  } catch (err) {
    console.error(err)
    if (currentRequestId === metadataRequestId.value) {
      metadataTables.value = []
      metadataColumns.value = []
      selectedMetadataTable.value = ''
    }
  } finally {
    if (currentRequestId === metadataRequestId.value) {
      metadataLoading.value = false
    }
  }
}

/**
 * 加载查询历史
 */
const loadQueryHistory = () => {
  try {
    const raw = localStorage.getItem(HISTORY_STORAGE_KEY)
    if (!raw) {
      queryHistory.value = []
      return
    }
    queryHistory.value = JSON.parse(raw) || []
  } catch (err) {
    console.error(err)
    queryHistory.value = []
  }
}

/**
 * 保存查询历史
 */
const saveQueryHistory = () => {
  localStorage.setItem(HISTORY_STORAGE_KEY, JSON.stringify(queryHistory.value))
}

/**
 * 新增查询历史
 */
const addQueryHistory = () => {
  const sql = queryText.value.trim()
  if (!sql || !selectedConnectionName.value) return

  const item: QueryHistoryItem = {
    id: `${Date.now()}`,
    connectionName: selectedConnectionName.value,
    dbType: queryDBType.value,
    sql,
    createdAt: new Date().toLocaleString(),
  }

  const filtered = queryHistory.value.filter(
    (x) => !(x.connectionName === item.connectionName && x.sql === item.sql),
  )

  queryHistory.value = [item, ...filtered].slice(0, 20)
  saveQueryHistory()
}

/**
 * 应用历史 SQL
 */
const applyHistoryItem = (item: QueryHistoryItem) => {
  queryDBType.value = item.dbType
  selectedConnectionName.value = item.connectionName
  queryText.value = item.sql
  queryMessage.value = `已加载历史查询：${item.createdAt}`
}

/**
 * 删除历史 SQL
 */
const deleteHistoryItem = (id: string) => {
  queryHistory.value = queryHistory.value.filter((item) => item.id !== id)
  saveQueryHistory()
}

/**
 * 执行查询
 */
const executeQuery = async () => {
  if (queryLoading.value) return

  if (!selectedConnectionName.value) {
    queryMessage.value = '请选择可查询数据库连接'
    return
  }

  const conn = queryConnections.value.find(c => c.name === selectedConnectionName.value)
  if (conn && conn.canConnect === 0) {
    queryMessage.value = '该数据库不可查询'
    return
  }

  queryAbortController.value = new AbortController()
  queryLoading.value = true
  queryColumns.value = []
  queryRows.value = []
  queryMessage.value = '查询中...'

  try {
    const res = await fetch('/api/query-data', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({
        connectionName: selectedConnectionName.value,
        sql: queryText.value,
      }),
      signal: queryAbortController.value.signal,
    })

    if (res.status === 401) {
      handleUnauthorized('登录已失效，请重新登录')
      return
    }

    const data = await res.json()
    queryMessage.value = data.message || '查询完成'
    queryColumns.value = data.columns || []
    queryRows.value = data.rows || []
    currentPageNo.value = 1

    if (data.ok) {
      addQueryHistory()
    }
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') {
      queryMessage.value = '查询已中止'
    } else {
      console.error(err)
      queryMessage.value = '查询失败，请检查后端是否启动'
    }
  } finally {
    queryLoading.value = false
    queryAbortController.value = null
  }
}

/**
 * 执行计划查询 (Explain)
 */
const explainQuery = async () => {
  if (queryLoading.value) return

  if (!selectedConnectionName.value) {
    queryMessage.value = '请选择可查询数据库连接'
    return
  }

  const conn = queryConnections.value.find(c => c.name === selectedConnectionName.value)
  if (conn && conn.canConnect === 0) {
    queryMessage.value = '该数据库不可检测'
    return
  }

  if (!queryText.value.trim()) {
    queryMessage.value = '请输入需要检测的 SQL 语句'
    return
  }

  queryAbortController.value = new AbortController()
  queryLoading.value = true
  queryColumns.value = []
  queryRows.value = []
  queryMessage.value = '正在获取执行计划...'

  try {
    const res = await fetch('/api/query-plan', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({
        connectionName: selectedConnectionName.value,
        sql: queryText.value,
      }),
      signal: queryAbortController.value.signal,
    })

    if (res.status === 401) {
      handleUnauthorized('登录已失效，请重新登录')
      return
    }

    const data = await res.json()
    queryMessage.value = data.message || '获取执行计划完成'
    queryColumns.value = data.columns || []
    queryRows.value = data.rows || []
    // ====== 关键修改：接收后端返回的 Score 和 AuditID ======
    queryScore.value = data.score || 0
    queryAuditId.value = data.auditId || 0
    // ========================================

    currentPageNo.value = 1
    addQueryHistory()
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') {
      queryMessage.value = '检测已中止'
    } else {
      console.error(err)
      queryMessage.value = '检测失败，请检查后端是否启动'
    }
  } finally {
    queryLoading.value = false
    queryAbortController.value = null
  }
}

/**
 * 手动提交执行计划进行性能检测
 * 用于不可自动连接的数据库
 */
const manualExplainQuery = async (payload: { executionPlan: string }) => {
  if (queryLoading.value) return

  if (!selectedConnectionName.value) {
    queryMessage.value = '请选择数据库连接'
    return
  }

  if (!queryText.value.trim()) {
    queryMessage.value = '请输入需要检测的 SQL 语句'
    return
  }

  if (!payload.executionPlan.trim()) {
    queryMessage.value = '请粘贴执行计划内容'
    return
  }

  queryAbortController.value = new AbortController()
  queryLoading.value = true
  queryColumns.value = []
  queryRows.value = []
  queryMessage.value = '正在分析执行计划...'

  try {
    const res = await fetch('/api/query-plan/manual', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({
        connectionName: selectedConnectionName.value,
        sql: queryText.value,
        executionPlan: payload.executionPlan,
      }),
      signal: queryAbortController.value.signal,
    })

    if (res.status === 401) {
      handleUnauthorized('登录已失效，请重新登录')
      return
    }

    const data = await res.json()
    queryMessage.value = data.message || '检测完成'
    queryScore.value = data.score || 0
    queryAuditId.value = data.auditId || 0
    queryColumns.value = data.columns || []
    queryRows.value = data.rows || []
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') {
      queryMessage.value = '检测已中止'
    } else {
      console.error(err)
      queryMessage.value = '检测失败，请检查后端是否启动'
    }
  } finally {
    queryLoading.value = false
    queryAbortController.value = null
  }
}

/**
 * 中止查询
 */
const cancelQuery = () => {
  if (queryAbortController.value) {
    queryAbortController.value.abort()
    queryAbortController.value = null
  }
}

/**
 * 导出 Excel
 */
const exportQueryExcel = async () => {
  if (!selectedConnectionName.value) {
    queryMessage.value = '请选择可导出的数据库连接'
    return
  }

  exportLoading.value = true
  try {
    const res = await fetch('/api/query-export-excel', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({
        connectionName: selectedConnectionName.value,
        sql: queryText.value,
      }),
    })

    if (res.status === 401) {
      handleUnauthorized('登录已失效，请重新登录')
      return
    }

    if (!res.ok) {
      const errData = await res.json().catch(() => null)
      queryMessage.value = errData?.message || '导出失败'
      return
    }

    const blob = await res.blob()
    const disposition = res.headers.get('Content-Disposition') || ''
    let outFileName = 'query_result.xlsx'
    const match = disposition.match(/filename="([^"]+)"/)
    if (match && match[1]) {
      outFileName = decodeURIComponent(match[1])
    }

    const url = window.URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = outFileName
    document.body.appendChild(a)
    a.click()
    a.remove()
    window.URL.revokeObjectURL(url)

    queryMessage.value = 'Excel 导出成功'
  } catch (err) {
    console.error(err)
    queryMessage.value = '导出失败，请检查后端是否启动'
  } finally {
    exportLoading.value = false
  }
}

/**
 * 上一页
 */
const goPrevPage = () => {
  if (currentPageNo.value > 1) currentPageNo.value--
}

/**
 * 下一页
 */
const goNextPage = () => {
  if (currentPageNo.value < totalPages.value) currentPageNo.value++
}

/**
 * 接收 QueryPanel 的消息
 */
const handleQueryPanelMessage = (message: string) => {
  queryMessage.value = message
}

/* ------------------------------------------------------------------ */
/* SQL 收藏弹窗逻辑                                                    */
/* ------------------------------------------------------------------ */

/**
 * SQL 收藏弹窗模式：add 新增 / view 查看列表
 */
const sqlFavoriteMode = ref<'add' | 'view'>('add')

/**
 * 打开 SQL 收藏弹窗（新增模式）
 */
const openSQLFavoritesAdd = () => {
  sqlFavoriteMode.value = 'add'
  sqlFavoriteVisible.value = true
}

/**
 * 打开 SQL 收藏弹窗（查看列表模式）
 */
const openSQLFavoritesView = () => {
  sqlFavoriteMode.value = 'view'
  sqlFavoriteVisible.value = true
}

/**
 * 关闭 SQL 收藏弹窗
 */
const closeSQLFavorites = () => {
  sqlFavoriteVisible.value = false
}

/**
 * 处理“使用收藏”
 * 把收藏里的 dbType / connectionName / sqlText 回填到查询页
 */
const handleApplyFavorite = (payload: FavoriteApplyPayload) => {
  queryDBType.value = payload.dbType
  selectedConnectionName.value = payload.connectionName || ''
  queryText.value = payload.sqlText || ''
  queryMessage.value = '已从 SQL 收藏回填到查询页'
}

/**
 * 接收收藏弹窗内部发出的提示信息
 */
const handleFavoriteMessage = (message: string) => {
  queryMessage.value = message
}

/* ------------------------------------------------------------------ */
/* 其它                                                                */
/* ------------------------------------------------------------------ */

/**
 * 回到顶部
 */
const scrollToTop = () => {
  window.scrollTo({
    top: 0,
    behavior: 'smooth',
  })
}
</script>

<template>
  <div class="app-layout">
    <!-- 左侧导航栏 -->
    <aside class="sidebar" :class="{ collapsed: sidebarCollapsed }">
      <div class="sidebar-brand" v-if="!sidebarCollapsed">SQL 综合管理平台</div>
      <div class="sidebar-brand" v-else>SQL</div>

      <nav class="sidebar-nav">
        <div class="nav-group-label" v-if="!sidebarCollapsed">主要功能</div>
        <button v-if="isAuthenticated" :class="['nav-item', currentPage === 'dashboard' ? 'active' : '']" @click="navigateTo('dashboard')" type="button" :title="sidebarCollapsed ? '工作看板' : ''">
          <span class="nav-icon">📈</span><span v-if="!sidebarCollapsed">工作看板</span>
        </button>
        <button :class="['nav-item', currentPage === 'home' ? 'active' : '']" @click="navigateTo('home')" type="button" :title="sidebarCollapsed ? 'SQL 自查工具' : ''">
          <span class="nav-icon">🔍</span><span v-if="!sidebarCollapsed">SQL 自查工具</span>
        </button>
        <button v-if="hasQueryDataPermission" :class="['nav-item', currentPage === 'query' ? 'active' : '']" @click="navigateTo('query')" type="button" :title="sidebarCollapsed ? '数据库查询' : ''">
          <span class="nav-icon">📊</span><span v-if="!sidebarCollapsed">数据库查询</span>
        </button>
        <button v-if="hasQueryPlanPermission" :class="['nav-item', currentPage === 'query-plan' ? 'active' : '']" @click="navigateTo('query-plan')" type="button" :title="sidebarCollapsed ? 'SQL 性能检测' : ''">
          <span class="nav-icon">⚡</span><span v-if="!sidebarCollapsed">SQL 性能检测</span>
        </button>

        <template v-if="isAuthenticated">
          <div class="nav-group-label" v-if="!sidebarCollapsed">我的申请</div>
          <button :class="['nav-item', currentPage === 'db-change-requests' ? 'active' : '']" @click="navigateTo('db-change-requests')" type="button" :title="sidebarCollapsed ? '数据库变更申请' : ''">
            <span class="nav-icon">📝</span><span v-if="!sidebarCollapsed">数据库变更申请</span>
          </button>
          <button :class="['nav-item', currentPage === 'db-data-sync-requests' ? 'active' : '']" @click="navigateTo('db-data-sync-requests')" type="button" :title="sidebarCollapsed ? '数据同步申请' : ''">
            <span class="nav-icon">🔄</span><span v-if="!sidebarCollapsed">数据同步申请</span>
          </button>
          <button :class="['nav-item', currentPage === 'audit-history' ? 'active' : '']" @click="navigateTo('audit-history')" type="button" :title="sidebarCollapsed ? '审核历史' : ''">
            <span class="nav-icon">📋</span><span v-if="!sidebarCollapsed">审核历史</span>
          </button>
        </template>

        <template v-if="isAdmin">
          <div class="nav-group-label" v-if="!sidebarCollapsed">管理功能</div>
          <button :class="['nav-item', currentPage === 'personal-dashboard' ? 'active' : '']" @click="navigateTo('personal-dashboard')" type="button" :title="sidebarCollapsed ? '个人看板' : ''">
            <span class="nav-icon">👤</span><span v-if="!sidebarCollapsed">个人看板</span>
            <span v-if="personalPendingCount > 0" class="nav-badge">{{ personalPendingCount }}</span>
          </button>
          <button :class="['nav-item', currentPage === 'admin-audits' ? 'active' : '']" @click="navigateTo('admin-audits')" type="button" :title="sidebarCollapsed ? 'SQL 审核管理' : ''">
            <span class="nav-icon">✅</span><span v-if="!sidebarCollapsed">SQL 审核管理</span>
          </button>
          <button :class="['nav-item', currentPage === 'admin-db-change-release' ? 'active' : '']" @click="navigateTo('admin-db-change-release')" type="button" :title="sidebarCollapsed ? '数据库变更管理' : ''">
            <span class="nav-icon">📦</span><span v-if="!sidebarCollapsed">数据库变更管理</span>
          </button>
          <button :class="['nav-item', currentPage === 'admin-db-data-sync-requests' ? 'active' : '']" @click="navigateTo('admin-db-data-sync-requests')" type="button" :title="sidebarCollapsed ? '数据同步管理' : ''">
            <span class="nav-icon">🔁</span><span v-if="!sidebarCollapsed">数据同步管理</span>
          </button>
          <button :class="['nav-item', currentPage === 'admin-db-alert-handles' ? 'active' : '']" @click="navigateTo('admin-db-alert-handles')" type="button" :title="sidebarCollapsed ? '数据库告警处理' : ''">
            <span class="nav-icon">🚨</span><span v-if="!sidebarCollapsed">数据库告警处理</span>
          </button>
          <button :class="['nav-item', currentPage === 'admin-ops-change-records' ? 'active' : '']" @click="navigateTo('admin-ops-change-records')" type="button" :title="sidebarCollapsed ? '运维变更记录' : ''">
            <span class="nav-icon">🔧</span><span v-if="!sidebarCollapsed">运维变更记录</span>
          </button>
          <button :class="['nav-item', currentPage === 'admin-connections' ? 'active' : '']" @click="navigateTo('admin-connections')" type="button" :title="sidebarCollapsed ? '查询数据库管理' : ''">
            <span class="nav-icon">🗄</span><span v-if="!sidebarCollapsed">查询数据库管理</span>
          </button>
          <button :class="['nav-item', currentPage === 'admin-team-db-envs' ? 'active' : '']" @click="navigateTo('admin-team-db-envs')" type="button" :title="sidebarCollapsed ? '团队数据库环境' : ''">
            <span class="nav-icon">🌍</span><span v-if="!sidebarCollapsed">团队数据库环境</span>
          </button>
          <button :class="['nav-item', currentPage === 'admin-users' ? 'active' : '']" @click="navigateTo('admin-users')" type="button" :title="sidebarCollapsed ? '用户管理' : ''">
            <span class="nav-icon">👥</span><span v-if="!sidebarCollapsed">用户管理</span>
          </button>
        </template>
      </nav>

      <div class="sidebar-footer">
        <button class="sidebar-toggle-btn" @click="toggleSidebar" type="button">
          {{ sidebarCollapsed ? '▶' : '◀ 折叠' }}
        </button>
      </div>
    </aside>

    <!-- 右侧主区域 -->
    <div class="main-area">
      <!-- 顶栏：面包屑 + 用户信息 -->
      <div class="topbar">
        <div class="breadcrumb">
          <span class="breadcrumb-home" @click="navigateTo('home')">首页</span>
          <span class="breadcrumb-sep">/</span>
          <span class="breadcrumb-current">{{ breadcrumbTitle }}</span>
        </div>
        <div class="topbar-user">
          <template v-if="isAuthenticated && currentUser">
            <span class="user-name-tag">{{ currentUser.displayName || currentUser.username }}</span>
            <span v-if="isAdmin" class="role-badge">管理员</span>
            <button class="topbar-btn" @click="openChangePasswordDialog" type="button">修改密码</button>
            <button class="topbar-btn logout" @click="logout" type="button">退出</button>
          </template>
          <template v-else>
            <button class="topbar-btn login" @click="openLoginDialog" type="button">登录</button>
          </template>
        </div>
      </div>

      <!-- 内容区 -->
      <div class="content-area">
        <transition name="page-fade" mode="out-in">

      <!-- 首页 -->
      <div v-if="currentPage === 'home'" key="home">
        <HomePanel />
      </div>

      <!-- 工作看板（登录后） -->
      <div v-else-if="currentPage === 'dashboard'" key="dashboard">
        <DashboardPanel />
      </div>

      <!-- 查询页 -->
      <div v-else-if="currentPage === 'query'" key="query">
        <div class="query-main-grid">
          <!-- 左侧元数据 -->
          <MetadataPanel
            :loading="metadataLoading"
            :keyword="metadataKeyword"
            :tables="filteredMetadataTables"
            :selected-table-name="selectedMetadataTable"
            :current-table-columns="filteredMetadataColumns"
            @update:keyword="metadataKeyword = $event"
            @search="loadQueryMetadata"
            @select-table="selectMetadataTable"
            @insert-table-name="queryText = queryText ? `${queryText} ${$event}` : $event"
            @insert-column-name="queryText = queryText ? `${queryText} ${$event}` : $event"
          />

          <!-- 中间查询面板 -->
          <QueryPanel
            v-model:db-type="queryDBType"
            v-model:connection-name="selectedConnectionName"
            v-model:sql-text="queryText"
            :connections="filteredConnections"
            :selected-connection="selectedConnection || null"
            :loading="queryLoading"
            :export-loading="exportLoading"
            :query-message="queryMessage"
            :metadata-tables="metadataTables"
            :metadata-columns="metadataColumns"
            @message="handleQueryPanelMessage"
            @execute="executeQuery"
            @cancel="cancelQuery"
            @export="exportQueryExcel"
            @clear="clearQuery"
            @refresh-metadata="loadQueryMetadata"
            @open-favorites="openSQLFavoritesView"
            @favorite-current-sql="openSQLFavoritesAdd"
          />

          <!-- 右侧查询历史 -->
          <QueryHistoryPanel
            :history-list="queryHistory"
            @apply-history="applyHistoryItem"
            @delete-history="deleteHistoryItem"
          />
        </div>

        <!-- 查询结果 -->
        <div class="card query-result-card">
          <h2>查询结果</h2>
          <p class="result">{{ queryMessage }}</p>

          <div v-if="queryColumns.length > 0" class="query-toolbar">
            <div class="query-toolbar-left">
              <span>共 {{ totalQueryRows }} 行</span>
              <span>每页显示</span>
              <select v-model="pageSize" class="page-size-select">
                <option v-for="size in pageSizeOptions" :key="size" :value="size">
                  {{ size }}
                </option>
              </select>
              <span>行</span>
            </div>

            <div class="query-toolbar-right">
              <button class="pager-btn" @click="goPrevPage" :disabled="currentPageNo <= 1" type="button">
                上一页
              </button>
              <span>第 {{ currentPageNo }} / {{ totalPages }} 页</span>
              <button class="pager-btn" @click="goNextPage" :disabled="currentPageNo >= totalPages" type="button">
                下一页
              </button>
            </div>
          </div>

          <div v-if="queryColumns.length > 0" class="table-wrap">
            <table class="result-table">
              <thead>
                <tr>
                  <th v-for="col in queryColumns" :key="col">{{ col }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(row, idx) in pagedQueryRows" :key="idx">
                  <td v-for="col in queryColumns" :key="col" :title="String(row[col] ?? '')">
                    {{ row[col] }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div v-else class="query-result-empty"></div>
        </div>
      </div>

      <!-- SQL 性能检测页 -->
      <div v-else-if="currentPage === 'query-plan'" key="query-plan">
        <div class="query-main-grid">
          <!-- 左侧元数据 -->
          <MetadataPanel
            :loading="metadataLoading"
            :keyword="metadataKeyword"
            :tables="filteredMetadataTables"
            :selected-table-name="selectedMetadataTable"
            :current-table-columns="filteredMetadataColumns"
            @update:keyword="metadataKeyword = $event"
            @search="loadQueryMetadata"
            @select-table="selectMetadataTable"
            @insert-table-name="queryText = queryText ? `${queryText} ${$event}` : $event"
            @insert-column-name="queryText = queryText ? `${queryText} ${$event}` : $event"
          />

          <!-- 中间检测面板 -->
          <QueryPlanPanel
            v-model:db-type="queryDBType"
            v-model:connection-name="selectedConnectionName"
            v-model:sql-text="queryText"
            :connections="filteredConnections"
            :selected-connection="selectedConnection || null"
            :loading="queryLoading"
            :query-message="queryMessage"
            :query-score="queryScore"
            :query-audit-id="queryAuditId"
            :query-columns="queryColumns || []"
            :query-rows="queryRows || []"
            :metadata-tables="metadataTables"
            :metadata-columns="metadataColumns"
            @message="handleQueryPanelMessage"
  @explain="explainQuery"
  @manual-explain="manualExplainQuery"
  @cancel="cancelQuery"
            @refresh-metadata="loadQueryMetadata"
            @open-audit-history="navigateTo('audit-history')"
          />

          <!-- 右侧查询历史 -->
          <QueryHistoryPanel
            :history-list="queryHistory"
            @apply-history="applyHistoryItem"
            @delete-history="deleteHistoryItem"
          />
        </div>

        <!-- 执行计划结果的 Teleport 挂载点 -->
        <div id="plan-result-teleport-target"></div>
      </div>

      <!-- SQL AI 审核历史页 -->
      <div v-else-if="currentPage === 'audit-history'" key="audit-history">
        <AuditHistoryPanel />
      </div>

      <!-- 数据库变更申请页 -->
      <div v-else-if="currentPage === 'db-change-requests'" key="db-change-requests">
        <DbChangeRequestPanel />
      </div>

      <!-- 数据库数据同步申请页 -->
      <div v-else-if="currentPage === 'db-data-sync-requests'" key="db-data-sync-requests">
        <DbDataSyncRequestPanel />
      </div>

      <!-- 管理员页面：用户管理 -->
      <div v-else-if="currentPage === 'admin-users'" key="admin-users">
        <AdminUserPanel />
      </div>

      <!-- 管理员页面：数据库管理 -->
      <div v-else-if="currentPage === 'admin-connections'" key="admin-connections">
        <AdminConnectionPanel />
      </div>

      <!-- 管理员页面：SQL 审核管理 -->
      <div v-else-if="currentPage === 'admin-audits'" key="admin-audits">
        <AdminAuditPanel />
      </div>

      <!-- 管理员页面：数据库变更 -->
      <div v-else-if="currentPage === 'admin-db-change-release'" key="admin-db-change-release">
        <AdminDbChangeReleasePanel />
      </div>

      <!-- 管理员页面：数据同步管理 -->
      <div v-else-if="currentPage === 'admin-db-data-sync-requests'" key="admin-db-data-sync-requests">
        <AdminDbDataSyncRequestPanel />
      </div>

      <!-- 管理员页面：团队数据库环境 -->
      <div v-else-if="currentPage === 'admin-team-db-envs'" key="admin-team-db-envs">
        <AdminTeamDbEnvPanel />
      </div>

      <!-- 管理员页面：数据库告警处理 -->
      <div v-else-if="currentPage === 'admin-db-alert-handles'" key="admin-db-alert-handles">
        <AdminDbAlertHandlePanel />
      </div>

      <!-- 管理员页面：运维变更记录 -->
      <div v-else-if="currentPage === 'admin-ops-change-records'" key="admin-ops-change-records">
        <AdminOpsChangePanel />
      </div>

      <!-- 管理员页面：个人看板 -->
      <div v-else-if="currentPage === 'personal-dashboard'" key="personal-dashboard">
        <PersonalDashboardPanel @navigate="(p: string) => navigateTo(p as any)" />
      </div>
        </transition>
      </div><!-- content-area -->
    </div><!-- main-area -->

    <!-- 登录弹窗 -->
    <LoginDialog
      :visible="loginDialogVisible"
      :loading="loginLoading"
      :message="loginMessage"
      :username="loginForm.username"
      :password="loginForm.password"
      @update:username="loginForm.username = $event"
      @update:password="loginForm.password = $event"
      @submit="submitLogin"
      @sso-success="handleSsoSuccess"
      @close="closeLoginDialog"
    />

    <ChangePasswordDialog
      ref="changePasswordDialogRef"
      :visible="changePasswordDialogVisible"
      :is-force="isForceChangePwd"
      @submit="handleChangePasswordSubmit"
      @cancel="changePasswordDialogVisible = false"
    />

    <!-- SQL 收藏弹窗 -->
    <SqlFavoritePanel
      :visible="sqlFavoriteVisible"
      :mode="sqlFavoriteMode"
      :current-db-type="queryDBType"
      :current-connection-name="selectedConnectionName"
      :current-sql-text="queryText"
      :connections="queryConnections"
      @close="closeSQLFavorites"
      @apply-favorite="handleApplyFavorite"
      @message="handleFavoriteMessage"
    />

    <!-- 回到顶部 -->
    <button class="back-top-btn" @click="scrollToTop" type="button">
      回到顶部
    </button>

    <!-- 全局 Toast 通知 -->
    <div class="toast-container">
      <div
        v-for="t in useToasts().value"
        :key="t.id"
        :class="['toast-item', `toast-${t.type}`]"
      >
        {{ t.message }}
      </div>
    </div>

    <!-- 全局 Confirm 确认弹窗 -->
    <div v-if="useConfirm().value.visible" class="confirm-overlay" @click.self="resolveConfirm(false)">
      <div class="confirm-box">
        <h3>{{ useConfirm().value.title }}</h3>
        <p class="confirm-message">{{ useConfirm().value.message }}</p>
        <div class="confirm-actions">
          <button class="action-btn secondary-btn" @click="resolveConfirm(false)" type="button">取消</button>
          <button class="action-btn danger-btn" @click="resolveConfirm(true)" type="button">确定</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
:global(html),
:global(body),
:global(#app) {
  margin: 0;
  padding: 0;
  width: 100%;
  min-width: 100%;
  min-height: 100%;
  background: #f0f2f5;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Arial, sans-serif;
  font-size: 14px;
  color: #303133;
}

:global(*) {
  box-sizing: border-box;
}

/* ========== Zabbix 风格布局 ========== */
.app-layout {
  display: flex;
  min-height: 100vh;
  background: #f0f2f5;
}

/* 左侧导航栏 */
.sidebar {
  width: 220px;
  flex-shrink: 0;
  background: #1f2937;
  color: #d1d5db;
  display: flex;
  flex-direction: column;
  transition: width 0.2s;
  position: sticky;
  top: 0;
  height: 100vh;
  overflow-y: auto;
}
.sidebar.collapsed {
  width: 56px;
}

.sidebar-brand {
  padding: 16px;
  font-size: 16px;
  font-weight: bold;
  color: #fff;
  text-align: center;
  border-bottom: 1px solid #374151;
  white-space: nowrap;
  overflow: hidden;
}

.sidebar-nav {
  flex: 1;
  padding: 8px 0;
}

.nav-group-label {
  padding: 8px 16px 4px;
  font-size: 11px;
  color: #6b7280;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 8px 16px;
  background: transparent;
  border: none;
  color: #d1d5db;
  font-size: 14px;
  cursor: pointer;
  text-align: left;
  transition: background 0.15s;
  white-space: nowrap;
  overflow: hidden;
}
.nav-item:hover {
  background: #374151;
}
.nav-item.active {
  background: #3b82f6;
  color: #fff;
}
.nav-badge {
  display: inline-block;
  min-width: 18px;
  padding: 1px 6px;
  border-radius: 9px;
  background: #f56c6c;
  color: #fff;
  font-size: 11px;
  font-weight: bold;
  text-align: center;
  margin-left: auto;
  flex-shrink: 0;
}
.nav-icon {
  flex-shrink: 0;
  width: 20px;
  text-align: center;
}

.sidebar-footer {
  border-top: 1px solid #374151;
  padding: 8px;
}
.sidebar-toggle-btn {
  width: 100%;
  padding: 6px;
  background: transparent;
  border: none;
  color: #6b7280;
  cursor: pointer;
  font-size: 13px;
}
.sidebar-toggle-btn:hover {
  color: #d1d5db;
}

/* 右侧主区域 */
.main-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

/* 顶栏 */
.topbar {
  height: 48px;
  background: #fff;
  border-bottom: 1px solid #e4e7ed;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  position: sticky;
  top: 0;
  z-index: 100;
  flex-shrink: 0;
}

.breadcrumb {
  font-size: 14px;
  color: #909399;
  display: flex;
  align-items: center;
  gap: 8px;
}
.breadcrumb-home {
  cursor: pointer;
  color: #606266;
}
.breadcrumb-home:hover {
  color: #409eff;
}
.breadcrumb-sep {
  color: #c0c4cc;
}
.breadcrumb-current {
  color: #303133;
  font-weight: 500;
}

.topbar-user {
  display: flex;
  align-items: center;
  gap: 8px;
}
.user-name-tag {
  font-size: 14px;
  color: #303133;
  font-weight: 500;
}
.role-badge {
  font-size: 11px;
  padding: 2px 6px;
  background: #f56c6c;
  color: #fff;
  border-radius: 3px;
}
.topbar-btn {
  padding: 4px 12px;
  font-size: 13px;
  background: transparent;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  color: #606266;
  cursor: pointer;
}
.topbar-btn:hover {
  color: #409eff;
  border-color: #409eff;
}
.topbar-btn.logout:hover {
  color: #f56c6c;
  border-color: #f56c6c;
}
.topbar-btn.login {
  background: #409eff;
  color: #fff;
  border-color: #409eff;
}

/* 内容区 */
.content-area {
  flex: 1;
  padding: 16px;
  overflow-y: auto;
}

/* 页面切换过渡动画 */
.page-fade-enter-active {
  transition: opacity 0.2s ease;
}
.page-fade-leave-active {
  transition: opacity 0.15s ease;
}
.page-fade-enter-from,
.page-fade-leave-to {
  opacity: 0;
}

/* 查询页三栏布局 */
.query-main-grid {
  display: grid;
  grid-template-columns: 320px minmax(860px, 1fr) 260px;
  gap: 16px;
  align-items: stretch;
  margin-bottom: 16px;
}

/* 查询结果 */
.query-result-card {
  width: 100%;
  margin-bottom: 24px;
}
.query-result-empty {
  min-height: 160px;
}
.query-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}
.query-toolbar-left,
.query-toolbar-right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.page-size-select {
  width: auto;
  min-width: 80px;
  padding: 6px 10px;
  font-size: 14px;
}
.pager-btn {
  padding: 8px 14px;
  font-size: 14px;
  cursor: pointer;
  border: none;
  border-radius: 6px;
  background: #409eff;
  color: #fff;
}
.pager-btn:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

/* 回到顶部 */
.back-top-btn {
  position: fixed;
  right: 24px;
  bottom: 24px;
  z-index: 999;
  padding: 12px 16px;
  border: none;
  border-radius: 999px;
  background: #409eff;
  color: #fff;
  font-size: 14px;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.back-top-btn:hover {
  background: #2f86d6;
}

/* ========== 全局 Toast ========== */
.toast-container {
  position: fixed;
  top: 60px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 9999;
  display: flex;
  flex-direction: column;
  gap: 8px;
  pointer-events: none;
}
.toast-item {
  padding: 10px 24px;
  border-radius: 6px;
  color: #fff;
  font-size: 14px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.15);
  animation: toast-slide 0.3s ease;
}
.toast-success { background: #67c23a; }
.toast-error { background: #f56c6c; }
.toast-warning { background: #e6a23c; }
.toast-info { background: #909399; }
@keyframes toast-slide {
  from { opacity: 0; transform: translateY(-10px); }
  to { opacity: 1; transform: translateY(0); }
}

/* ========== 全局 Confirm ========== */
.confirm-overlay {
  position: fixed;
  inset: 0;
  z-index: 9998;
  background: rgba(0,0,0,0.45);
  display: flex;
  align-items: center;
  justify-content: center;
}
.confirm-box {
  background: #fff;
  border-radius: 8px;
  width: 90%;
  max-width: 420px;
  padding: 24px;
  box-shadow: 0 8px 24px rgba(0,0,0,0.2);
}
.confirm-box h3 {
  margin: 0 0 12px;
  font-size: 18px;
  color: #303133;
}
.confirm-message {
  margin: 0 0 20px;
  font-size: 15px;
  color: #606266;
  line-height: 1.6;
  white-space: pre-wrap;
}
.confirm-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}
</style>
