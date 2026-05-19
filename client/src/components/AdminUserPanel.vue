<script setup lang="ts">
/**
 * AdminUserPanel.vue
 * ------------------------------------------------------------------
 * 该组件是“连接用户管理”页面。
 *
 * 本版本新增：
 * 1. 用户列表增加“编辑 / 删除”按钮
 * 2. 支持编辑用户
 * 3. 支持删除用户
 * 4. 编辑普通用户时，可以重新分配可查询数据库连接
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
  label: string
  dbType: string
  host: string
  port: number
  username: string
  databaseName: string
  serviceName: string
  isEnabled: number
}

/**
 * 页面状态
 */
const loading = ref(false)
const message = ref('')

/**
 * 用户列表
 */
const users = ref<AdminUserItem[]>([])

/**
 * 可分配数据库连接列表
 */
const connections = ref<AdminConnectionItem[]>([])

/**
 * 当前是否编辑模式
 */
const isEditMode = ref(false)

/**
 * 创建 / 编辑用户表单
 */
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

/**
 * 是否普通用户
 * 普通用户必须分配至少一个数据库连接
 */
const isNormalUser = computed(() => form.value.roleName === 'user')

/**
 * 重置表单
 */
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
 * 加载用户列表
 */
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

/**
 * 加载可分配连接列表
 */
const loadConnections = async () => {
  try {
    const res = await fetch('/api/admin/db-connections', {
      credentials: 'include',
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || '加载连接列表失败'
      connections.value = []
      return
    }

    connections.value = data.connections || []
  } catch (err) {
    console.error(err)
    message.value = '加载连接列表失败，请检查后端是否启动'
    connections.value = []
  }
}

/**
 * 勾选 / 取消勾选某个连接
 */
const toggleConnection = (name: string) => {
  const idx = form.value.allowedConnections.indexOf(name)
  if (idx >= 0) {
    form.value.allowedConnections.splice(idx, 1)
  } else {
    form.value.allowedConnections.push(name)
  }
}

/**
 * 判断某连接是否已勾选
 */
const isConnectionChecked = (name: string) => {
  return form.value.allowedConnections.includes(name)
}

/**
 * 创建用户
 */
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
    resetForm()
    await loadUsers()
  } catch (err) {
    console.error(err)
    message.value = '创建用户失败，请检查后端是否启动'
  } finally {
    loading.value = false
  }
}

/**
 * 进入编辑模式
 */
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
}

/**
 * 保存编辑
 */
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
    resetForm()
    await loadUsers()
  } catch (err) {
    console.error(err)
    message.value = '编辑用户失败，请检查后端是否启动'
  } finally {
    loading.value = false
  }
}

/**
 * 删除用户
 */
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

    if (isEditMode.value && form.value.id === item.id) {
      resetForm()
    }

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
    <div class="card form-card">
      <h2>{{ isEditMode ? '编辑用户' : '连接用户管理' }}</h2>
      <p class="result">{{ message }}</p>

      <div class="form-grid">
        <div class="form-item">
          <label>用户名</label>
          <input v-model="form.username" placeholder="请输入用户名" />
        </div>

        <div class="form-item">
          <label>
            {{ isEditMode ? '密码（留空表示不修改）' : '密码' }}
          </label>
          <input v-model="form.password" type="password" :placeholder="isEditMode ? '留空表示不修改密码' : '请输入密码'" />
        </div>

        <div class="form-item">
          <label>显示名称</label>
          <input v-model="form.displayName" placeholder="请输入显示名称" />
        </div>

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

        <div class="form-item">
          <label>是否允许访问执行计划页</label>
          <select v-model="form.canQueryPlan">
            <option :value="1">允许</option>
            <option :value="0">禁止</option>
          </select>
        </div>
      </div>

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
              {{ item.label }}（{{ item.name }} / {{ item.dbType }}）
            </span>
          </label>
        </div>
      </div>

      <div class="btn-row">
        <button
          class="action-btn primary-btn"
          :disabled="loading"
          @click="isEditMode ? updateUser() : createUser()"
          type="button"
        >
          {{ loading ? '处理中...' : (isEditMode ? '保存编辑' : '创建用户') }}
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

        <button class="action-btn secondary-btn" :disabled="loading" @click="loadUsers" type="button">
          刷新列表
        </button>
      </div>
    </div>

    <div class="card table-card">
      <h2>用户列表</h2>

      <div class="table-wrap">
        <table class="result-table">
          <thead>
            <tr>
              <th>ID</th>
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
            <tr v-for="item in users" :key="item.id">
              <td>{{ item.id }}</td>
              <td>{{ item.username }}</td>
              <td>{{ item.displayName }}</td>
              <td>{{ item.roleName }}</td>
              <td>{{ item.isEnabled === 1 ? '启用' : '禁用' }}</td>
              <td>{{ item.canQueryData === 1 ? '允许' : '禁止' }}</td>
              <td>{{ item.canQueryPlan === 1 ? '允许' : '禁止' }}</td>
              <td>{{ (item.allowedConnections || []).join(', ') || '-' }}</td>
              <td>{{ item.createTime }}</td>
              <td>{{ item.updateTime }}</td>
              <td>
                <div class="row-btns">
                  <button class="mini-btn edit-btn" @click="startEdit(item)" type="button">
                    编辑
                  </button>
                  <button class="mini-btn delete-btn" @click="deleteUser(item)" type="button">
                    删除
                  </button>
                </div>
              </td>
            </tr>
            <tr v-if="users.length === 0">
              <td colspan="9">暂无用户数据</td>
            </tr>
          </tbody>
        </table>
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

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(260px, 1fr));
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
  grid-template-columns: repeat(2, minmax(260px, 1fr));
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
</style>