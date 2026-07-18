<script setup lang="ts">
/**
 * AdminUserPanel.vue
 * ------------------------------------------------------------------
 * 该组件是"用户管理"页面。
 *
 * 布局模式：默认进入用户列表，右上角「新增用户」切换到表单视图。
 *
 * 主要功能：
 * 1. 展示用户列表
 * 2. 新增用户
 * 3. 编辑用户
 * 4. 删除用户
 * 5. 编辑普通用户时，可以重新分配可查询数据库连接
 */

import { computed, onMounted, ref } from 'vue'

interface AdminUserItem {
  id: number
  username: string
  displayName: string
  roleName: string
  isEnabled: number
  canQueryData: number
  canQueryPlan: number
  allowedConnections: string[]
  createTime: string
  updateTime: string
}

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
}

const loading = ref(false)
const message = ref('')

/**
 * 视图模式：list 用户列表 / form 新增/编辑
 */
const viewMode = ref<'list' | 'form'>('list')

/**
 * 当前是否编辑模式
 */
const isEditMode = ref(false)

const users = ref<AdminUserItem[]>([])
const connections = ref<AdminConnectionItem[]>([])

const form = ref({
  id: 0,
  username: '',
  password: '',
  displayName: '',
  roleName: 'user',
  isEnabled: 1,
  canQueryData: 1,
  canQueryPlan: 1,
  allowedConnections: [] as string[],
})

const isNormalUser = computed(() => form.value.roleName === 'user')

/**
 * 将后端时间字符串（如 2026-07-08T08:07:00+08:00 或 2026-07-08 08:07:00）
 * 统一格式化为 2026-07-08 08:07
 */
const formatTime = (v: string): string => {
  if (!v) return ''
  return v.replace('T', ' ').slice(0, 16)
}

const resetForm = () => {
  isEditMode.value = false
  form.value = {
    id: 0,
    username: '',
    password: '',
    displayName: '',
    roleName: 'user',
    isEnabled: 1,
    canQueryData: 1,
    canQueryPlan: 1,
    allowedConnections: [],
  }
}

/**
 * 进入新增视图
 */
