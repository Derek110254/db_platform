<script setup lang="ts">
/**
 * AdminConnectionPanel.vue
 * ------------------------------------------------------------------
 * 该组件用于“数据库管理”页面。
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
 *    - mysql  -> 显示“MySQL 数据库名”
 *    - oracle -> 显示“Oracle 服务名”
 *
 * 说明：
 * 1. 编辑模式下，连接名称 name 不允许修改
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
  label: string
  dbType: string
  host: string
  port: number
  username: string
  databaseName: string
  serviceName: string
  isEnabled: number
  createTime: string
  updateTime: string
}

/**
 * 页面加载状态
 */
const loading = ref(false)

/**
 * 页面提示消息
 */
const message = ref('')

/**
 * 当前是否编辑模式
 *
 * false：新增连接
 * true：编辑已有连接
 */
const isEditMode = ref(false)

/**
 * 连接配置列表
 */
const connections = ref<AdminConnectionItem[]>([])

/**
 * 创建 / 编辑连接使用的表单
 */
const form = ref({
  id: 0,
  name: '',
  label: '',
  dbType: 'mysql',
  host: '',
  port: 3306,
  username: '',
  password: '',
  databaseName: '',
  serviceName: '',
  isEnabled: 1,
})

/**
 * 当前是否 MySQL
 */
const isMySQL = computed(() => form.value.dbType === 'mysql')

/**
 * 当前是否 Oracle
 */
const isOracle = computed(() => form.value.dbType === 'oracle')

/**
 * 根据数据库类型，自动切换默认端口，并清空无关字段
 *
 * 规则：
 * 1. mysql：
 *    - 默认端口 3306
 *    - 清空 Oracle 服务名
 *
 * 2. oracle：
 *    - 默认端口 1521
 *    - 清空 MySQL 数据库名
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

/**
 * 数据库类型切换事件
 *
 * 这里直接在下拉框 change 时处理，
 * 能保证用户手动切换类型时，端口立即更新。
 */
const handleDBTypeChange = () => {
  applyDBTypeDefaults(form.value.dbType)
}

/**
 * 兜底监听：
 * 如果 dbType 不是通过下拉 change 改变，而是其它逻辑改动，
 * 也能自动同步端口和字段。
 */
watch(
  () => form.value.dbType,
  (newDBType, oldDBType) => {
    // 当值真的发生变化时再处理，避免无意义重复设置
    if (newDBType !== oldDBType) {
      applyDBTypeDefaults(newDBType)
    }
  },
)

/**
 * 重置表单
 *
 * 用于：
 * 1. 新增成功后恢复初始状态
 * 2. 编辑取消后恢复初始状态
 */
const resetForm = () => {
  isEditMode.value = false
  form.value = {
    id: 0,
    name: '',
    label: '',
    dbType: 'mysql',
    host: '',
    port: 3306,
    username: '',
    password: '',
    databaseName: '',
    serviceName: '',
    isEnabled: 1,
  }
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
  loading.value = true
  message.value = '正在测试连接...'
  
  try {
    const res = await fetch('/api/admin/db-connections/test', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(form.value),
    })
    const data = await res.json()
    
    if (!res.ok || !data.ok) {
      // 连接失败，弹出警告
      return await showCustomConfirm(`连接测试失败：\n${data.message || '未知错误'}\n\n是否仍要继续保存？`)
    }
    
    return true // 测试成功，继续保存
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
    message.value = '已取消保存'
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
    resetForm()
    await loadConnections()
  } catch (err) {
    console.error(err)
    message.value = '创建连接配置失败，请检查后端是否启动'
  } finally {
    loading.value = false
  }
}

/**
 * 进入编辑模式
 *
 * 说明：
 * 1. name 不允许修改，因此编辑模式下会禁用连接名称输入框
 * 2. password 默认清空，表示“留空则不修改密码”
 * 3. 这里保留原始端口，不自动改端口
 *    只有用户后续手动切换 dbType 时，才会切换默认端口
 */
