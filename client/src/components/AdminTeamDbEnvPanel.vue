<script setup lang="ts">
import { onMounted, ref } from 'vue'

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
  createTime: string
  updateTime: string
}

const loading = ref(false)
const message = ref('')
const isEditMode = ref(false)

const records = ref<TeamDbEnvItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const filterTeamName = ref('')

const handleFilterChange = () => {
  page.value = 1
  loadRecords()
}

const form = ref({
  id: 0,
  teamName: '',
  envName: '',
  dbType: '',
  testDbIp: '',
  testDbName: '',
  testDbSchema: '',
  prodDbIp: '',
  prodDbName: '',
  prodDbSchema: '',
})

const resetForm = () => {
  isEditMode.value = false
  form.value = {
    id: 0,
    teamName: '',
    envName: '',
    dbType: '',
    testDbIp: '',
    testDbName: '',
    testDbSchema: '',
    prodDbIp: '',
    prodDbName: '',
    prodDbSchema: '',
  }
}

const loadRecords = async () => {
  loading.value = true
  message.value = ''

  try {
    const res = await fetch(`/api/admin/team-db-envs?page=${page.value}&pageSize=${pageSize.value}&teamName=${encodeURIComponent(filterTeamName.value)}`, {
      credentials: 'include',
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || '加载列表失败'
      records.value = []
      return
    }

    records.value = data.records || []
    total.value = data.total || 0
    message.value = '加载列表成功'
  } catch (err) {
    console.error(err)
    message.value = '加载列表失败，请检查后端是否启动'
    records.value = []
  } finally {
    loading.value = false
  }
}

const createRecord = async () => {
  loading.value = true
  message.value = ''

  try {
    const res = await fetch('/api/admin/team-db-envs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(form.value),
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || '创建配置失败'
      return
    }

    message.value = data.message || '创建配置成功'
    resetForm()
    await loadRecords()
  } catch (err) {
    console.error(err)
    message.value = '创建配置失败，请检查后端是否启动'
  } finally {
    loading.value = false
  }
}

const startEdit = (item: TeamDbEnvItem) => {
  isEditMode.value = true
  form.value = { ...item }
  message.value = `正在编辑配置：${item.teamName} - ${item.envName}`
}

const updateRecord = async () => {
  loading.value = true
  message.value = ''

  try {
    const res = await fetch('/api/admin/team-db-envs', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(form.value),
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || '更新配置失败'
      return
    }

    message.value = data.message || '更新配置成功'
    resetForm()
    await loadRecords()
  } catch (err) {
    console.error(err)
    message.value = '更新配置失败，请检查后端是否启动'
  } finally {
    loading.value = false
  }
}

const cloneRecord = (item: TeamDbEnvItem) => {
  isEditMode.value = false
  form.value = { ...item, id: 0 }
  message.value = `正在克隆配置：${item.teamName} - ${item.envName}，请修改后点击“创建配置”`
  window.scrollTo({
    top: 0,
    behavior: 'smooth'
  })
}

const deleteRecord = async (item: TeamDbEnvItem) => {
  const ok = window.confirm(`确定要删除配置【${item.teamName} - ${item.envName}】吗？`)
  if (!ok) return

  loading.value = true
  message.value = ''

  try {
    const res = await fetch('/api/admin/team-db-envs', {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ id: item.id }),
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || '删除配置失败'
      return
    }

    message.value = data.message || '删除配置成功'

    if (isEditMode.value && form.value.id === item.id) {
      resetForm()
    }

    await loadRecords()
  } catch (err) {
    console.error(err)
    message.value = '删除配置失败，请检查后端是否启动'
  } finally {
    loading.value = false
  }
}

const handlePageChange = (delta: number) => {
  const newPage = page.value + delta
  if (newPage < 1) return
  page.value = newPage
  loadRecords()
}

onMounted(() => {
  loadRecords()
})
</script>