const showCreateForm = () => {
  resetForm()
  message.value = '请填写用户信息'
  viewMode.value = 'form'
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

/**
 * 返回列表视图
 */
const backToList = () => {
  viewMode.value = 'list'
  message.value = ''
  loadUsers()
}

const loadUsers = async () => {
  loading.value = true
  message.value = ''

  try {
    const res = await fetch('/api/admin/users', {
      credentials: 'include',
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || '加载用户列表失败'
      users.value = []
      return
    }

    users.value = data.users || []
    message.value = '用户列表加载成功'
  } catch (err) {
    console.error(err)
    message.value = '加载用户列表失败，请检查后端是否启动'
    users.value = []
  } finally {
    loading.value = false
  }
}

const loadConnections = async () => {
  try {
    const res = await fetch('/api/admin/db-connections', {
      credentials: 'include',
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      connections.value = []
      return
    }

    connections.value = data.connections || []
  } catch (err) {
    console.error(err)
    connections.value = []
  }
}

const toggleConnection = (name: string) => {
  const idx = form.value.allowedConnections.indexOf(name)
  if (idx >= 0) {
    form.value.allowedConnections.splice(idx, 1)
  } else {
    form.value.allowedConnections.push(name)
  }
}

const isConnectionChecked = (name: string) => {
  return form.value.allowedConnections.includes(name)
}

const createUser = async () => {
  if (isNormalUser.value && form.value.allowedConnections.length === 0) {
    message.value = '普通用户必须分配至少一个可查询数据库连接'
    return
  }

  loading.value = true
  message.value = ''

  try {
    const res = await fetch('/api/admin/users', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({
        username: form.value.username,
        password: form.value.password,
        displayName: form.value.displayName,
        roleName: form.value.roleName,
        isEnabled: form.value.isEnabled,
        canQueryData: form.value.canQueryData,
        canQueryPlan: form.value.canQueryPlan,
        allowedConnections: form.value.allowedConnections,
      }),
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || '创建用户失败'
      return
    }

    message.value = data.message || '创建用户成功'
    backToList()
  } catch (err) {
    console.error(err)
    message.value = '创建用户失败，请检查后端是否启动'
  } finally {
    loading.value = false
  }
}

const startEdit = (item: AdminUserItem) => {
  isEditMode.value = true
  form.value = {
    id: item.id,
    username: item.username,
    password: '',
    displayName: item.displayName,
    roleName: item.roleName,
    isEnabled: item.isEnabled,
    canQueryData: item.canQueryData,
    canQueryPlan: item.canQueryPlan,
    allowedConnections: [...(item.allowedConnections || [])],
  }
  message.value = `正在编辑用户：${item.username}`
  viewMode.value = 'form'
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

const updateUser = async () => {
  if (isNormalUser.value && form.value.allowedConnections.length === 0) {
    message.value = '普通用户必须分配至少一个可查询数据库连接'
    return
  }

  loading.value = true
  message.value = ''

  try {
    const res = await fetch('/api/admin/users', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({
        id: form.value.id,
        username: form.value.username,
        password: form.value.password,
        displayName: form.value.displayName,
        roleName: form.value.roleName,
        isEnabled: form.value.isEnabled,
        canQueryData: form.value.canQueryData,
        canQueryPlan: form.value.canQueryPlan,
        allowedConnections: form.value.allowedConnections,
      }),
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || '编辑用户失败'
      return
    }

    message.value = data.message || '编辑用户成功'
    backToList()
  } catch (err) {
    console.error(err)
    message.value = '编辑用户失败，请检查后端是否启动'
  } finally {
    loading.value = false
  }
}

const deleteUser = async (item: AdminUserItem) => {
  const ok = window.confirm(`确定要删除用户【${item.username}】吗？`)
  if (!ok) return

  loading.value = true
  message.value = ''

  try {
    const res = await fetch('/api/admin/users', {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({
        id: item.id,
      }),
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || '删除用户失败'
      return
    }

    message.value = data.message || '删除用户成功'
    await loadUsers()
  } catch (err) {
    console.error(err)
    message.value = '删除用户失败，请检查后端是否启动'
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await loadConnections()
  await loadUsers()
})
</script>

<template>
  <div class="admin-page">
    <!-- ============ 用户列表视图 ============ -->
    <div v-if="viewMode === 'list'" class="card table-card">
      <div class="table-header">
        <h2>用户列表 (总数: {{ users.length }})</h2>
        <button class="action-btn primary-btn" :disabled="loading" @click="showCreateForm" type="button">
          + 新增用户
        </button>
      </div>

      <p class="result">{{ message }}</p>

      <div class="table-wrap">
        <table class="result-table">
          <colgroup>
            <col style="width: 10%" />
            <col style="width: 10%" />
            <col style="width: 7%" />
            <col style="width: 8%" />
            <col style="width: 7%" />
            <col style="width: 7%" />
            <col style="width: 18%" />
            <col style="width: 11%" />
            <col style="width: 11%" />
            <col style="width: 11%" />
          </colgroup>
          <thead>
            <tr>
              <th>用户名</th>
              <th>显示名称</th>
              <th>角色</th>
              <th>是否启用</th>
              <th>查询页</th>
              <th>计划页</th>
              <th>可查询连接</th>
              <th>创建时间</th>
              <th>更新时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in users" :key="item.id" class="data-row" @dblclick="startEdit(item)">
              <td>{{ item.username }}</td>
              <td>{{ item.displayName }}</td>
              <td>
                <span :class="['role-tag', item.roleName === 'admin' ? 'role-admin' : 'role-user']">
                  {{ item.roleName }}
                </span>
              </td>
              <td>
                <span :class="['enable-tag', item.isEnabled === 1 ? 'enable-on' : 'enable-off']">
                  {{ item.isEnabled === 1 ? '启用' : '禁用' }}
                </span>
              </td>
              <td>{{ item.canQueryData === 1 ? '允许' : '禁止' }}</td>
              <td>{{ item.canQueryPlan === 1 ? '允许' : '禁止' }}</td>
              <td class="cell-wrap">{{ (item.allowedConnections || []).join(', ') || '-' }}</td>
              <td>{{ formatTime(item.createTime) }}</td>
              <td>{{ formatTime(item.updateTime) }}</td>
              <td>
                <div class="row-btns">
                  <button class="mini-btn edit-btn" @click="startEdit(item)" type="button">编辑</button>
                  <button class="mini-btn delete-btn" @click="deleteUser(item)" type="button">删除</button>
                </div>
              </td>
            </tr>
            <tr v-if="users.length === 0">
              <td colspan="10" class="empty-text">暂无用户数据</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- ============ 新增/编辑视图 ============ -->
    <div v-else class="card form-card">
      <div class="form-header">
        <h2>{{ isEditMode ? '编辑用户' : '新增用户' }}</h2>
        <button class="action-btn secondary-btn back-btn" @click="backToList" type="button">
          ← 返回列表
        </button>
      </div>
      <p class="result">{{ message }}</p>

      <div class="form-grid">
        <!-- 第一行：用户名 / 密码 / 显示名称 -->
        <div class="form-item">
          <label>用户名</label>
          <input v-model="form.username" placeholder="请输入用户名" />
        </div>
        <div class="form-item">
          <label>{{ isEditMode ? '密码（留空表示不修改）' : '密码' }}</label>
          <input v-model="form.password" type="password" :placeholder="isEditMode ? '留空表示不修改密码' : '请输入密码'" />
        </div>
        <div class="form-item">
          <label>显示名称</label>
          <input v-model="form.displayName" placeholder="请输入显示名称" />
        </div>

        <!-- 第二行：角色 / 是否启用 / 查询数据页 -->
        <div class="form-item">
          <label>角色</label>
          <select v-model="form.roleName">
            <option value="user">user</option>
            <option value="admin">admin</option>
          </select>
        </div>
        <div class="form-item">
          <label>是否启用</label>
          <select v-model="form.isEnabled">
            <option :value="1">启用</option>
            <option :value="0">禁用</option>
          </select>
        </div>
        <div class="form-item">
          <label>是否允许访问查询数据页</label>
          <select v-model="form.canQueryData">
            <option :value="1">允许</option>
            <option :value="0">禁止</option>
          </select>
        </div>

        <!-- 第三行：执行计划页 -->
        <div class="form-item">
          <label>是否允许访问执行计划页</label>
          <select v-model="form.canQueryPlan">
            <option :value="1">允许</option>
            <option :value="0">禁止</option>
          </select>
        </div>
      </div>

      <!-- 分配可查询数据库连接（普通用户才显示，整行） -->
      <div v-if="isNormalUser" class="connection-assign-block">
        <div class="connection-title">分配可查询数据库连接</div>

        <div v-if="connections.length === 0" class="empty-tip">
          当前没有可分配的数据库连接，请先创建数据库连接配置
        </div>

        <div v-else class="connection-grid">
          <label
            v-for="item in connections"
            :key="item.name"
            class="connection-checkbox"
          >
            <input
              type="checkbox"
              :checked="isConnectionChecked(item.name)"
              @change="toggleConnection(item.name)"
            />
            <span>
              {{ item.name }}（{{ item.dbType }}）
            </span>
          </label>
        </div>
      </div>

      <div class="btn-row">
        <button
          v-if="!isEditMode"
          class="action-btn primary-btn"
          :disabled="loading"
          @click="createUser"
          type="button"
        >
          {{ loading ? '处理中...' : '创建用户' }}
        </button>
        <template v-else>
          <button
            class="action-btn primary-btn"
            :disabled="loading"
            @click="updateUser"
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

.connection-assign-block {
  margin-bottom: 16px;
  padding: 16px;
  border: 1px solid #ebeef5;
  border-radius: 8px;
  background: #fafafa;
}

.connection-title {
  margin-bottom: 10px;
  font-size: 14px;
  font-weight: 700;
  color: #2c3e50;
}

.connection-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(220px, 1fr));
  gap: 10px;
}

.connection-checkbox {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #333;
}

.empty-tip {
  color: #999;
  font-size: 14px;
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

.cell-wrap {
  white-space: normal;
}

/**
 * 角色 / 启用状态标签
 */
.role-tag {
  display: inline-block;
  padding: 2px 10px;
  border-radius: 10px;
  font-size: 13px;
  color: #fff;
}

.role-admin {
  background: #f56c6c;
}

.role-user {
  background: #409eff;
}

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
</style>