const startEdit = (item: AdminConnectionItem) => {
  isEditMode.value = true
  form.value = {
    id: item.id,
    name: item.name,
    label: item.label,
    dbType: item.dbType,
    host: item.host,
    port: item.port,
    username: item.username,
    password: '',
    databaseName: item.databaseName || '',
    serviceName: item.serviceName || '',
    isEnabled: item.isEnabled,
  }
  message.value = `正在编辑连接：${item.name}`
}

/**
 * 保存编辑
 */
const updateConnection = async () => {
  const canSave = await testConnectionBeforeSave()
  if (!canSave) {
    message.value = '已取消编辑'
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
    resetForm()
    await loadConnections()
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
  const ok = window.confirm(`确定要删除连接【${item.label} / ${item.name}】吗？`)
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

    // 如果删除的是当前正在编辑的连接，则重置表单
    if (isEditMode.value && form.value.id === item.id) {
      resetForm()
    }

    await loadConnections()
  } catch (err) {
    console.error(err)
    message.value = '删除连接配置失败，请检查后端是否启动'
  } finally {
    loading.value = false
  }
}

/**
 * 页面挂载时自动加载连接列表
 */
onMounted(() => {
  loadConnections()
})
</script>

<template>
  <div class="admin-page">
    <!-- 表单区域：新增 / 编辑 -->
    <div class="card form-card">
      <h2>{{ isEditMode ? '编辑数据库连接' : '数据库管理' }}</h2>
      <p class="result">{{ message }}</p>

      <div class="form-grid">
        <!-- 连接名称 -->
        <div class="form-item">
          <label>连接名称</label>
          <input
            v-model="form.name"
            :disabled="isEditMode"
            placeholder="例如：mysql-dev"
          />
        </div>

        <!-- 展示名称 -->
        <div class="form-item">
          <label>展示名称</label>
          <input
            v-model="form.label"
            placeholder="例如：MySQL开发库"
          />
        </div>

        <!-- 数据库类型 -->
        <div class="form-item">
          <label>数据库类型</label>
          <select v-model="form.dbType" @change="handleDBTypeChange">
            <option value="mysql">mysql</option>
            <option value="oracle">oracle</option>
          </select>
        </div>

        <!-- 主机 -->
        <div class="form-item">
          <label>主机</label>
          <input
            v-model="form.host"
            placeholder="例如：127.0.0.1"
          />
        </div>

        <!-- 端口：根据数据库类型自动切换标题和提示 -->
        <div class="form-item">
          <label>{{ isOracle ? 'Oracle 端口' : 'MySQL 端口' }}</label>
          <input
            v-model.number="form.port"
            type="number"
            :placeholder="isOracle ? '请输入服务端口，默认 1521' : '请输入数据库端口，默认 3306'"
          />
        </div>

        <!-- 用户名 -->
        <div class="form-item">
          <label>用户名</label>
          <input
            v-model="form.username"
            placeholder="请输入数据库用户名"
          />
        </div>

        <!-- 密码 -->
        <div class="form-item">
          <label>
            {{ isEditMode ? '密码（留空表示不修改）' : '密码' }}
          </label>
          <input
            v-model="form.password"
            type="password"
            :placeholder="isEditMode ? '留空表示不修改密码' : '请输入数据库密码'"
          />
        </div>

        <!-- MySQL 数据库名 -->
        <div class="form-item" v-if="isMySQL">
          <label>MySQL 数据库名</label>
          <input
            v-model="form.databaseName"
            placeholder="请输入数据库名"
          />
        </div>

        <!-- Oracle 服务名 -->
        <div class="form-item" v-if="isOracle">
          <label>Oracle 服务名</label>
          <input
            v-model="form.serviceName"
            placeholder="请输入服务名"
          />
        </div>

        <!-- 是否启用 -->
        <div class="form-item">
          <label>是否启用</label>
          <select v-model="form.isEnabled">
            <option :value="1">启用</option>
            <option :value="0">禁用</option>
          </select>
        </div>
      </div>

      <!-- 表单操作按钮 -->
      <div class="btn-row">
        <button
          class="action-btn primary-btn"
          :disabled="loading"
          @click="isEditMode ? updateConnection() : createConnection()"
          type="button"
        >
          {{ loading ? '处理中...' : (isEditMode ? '保存编辑' : '创建连接配置') }}
        </button>

        <button
          v-if="isEditMode"
          class="action-btn warning-btn"
          :disabled="loading"
          @click="resetForm"
          type="button"
        >
          取消编辑
        </button>

        <button
          class="action-btn secondary-btn"
          :disabled="loading"
          @click="loadConnections"
          type="button"
        >
          刷新列表
        </button>
      </div>
    </div>

    <!-- 列表区域 -->
    <div class="card table-card">
      <h2>连接配置列表</h2>

      <div class="table-wrap">
        <table class="result-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>连接名称</th>
              <th>展示名称</th>
              <th>类型</th>
              <th>主机</th>
              <th>端口</th>
              <th>用户名</th>
              <th>数据库名</th>
              <th>服务名</th>
              <th>是否启用</th>
              <th>创建时间</th>
              <th>更新时间</th>
              <th>操作</th>
            </tr>
          </thead>

          <tbody>
            <tr v-for="item in connections" :key="item.id">
              <td>{{ item.id }}</td>
              <td>{{ item.name }}</td>
              <td>{{ item.label }}</td>
              <td>{{ item.dbType }}</td>
              <td>{{ item.host }}</td>
              <td>{{ item.port }}</td>
              <td>{{ item.username }}</td>
              <td>{{ item.databaseName }}</td>
              <td>{{ item.serviceName }}</td>
              <td>{{ item.isEnabled === 1 ? '启用' : '禁用' }}</td>
              <td>{{ item.createTime }}</td>
              <td>{{ item.updateTime }}</td>
              <td>
                <div class="row-btns">
                  <button
                    class="mini-btn edit-btn"
                    @click="startEdit(item)"
                    type="button"
                  >
                    编辑
                  </button>
                  <button
                    class="mini-btn delete-btn"
                    @click="deleteConnection(item)"
                    type="button"
                  >
                    删除
                  </button>
                </div>
              </td>
            </tr>

            <tr v-if="connections.length === 0">
              <td colspan="13">暂无连接配置数据</td>
            </tr>
          </tbody>
        </table>
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
          <button class="action-btn warning-btn" @click="handleConfirmYes">
            强制保存
          </button>
          <button class="action-btn secondary-btn" @click="handleConfirmNo">
            取消
          </button>
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
 * 表单采用两列布局
 */