<template>
  <div class="admin-page">
    <div class="card form-card">
      <h2>{{ isEditMode ? '编辑数据库环境配置' : '添加数据库环境配置' }}</h2>
      <p class="result">{{ message }}</p>

      <div class="form-grid">
        <div class="form-item">
          <label>团队名称</label>
          <select v-model="form.teamName">
            <option value="">请选择团队名称</option>
            <option value="交易开发">交易开发</option>
            <option value="运营开发">运营开发</option>
            <option value="后台开发">后台开发</option>
            <option value="增长开发">增长开发</option>
          </select>
        </div>
        <div class="form-item">
          <label>环境名称</label>
          <input v-model="form.envName" placeholder="例如：交易核心库" />
        </div>
        <div class="form-item">
          <label>数据库类型</label>
          <input v-model="form.dbType" placeholder="例如：MySQL, Oracle, redis" />
        </div>
        <div class="form-item">
          <label>测试线数据库IP</label>
          <input v-model="form.testDbIp" placeholder="" />
        </div>
        <div class="form-item">
          <label>测试线库名</label>
          <input v-model="form.testDbName" placeholder="" />
        </div>
        <div class="form-item">
          <label>测试线Schema</label>
          <input v-model="form.testDbSchema" placeholder="" />
        </div>
        <div class="form-item">
          <label>生产线数据库IP</label>
          <input v-model="form.prodDbIp" placeholder="" />
        </div>
        <div class="form-item">
          <label>生产线库名/实例名</label>
          <input v-model="form.prodDbName" placeholder="" />
        </div>
        <div class="form-item">
          <label>生产线Schema</label>
          <input v-model="form.prodDbSchema" placeholder="" />
        </div>
      </div>

      <div class="btn-row">
        <button
          v-if="!isEditMode"
          @click="createRecord"
          class="action-btn primary-btn"
          :disabled="loading"
          type="button"
        >
          {{ loading ? '处理中...' : '创建配置' }}
        </button>

        <template v-else>
          <button 
            @click="updateRecord" 
            class="action-btn primary-btn" 
            :disabled="loading"
            type="button"
          >
            {{ loading ? '保存中...' : '保存修改' }}
          </button>
          <button 
            @click="resetForm" 
            class="action-btn warning-btn" 
            :disabled="loading"
            type="button"
          >
            取消编辑
          </button>
        </template>

        <button
          class="action-btn secondary-btn"
          :disabled="loading"
          @click="loadRecords"
          type="button"
        >
          刷新列表
        </button>
      </div>
    </div>

    <div class="card table-card">
      <div class="table-header">
        <h2>配置列表 (总数: {{ total }})</h2>
        <select v-model="filterTeamName" @change="handleFilterChange" class="filter-select">
          <option value="">全部团队</option>
          <option value="交易开发">交易开发</option>
          <option value="运营开发">运营开发</option>
          <option value="后台开发">后台开发</option>
          <option value="增长开发">增长开发</option>
        </select>
      </div>
      <div class="table-wrap">
        <table class="result-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>团队名称</th>
              <th>环境名称</th>
              <th>数据库类型</th>
              <th>测试线IP</th>
              <th>测试线库名</th>
              <th>测试线Schema</th>
              <th>生产线IP</th>
              <th>生产线库名/实例名</th>
              <th>生产线Schema</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in records" :key="item.id">
              <td>{{ item.id }}</td>
              <td>{{ item.teamName }}</td>
              <td>{{ item.envName }}</td>
              <td>{{ item.dbType }}</td>
              <td>{{ item.testDbIp }}</td>
              <td>{{ item.testDbName }}</td>
              <td>{{ item.testDbSchema }}</td>
              <td>{{ item.prodDbIp }}</td>
              <td>{{ item.prodDbName }}</td>
              <td>{{ item.prodDbSchema }}</td>
              <td>
                <div class="row-btns">
                  <button @click="startEdit(item)" class="mini-btn edit-btn">编辑</button>
                  <button @click="cloneRecord(item)" class="mini-btn warning-btn">克隆</button>
                  <button @click="deleteRecord(item)" class="mini-btn delete-btn">删除</button>
                </div>
              </td>
            </tr>
            <tr v-if="records.length === 0">
              <td colspan="10" class="empty-text">暂无数据</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="pagination">
        <button
          class="action-btn secondary-btn"
          :disabled="page <= 1 || loading"
          @click="handlePageChange(-1)"
        >
          上一页
        </button>
        <span class="page-info">第 {{ page }} 页</span>
        <button
          class="action-btn secondary-btn"
          :disabled="records.length < pageSize || loading"
          @click="handlePageChange(1)"
        >
          下一页
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

.pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
  margin-top: 20px;
}

.page-info {
  font-size: 14px;
}

.table-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.table-header h2 {
  margin-bottom: 0;
}
.filter-select {
  width: 200px;
  padding: 6px 12px;
  border-radius: 6px;
  border: 1px solid #ccc;
}
</style>
