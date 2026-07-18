<script setup lang="ts">
/**
 * AdminConnectionPanel.vue
 * ------------------------------------------------------------------
 * 该组件用于"查询数据库管理"页面。
 *
 * 布局模式：默认进入配置列表，右上角「新增连接」切换到表单视图。
 *
 * 主要功能：
 * 1. 展示数据库连接配置列表
 * 2. 新增数据库连接配置
 * 3. 编辑数据库连接配置
 * 4. 删除数据库连接配置
 *
 * 本版本支持：
 * 1. 切换数据库类型时自动调整端口
 *    - mysql  -> 3306
 *    - oracle -> 1521
 *
 * 2. 根据数据库类型自动显示对应字段
 *    - mysql  -> 显示"MySQL 数据库名"
 *    - oracle -> 显示"Oracle 服务名"
 *
 * 说明：
 * 1. 编辑模式下，连接名称 name 可以修改，后端会级联更新关联表
 * 2. 编辑模式下，密码可以留空，表示不修改密码
 * 3. 删除连接时，如果后端检测到该连接仍被用户权限引用，会返回错误提示
 */

import { computed, onMounted, ref, watch } from 'vue'
/**
 * 单条数据库连接配置
 */
interface AdminConnectionItem {
  id: number
  name: string
  dbType: string
  host: string
  port: number
  username: string
  databaseName: string
  serviceName: string
  isEnabled: number
  canConnect: number
  createTime: string
  updateTime: string
}

const loading = ref(false)
const message = ref('')

/**
 * 视图模式：list 配置列表 / form 新增/编辑
 */
const viewMode = ref<'list' | 'form'>('list')

/**
 * 当前是否编辑模式
 */
const isEditMode = ref(false)

const connections = ref<AdminConnectionItem[]>([])

/**
 * 名称搜索
 */
const searchName = ref('')

const filteredConnections = computed(() => {
  const keyword = searchName.value.trim().toLowerCase()
  if (!keyword) return connections.value
  return connections.value.filter(c => c.name.toLowerCase().includes(keyword))
})

/**
 * 分页
 */
const currentPage = ref(1)
const pageSize = ref(10)
const pageSizeOptions = [10, 20, 50]

const totalPages = computed(() => {
  if (filteredConnections.value.length === 0) return 1
  return Math.ceil(filteredConnections.value.length / pageSize.value)
})

const pagedConnections = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredConnections.value.slice(start, start + pageSize.value)
})

const handlePageSizeChange = () => {
  currentPage.value = 1
}

const handleSearch = () => {
  currentPage.value = 1
}

const goPrevPage = () => {
  if (currentPage.value > 1) {
    currentPage.value--
  }
}

const goNextPage = () => {
  if (currentPage.value < totalPages.value) {
    currentPage.value++
  }
}

/**
 * 创建 / 编辑连接使用的表单
 */
const form = ref({
  id: 0,
  name: '',
  dbType: 'mysql',
  host: '',
  port: 3306,
  username: '',
  password: '',
  databaseName: '',
  serviceName: '',
  isEnabled: 1,
  canConnect: 0,
})

const isMySQL = computed(() => form.value.dbType === 'mysql')
const isOracle = computed(() => form.value.dbType === 'oracle')

/**
 * 根据数据库类型，自动切换默认端口，并清空无关字段
 */
const applyDBTypeDefaults = (dbType: string) => {
  if (dbType === 'oracle') {
    form.value.port = 1521
    form.value.databaseName = ''
    return
  }

  if (dbType === 'mysql') {
    form.value.port = 3306
    form.value.serviceName = ''
  }
}

const handleDBTypeChange = () => {
  applyDBTypeDefaults(form.value.dbType)
}

watch(
  () => form.value.dbType,
  (newDBType, oldDBType) => {
    if (newDBType !== oldDBType) {
      applyDBTypeDefaults(newDBType)
    }
  },
)

const resetForm = () => {
  isEditMode.value = false
  form.value = {
    id: 0,
    name: '',
    dbType: 'mysql',
    host: '',
    port: 3306,
    username: '',
    password: '',
    databaseName: '',
    serviceName: '',
    isEnabled: 1,
    canConnect: 0,
  }
}