.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(260px, 1fr));
  gap: 16px;
  margin-bottom: 16px;
}

/**
 * 单个表单项
 */
.form-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.form-item label {
  font-size: 14px;
  color: #333;
}

/**
 * 输入框与下拉框统一样式
 */
input,
select {
  width: 100%;
  padding: 10px 12px;
  font-size: 16px;
  border: 1px solid #ccc;
  border-radius: 6px;
  background: #fff;
}

/**
 * 按钮区域
 */
.btn-row {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

/**
 * 主按钮通用样式
 */
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
 * 表格容器支持横向滚动
 */
.table-wrap {
  width: 100%;
  overflow-x: auto;
}

.result-table {
  width: 100%;
  border-collapse: collapse;
  background: #fff;
}

.result-table th,
.result-table td {
  border: 1px solid #ddd;
  padding: 10px;
  text-align: left;
  white-space: nowrap;
}

.result-table th {
  background: #f5f7fa;
}

/**
 * 行内操作按钮
 */
.row-btns {
  display: flex;
  gap: 8px;
}

.mini-btn {
  padding: 6px 12px;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  color: #fff;
}

.edit-btn {
  background: #409eff;
}

.delete-btn {
  background: #f56c6c;
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
  display: flex;
  align-items: center;
  gap: 8px;
}

.modal-body {
  padding: 24px 20px;
}

.modal-message {
  margin: 0;
  color: #606266;
  font-size: 15px;
  line-height: 1.6;
  white-space: pre-wrap; /* 保证换行符能正常显示 */
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