/**
 * 进入新增视图
 */
const showCreateForm = () => {
  resetForm()
  message.value = '请填写数据库连接配置信息'
  viewMode.value = 'form'
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

/**
 * 返回列表视图
 */
const backToList = () => {
  viewMode.value = 'list'
  message.value = ''
  loadConnections()
}

/**
 * 加载数据库连接配置列表
 */
const loadConnections = async () => {
  loading.value = true
  message.value = ''

  try {
    const res = await fetch('/api/admin/db-connections', {
      credentials: 'include',
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || '加载连接配置失败'
      connections.value = []
      return
    }

    connections.value = data.connections || []
    message.value = '连接配置列表加载成功'
  } catch (err) {
    console.error(err)
    message.value = '加载连接配置失败，请检查后端是否启动'
    connections.value = []
  } finally {
    loading.value = false
  }
}

/**
 * 自定义确认弹窗状态
 */
const confirmDialog = ref({
  visible: false,
  message: '',
  resolve: null as ((value: boolean) => void) | null,
})

const showCustomConfirm = (msg: string): Promise<boolean> => {
  return new Promise((resolve) => {
    confirmDialog.value.message = msg
    confirmDialog.value.resolve = resolve
    confirmDialog.value.visible = true
  })
}

const handleConfirmYes = () => {
  if (confirmDialog.value.resolve) {
    confirmDialog.value.resolve(true)
  }
  confirmDialog.value.visible = false
}

const handleConfirmNo = () => {
  if (confirmDialog.value.resolve) {
    confirmDialog.value.resolve(false)
  }
  confirmDialog.value.visible = false
}

/**
 * 测试数据库连接
 * 如果连接失败，会弹出 confirm 询问用户是否继续保存。
 * 返回 true 表示继续保存，false 表示取消保存。
 */
const testConnectionBeforeSave = async (): Promise<boolean> => {
  // 不可连接的库，跳过测试直接允许保存
  if (form.value.canConnect === 0) {
    return true
  }

  loading.value = true
  message.value = '正在测试端口连接...'

  try {
    const res = await fetch('/api/admin/db-connections/test', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(form.value),
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      const errorType = data.errorType || ''

      if (errorType === 'port') {
        // 端口不通 → 允许强制保存
        message.value = data.message || '端口无法连接'
        return await showCustomConfirm(`${data.message}\n\n是否仍要强制保存？`)
      } else {
        // DB 认证失败等 → 不允许保存，保留具体错误信息
        message.value = data.message || '连接测试失败'
        return false
      }
    }

    return true
  } catch (err) {
    console.error(err)
    return await showCustomConfirm('测试连接请求异常，后端可能未启动。\n是否仍要继续保存？')
  } finally {
    loading.value = false
  }
}

/**
 * 创建连接配置
 */
const createConnection = async () => {
  const canSave = await testConnectionBeforeSave()
  if (!canSave) {
    return
  }

  loading.value = true
  message.value = ''

  try {
    const res = await fetch('/api/admin/db-connections', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(form.value),
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || '创建连接配置失败'
      return
    }

    message.value = data.message || '创建连接配置成功'
    backToList()
  } catch (err) {
    console.error(err)
    message.value = '创建连接配置失败，请检查后端是否启动'
  } finally {
    loading.value = false
  }
}

/**
 * 进入编辑视图
 */
const startEdit = (item: AdminConnectionItem) => {
  isEditMode.value = true
  form.value = {
    id: item.id,
    name: item.name,
    dbType: item.dbType,
    host: item.host,
    port: item.port,
    username: item.username,
    password: '',
    databaseName: item.databaseName || '',
    serviceName: item.serviceName || '',
    isEnabled: item.isEnabled,
    canConnect: item.canConnect,
  }
  message.value = `正在编辑连接：${item.name}`
  viewMode.value = 'form'
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

/**
 * 克隆连接（进入新增视图，回填数据但 id=0、name 清空）
 */
const cloneConnection = (item: AdminConnectionItem) => {
  isEditMode.value = false
  form.value = {
    id: 0,
    name: '',
    dbType: item.dbType,
    host: item.host,
    port: item.port,
    username: item.username,
    password: '',
    databaseName: item.databaseName || '',
    serviceName: item.serviceName || '',
    isEnabled: item.isEnabled,
    canConnect: item.canConnect,
  }
  message.value = `正在克隆连接：${item.name}，请修改连接名称后点击"创建连接配置"`
  viewMode.value = 'form'
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

/**
 * 保存编辑
 */
const updateConnection = async () => {
  const canSave = await testConnectionBeforeSave()
  if (!canSave) {
    return
  }

  loading.value = true
  message.value = ''

  try {
    const res = await fetch('/api/admin/db-connections', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(form.value),
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || '编辑连接配置失败'
      return
    }

    message.value = data.message || '编辑连接配置成功'
    backToList()
  } catch (err) {
    console.error(err)
    message.value = '编辑连接配置失败，请检查后端是否启动'
  } finally {
    loading.value = false
  }
}

/**
 * 删除连接配置
 */
const deleteConnection = async (item: AdminConnectionItem) => {
  const ok = window.confirm(`确定要删除连接【${item.name}】吗？`)
  if (!ok) return

  loading.value = true
  message.value = ''

  try {
    const res = await fetch('/api/admin/db-connections', {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({
        id: item.id,
      }),
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || '删除连接配置失败'
      return
    }

    message.value = data.message || '删除连接配置成功'
    await loadConnections()
  } catch (err) {
    console.error(err)
    message.value = '删除连接配置失败，请检查后端是否启动'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadConnections()
})
</script>

<template>
  <div class="admin-page">
    <!-- ============ 配置列表视图 ============ -->
    <div v-if="viewMode === 'list'" class="card table-card">
      <div class="table-header">
        <h2>查询数据库列表 (总数: {{ filteredConnections.length }})</h2>
        <div class="header-actions">
          <input
            v-model="searchName"
            @input="handleSearch"
            class="search-input"
            placeholder="按名称搜索..."
          />
          <button class="action-btn primary-btn" :disabled="loading" @click="showCreateForm" type="button">
            + 新增连接
          </button>
        </div>
      </div>

      <p class="result">{{ message }}</p>

      <div class="table-wrap">
        <table class="result-table">
          <colgroup>
            <col style="width: 14%" />
            <col style="width: 5%" />
            <col style="width: 10%" />
            <col style="width: 5%" />
            <col style="width: 12%" />
            <col style="width: 9%" />
            <col style="width: 9%" />
            <col style="width: 5%" />
            <col style="width: 7%" />
            <col style="width: 14%" />
          </colgroup>
          <thead>
            <tr>
              <th>连接名称</th>
              <th>类型</th>
              <th>主机</th>
              <th>端口</th>
              <th>用户名</th>
              <th>MySQL数据库名</th>
              <th>Oracle服务名</th>
              <th>是否启用</th>
              <th>可连接</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in pagedConnections" :key="item.id" class="data-row" @dblclick="startEdit(item)">
              <td>{{ item.name }}</td>
              <td>{{ item.dbType }}</td>
              <td>{{ item.host }}</td>
              <td>{{ item.port }}</td>
              <td>{{ item.username }}</td>
              <td>{{ item.databaseName }}</td>
              <td>{{ item.serviceName }}</td>
              <td>
                <span :class="['enable-tag', item.isEnabled === 1 ? 'enable-on' : 'enable-off']">
                  {{ item.isEnabled === 1 ? '启用' : '禁用' }}
                </span>
              </td>
              <td>
                <span :class="['enable-tag', item.canConnect === 1 ? 'enable-on' : 'enable-off']">
                  {{ item.canConnect === 1 ? '可连接' : '不可连接' }}
                </span>
              </td>
              <td>
                <div class="row-btns">
                  <button class="mini-btn edit-btn" @click="startEdit(item)" type="button">编辑</button>
                  <button class="mini-btn clone-btn" @click="cloneConnection(item)" type="button">克隆</button>
                  <button class="mini-btn delete-btn" @click="deleteConnection(item)" type="button">删除</button>
                </div>
              </td>
            </tr>
            <tr v-if="filteredConnections.length === 0">
              <td colspan="10" class="empty-text">暂无连接配置数据</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="pagination">
        <div class="page-size">
          共 {{ filteredConnections.length }} 条，每页
          <select v-model="pageSize" @change="handlePageSizeChange">
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
    </div>

    <!-- ============ 新增/编辑视图 ============ -->
    <div v-else class="card form-card">
      <div class="form-header">
        <h2>{{ isEditMode ? '编辑数据库连接' : '新增数据库连接' }}</h2>
        <button class="action-btn secondary-btn back-btn" @click="backToList" type="button">
          ← 返回列表
        </button>
      </div>
      <p class="result">{{ message }}</p>

      <div class="form-grid">
        <!-- 第一行：连接名称 / 数据库类型 / 主机 -->
        <div class="form-item">
          <label>连接名称</label>
          <input v-model="form.name" placeholder="例如：mysql-dev" />
        </div>
        <div class="form-item">
          <label>数据库类型</label>
          <select v-model="form.dbType" @change="handleDBTypeChange">
            <option value="mysql">mysql</option>
            <option value="oracle">oracle</option>
          </select>
        </div>

        <!-- 第二行：主机 / 端口 / 用户名 -->
        <div class="form-item">
          <label>主机</label>
          <input v-model="form.host" placeholder="例如：127.0.0.1" />
        </div>
        <div class="form-item">
          <label>{{ isOracle ? 'Oracle 端口' : 'MySQL 端口' }}</label>
          <input
            v-model.number="form.port"
            type="number"
            :placeholder="isOracle ? '请输入服务端口，默认 1521' : '请输入数据库端口，默认 3306'"
          />
        </div>
        <div class="form-item">
          <label>用户名</label>
          <input v-model="form.username" placeholder="请输入数据库用户名" />
        </div>

        <!-- 第三行：密码 / 数据库名或服务名 / 是否启用 -->
        <div class="form-item">
          <label>{{ isEditMode ? '密码（留空表示不修改）' : '密码' }}</label>
          <input
            v-model="form.password"
            type="password"
            :placeholder="isEditMode ? '留空表示不修改密码' : '请输入数据库密码'"
          />
        </div>
        <div class="form-item" v-if="isMySQL">
          <label>MySQL 数据库名</label>
          <input v-model="form.databaseName" placeholder="请输入数据库名" />
        </div>
        <div class="form-item" v-if="isOracle">
          <label>Oracle 服务名</label>
          <input v-model="form.serviceName" placeholder="请输入服务名" />
        </div>
        <div class="form-item">
          <label>是否启用</label>
          <select v-model="form.isEnabled">
            <option :value="1">启用</option>
            <option :value="0">禁用</option>
          </select>
        </div>
        <div class="form-item">
          <label>是否可连接</label>
          <select v-model="form.canConnect">
            <option :value="1">可连接</option>
            <option :value="0">不可连接</option>
          </select>
        </div>
      </div>

      <div class="btn-row">
        <button
          v-if="!isEditMode"
          class="action-btn primary-btn"
          :disabled="loading"
          @click="createConnection"
          type="button"
        >
          {{ loading ? '处理中...' : '创建连接配置' }}
        </button>
        <template v-else>
          <button
            class="action-btn primary-btn"
            :disabled="loading"
            @click="updateConnection"
            type="button"
          >
            {{ loading ? '保存中...' : '保存编辑' }}
          </button>
        </template>
        <button class="action-btn warning-btn" :disabled="loading" @click="backToList" type="button">
          取消
        </button>
      </div>
    </div>

    <!-- 自定义确认弹窗 -->
    <div v-if="confirmDialog.visible" class="modal-overlay">
      <div class="modal-container">
        <div class="modal-header">
          <h3>警告</h3>
        </div>
        <div class="modal-body">
          <p class="modal-message">{{ confirmDialog.message }}</p>
        </div>
        <div class="modal-footer">
          <button class="action-btn warning-btn" @click="handleConfirmYes">强制保存</button>
          <button class="action-btn secondary-btn" @click="handleConfirmNo">取消</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
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

h2 {
  margin-top: 0;
  color: #2c3e50;
}

.result {
  margin-bottom: 12px;
  font-size: 15px;
  color: #666;
}

/**
 * 表单采用三列布局
 */
.form-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(200px, 1fr));
  gap: 16px;
  margin-bottom: 16px;
}

.form-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.form-item label {
  font-size: 14px;
  color: #333;
}

input,
select {
  width: 100%;
  padding: 10px 12px;
  font-family: Consolas, Monaco, monospace;
  font-size: 16px;
  border: 1px solid #ccc;
  border-radius: 6px;
  background: #fff;
}

.btn-row {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.action-btn {
  padding: 10px 20px;
  font-size: 16px;
  cursor: pointer;
  border: none;
  border-radius: 6px;
  color: #fff;
}

.primary-btn {
  background: #409eff;
}

.secondary-btn {
  background: #909399;
}

.warning-btn {
  background: #e6a23c;
}

/**
 * 表格容器
 */
.table-wrap {
  width: 100%;
  overflow-x: hidden;
}

.result-table {
  width: 100%;
  max-width: 100%;
  border-collapse: collapse;
  background: #fff;
  table-layout: fixed;
  box-sizing: border-box;
}

.result-table th,
.result-table td {
  border: 1px solid #ddd;
  padding: 8px;
  text-align: left;
  vertical-align: top;
  word-break: break-all;
  overflow-wrap: break-word;
}

.result-table th {
  background: #f5f7fa;
  white-space: nowrap;
}

/**
 * 数据行支持双击进入编辑
 */
.data-row {
  cursor: pointer;
}

.data-row:hover {
  background: #f5f7fa;
}

/**
 * 启用/禁用标签
 */
.enable-tag {
  display: inline-block;
  padding: 2px 10px;
  border-radius: 10px;
  font-size: 13px;
  color: #fff;
}

.enable-on {
  background: #67c23a;
}

.enable-off {
  background: #909399;
}

/**
 * 行内操作按钮
 */
.row-btns {
  display: flex;
  gap: 6px;
  flex-wrap: nowrap;
  white-space: nowrap;
}

.mini-btn {
  padding: 6px 12px;
  font-size: 14px;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  color: #fff;
  line-height: 1.4;
}

.edit-btn {
  background: #409eff;
}

.clone-btn {
  background: #e6a23c;
}

.delete-btn {
  background: #f56c6c;
}

/**
 * 列表头部
 */
.table-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 12px;
}

.table-header h2 {
  margin-bottom: 0;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.search-input {
  width: 200px;
  padding: 8px 12px;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  font-size: 14px;
  font-family: Consolas, Monaco, monospace;
}

.search-input:focus {
  border-color: #409eff;
  outline: none;
}

/**
 * 表单视图头部
 */
.form-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.back-btn {
  white-space: nowrap;
}

.empty-text {
  text-align: center;
  color: #999;
  padding: 24px;
}

/**
 * 分页
 */
.pagination {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 14px;
  color: #606266;
  margin-top: 20px;
  flex-wrap: wrap;
  gap: 12px;
}

.page-size {
  display: flex;
  align-items: center;
  gap: 6px;
}

.page-size select {
  width: auto;
  padding: 4px 8px;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  font-size: 14px;
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
  font-size: 14px;
}

.pager-btn:disabled {
  background: #f5f7fa;
  color: #c0c4cc;
  cursor: not-allowed;
}

/**
 * 自定义弹窗样式
 */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 9999;
}

.modal-container {
  background-color: #fff;
  border-radius: 8px;
  width: 90%;
  max-width: 450px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  overflow: hidden;
  animation: modal-fade-in 0.3s ease-out;
}

.modal-header {
  padding: 16px 20px;
  background-color: #fef0f0;
  border-bottom: 1px solid #fde2e2;
}

.modal-header h3 {
  margin: 0;
  color: #f56c6c;
  font-size: 18px;
}

.modal-body {
  padding: 24px 20px;
}

.modal-message {
  margin: 0;
  color: #606266;
  font-size: 15px;
  line-height: 1.6;
  white-space: pre-wrap;
}

.modal-footer {
  padding: 16px 20px;
  border-top: 1px solid #ebeef5;
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

@keyframes modal-fade-in {
  from {
    opacity: 0;
    transform: translateY(-20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
