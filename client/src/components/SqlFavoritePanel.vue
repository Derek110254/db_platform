<script setup lang="ts">
/**
 * SqlFavoritePanel.vue
 * ------------------------------------------------------------------
 * 该组件是 SQL 收藏弹窗。
 *
 * 功能：
 * 1. 弹窗展示当前用户自己的 SQL 收藏列表
 * 2. 支持新增收藏
 * 3. 支持编辑收藏
 * 4. 支持删除收藏
 * 5. 支持使用收藏（把 SQL / dbType / connectionName 回填到查询页）
 * 6. 支持按数据库类型、连接名称、关键字筛选
 *
 * 说明：
 * - 该组件不放在右侧，而是弹窗方式展示
 * - 当前 SQL 收藏功能只面向当前登录用户本人
 * - 组件内部自行请求 /api/sql-favorites
 */

import { computed, onMounted, ref, watch } from 'vue'

type DBType = 'mysql' | 'oracle'

interface QueryConnectionInfo {
  name: string
  dbType: string
  host: string
  port: number
  database: string
  serviceName: string
}

interface SQLFavoriteItem {
  id: number
  userId: number
  favoriteName: string
  sqlText: string
  dbType: string
  connectionName: string
  remark: string
  isPinned: number
  createTime: string
  updateTime: string
}

const props = defineProps<{
  visible: boolean
  mode: 'add' | 'view'
  currentDbType: DBType
  currentConnectionName: string
  currentSqlText: string
  connections: QueryConnectionInfo[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'apply-favorite', payload: { dbType: DBType; connectionName: string; sqlText: string }): void
  (e: 'message', value: string): void
}>()

/**
 * 弹窗加载状态
 */
const loading = ref(false)

/**
 * 弹窗内部提示信息
 */
const message = ref('')

/**
 * 收藏列表
 */
const favorites = ref<SQLFavoriteItem[]>([])

/**
 * 当前是否编辑模式
 */
const isEditMode = ref(false)

/**
 * 筛选表单
 */
const filterForm = ref({
  dbType: '',
  connectionName: '',
  keyword: '',
})

/**
 * 新增 / 编辑收藏表单
 */
const favoriteForm = ref({
  id: 0,
  favoriteName: '',
  sqlText: props.currentSqlText,
  dbType: props.currentDbType,
  connectionName: props.currentConnectionName,
  remark: '',
  isPinned: 0,
})

/**
 * 根据当前收藏表单的 dbType 过滤连接列表
 */
const filteredConnections = computed(() =>
  props.connections.filter((item) => item.dbType === favoriteForm.value.dbType),
)

/**
 * 关闭弹窗
 */
const closePanel = () => {
  emit('close')
}

/**
 * 重置收藏表单
 * 默认带入当前查询页的 SQL / dbType / connectionName
 */
const resetFavoriteForm = () => {
  isEditMode.value = false
  favoriteForm.value = {
    id: 0,
    favoriteName: '',
    sqlText: props.currentSqlText || '',
    dbType: props.currentDbType,
    connectionName: props.currentConnectionName || '',
    remark: '',
    isPinned: 0,
  }
}

/**
 * 弹窗打开时：
 * 1. 把当前查询页 SQL 带进来，便于直接收藏
 * 2. 自动加载收藏列表
 */
watch(
  () => props.visible,
  (visible) => {
    if (visible) {
      resetFavoriteForm()
      loadFavorites()
    }
  },
)

/**
 * 加载 SQL 收藏列表
 */
const loadFavorites = async () => {
  loading.value = true
  message.value = ''

  try {
    const params = new URLSearchParams()
    if (filterForm.value.dbType) params.append('dbType', filterForm.value.dbType)
    if (filterForm.value.connectionName) params.append('connectionName', filterForm.value.connectionName)
    if (filterForm.value.keyword) params.append('keyword', filterForm.value.keyword)

    const res = await fetch(`/api/sql-favorites?${params.toString()}`, {
      credentials: 'include',
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || '加载 SQL 收藏失败'
      favorites.value = []
      return
    }

    favorites.value = data.favorites || []
    message.value = 'SQL 收藏列表加载成功'
  } catch (err) {
    console.error(err)
    message.value = '加载 SQL 收藏失败，请检查后端是否启动'
    favorites.value = []
  } finally {
    loading.value = false
  }
}

/**
 * 新增收藏
 */
const createFavorite = async () => {
  if (!favoriteForm.value.favoriteName.trim()) {
    message.value = '收藏名称不能为空'
    return
  }
  if (!favoriteForm.value.sqlText.trim()) {
    message.value = 'SQL 内容不能为空'
    return
  }

  loading.value = true
  message.value = ''

  try {
    const res = await fetch('/api/sql-favorites', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(favoriteForm.value),
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || 'SQL 收藏失败'
      emit('message', message.value)
      return
    }

    message.value = data.message || 'SQL 收藏成功'
    emit('message', message.value)
    resetFavoriteForm()
    await loadFavorites()
  } catch (err) {
    console.error(err)
    message.value = 'SQL 收藏失败，请检查后端是否启动'
    emit('message', message.value)
  } finally {
    loading.value = false
  }
}

/**
 * 开始编辑收藏
 */
const startEdit = (item: SQLFavoriteItem) => {
  isEditMode.value = true
  favoriteForm.value = {
    id: item.id,
    favoriteName: item.favoriteName,
    sqlText: item.sqlText,
    dbType: (item.dbType || 'mysql') as DBType,
    connectionName: item.connectionName || '',
    remark: item.remark || '',
    isPinned: item.isPinned || 0,
  }
  message.value = `正在编辑收藏：${item.favoriteName}`
}

/**
 * 保存编辑
 */
const updateFavorite = async () => {
  if (!favoriteForm.value.favoriteName.trim()) {
    message.value = '收藏名称不能为空'
    return
  }
  if (!favoriteForm.value.sqlText.trim()) {
    message.value = 'SQL 内容不能为空'
    return
  }

  loading.value = true
  message.value = ''

  try {
    const res = await fetch('/api/sql-favorites', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(favoriteForm.value),
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || '更新 SQL 收藏失败'
      emit('message', message.value)
      return
    }

    message.value = data.message || '更新 SQL 收藏成功'
    emit('message', message.value)
    resetFavoriteForm()
    await loadFavorites()
  } catch (err) {
    console.error(err)
    message.value = '更新 SQL 收藏失败，请检查后端是否启动'
    emit('message', message.value)
  } finally {
    loading.value = false
  }
}

/**
 * 删除收藏
 */
const deleteFavorite = async (item: SQLFavoriteItem) => {
  const ok = window.confirm(`确定要删除收藏【${item.favoriteName}】吗？`)
  if (!ok) return

  loading.value = true
  message.value = ''

  try {
    const res = await fetch('/api/sql-favorites', {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ id: item.id }),
    })
    const data = await res.json()

    if (!res.ok || !data.ok) {
      message.value = data.message || '删除 SQL 收藏失败'
      emit('message', message.value)
      return
    }

    message.value = data.message || '删除 SQL 收藏成功'
    emit('message', message.value)

    if (isEditMode.value && favoriteForm.value.id === item.id) {
      resetFavoriteForm()
    }

    await loadFavorites()
  } catch (err) {
    console.error(err)
    message.value = '删除 SQL 收藏失败，请检查后端是否启动'
    emit('message', message.value)
  } finally {
    loading.value = false
  }
}

/**
 * 使用收藏
 */
const applyFavorite = (item: SQLFavoriteItem) => {
  emit('apply-favorite', {
    dbType: (item.dbType || 'mysql') as DBType,
    connectionName: item.connectionName || '',
    sqlText: item.sqlText || '',
  })
  emit('message', `已应用收藏：${item.favoriteName}`)
  closePanel()
}

onMounted(() => {
  if (props.visible) {
    loadFavorites()
  }
})
</script>

<template>
  <div v-if="props.visible" class="favorite-modal-mask" @click.self="closePanel">
    <div class="favorite-modal">
      <div class="modal-header">
        <h2>SQL 收藏</h2>
        <button class="close-btn" @click="closePanel" type="button">关闭</button>
      </div>

      <p class="result">{{ message }}</p>

      <!-- 搜索筛选框（仅 view 模式显示） -->
      <div v-if="props.mode === 'view'" class="filter-card">
        <div class="filter-grid">
          <div class="form-item">
            <label>数据库类型</label>
            <select v-model="filterForm.dbType">
              <option value="">全部</option>
              <option value="mysql">mysql</option>
              <option value="oracle">oracle</option>
            </select>
          </div>

          <div class="form-item">
            <label>连接名称</label>
            <input v-model="filterForm.connectionName" placeholder="按连接名称筛选" />
          </div>

          <div class="form-item">
            <label>关键字</label>
            <input v-model="filterForm.keyword" placeholder="收藏名称 / 备注 / SQL 关键字" />
          </div>
        </div>

        <div class="btn-row">
          <button class="action-btn secondary-btn" :disabled="loading" @click="loadFavorites" type="button">
            查询收藏
          </button>
        </div>
      </div>

      <!-- 新增收藏表单（仅 add 模式显示） -->
      <div v-if="props.mode === 'add'" class="edit-card">
        <h3>{{ isEditMode ? '编辑收藏' : '新增收藏' }}</h3>

        <div class="form-grid">
          <div class="form-item">
            <label>收藏名称</label>
            <input v-model="favoriteForm.favoriteName" placeholder="请输入收藏名称" />
          </div>

          <div class="form-item">
            <label>数据库类型</label>
            <select v-model="favoriteForm.dbType">
              <option value="mysql">mysql</option>
              <option value="oracle">oracle</option>
            </select>
          </div>

          <div class="form-item">
            <label>连接配置</label>
            <select v-model="favoriteForm.connectionName">
              <option value="">不绑定连接</option>
              <option v-for="item in filteredConnections" :key="item.name" :value="item.name">
                {{ item.name }}
              </option>
            </select>
          </div>

          <div class="form-item">
            <label>是否置顶</label>
            <select v-model="favoriteForm.isPinned">
              <option :value="0">否</option>
              <option :value="1">是</option>
            </select>
          </div>

          <div class="form-item form-item-full">
            <label>备注</label>
            <input v-model="favoriteForm.remark" placeholder="请输入备注" />
          </div>

          <div class="form-item form-item-full">
            <label>SQL 内容</label>
            <textarea v-model="favoriteForm.sqlText" rows="8" placeholder="请输入需要收藏的 SQL"></textarea>
          </div>
        </div>

        <div class="btn-row">
          <button
            class="action-btn primary-btn"
            :disabled="loading"
            @click="isEditMode ? updateFavorite() : createFavorite()"
            type="button"
          >
            {{ loading ? '处理中...' : (isEditMode ? '保存编辑' : '新增收藏') }}
          </button>

          <button
            v-if="isEditMode"
            class="action-btn warning-btn"
            :disabled="loading"
            @click="resetFavoriteForm"
            type="button"
          >
            取消编辑
          </button>
        </div>
      </div>

      <!-- 收藏列表（仅 view 模式显示） -->
      <div v-if="props.mode === 'view'" class="list-card">
        <h3>收藏列表</h3>

        <div v-if="favorites.length === 0" class="empty-tip">
          暂无 SQL 收藏
        </div>

        <div v-else class="favorite-list">
          <div v-for="item in favorites" :key="item.id" class="favorite-item">
            <div class="favorite-top">
              <div class="favorite-title-row">
                <span class="favorite-name">{{ item.favoriteName }}</span>
                <span v-if="item.isPinned === 1" class="pinned-tag">置顶</span>
              </div>

              <div class="favorite-actions">
                <button class="mini-btn use-btn" @click="applyFavorite(item)" type="button">使用</button>
                <button class="mini-btn edit-btn" @click="startEdit(item)" type="button">编辑</button>
                <button class="mini-btn delete-btn" @click="deleteFavorite(item)" type="button">删除</button>
              </div>
            </div>

            <div class="favorite-meta">
              <span>{{ item.dbType || '-' }}</span>
              <span>{{ item.connectionName || '未绑定连接' }}</span>
              <span>{{ item.updateTime }}</span>
            </div>

            <div v-if="item.remark" class="favorite-remark">
              {{ item.remark }}
            </div>

            <pre class="favorite-sql">{{ item.sqlText }}</pre>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 组件特有样式（公共样式见 global.css） */

/* 弹窗 */
.favorite-modal-mask { position: fixed; inset: 0; z-index: 3000; background: rgba(0,0,0,0.45); display: flex; align-items: center; justify-content: center; padding: 24px; }
.favorite-modal { width: min(1200px, 100%); max-height: 90vh; overflow: auto; background: #fff; border-radius: 12px; padding: 24px; box-shadow: 0 12px 32px rgba(0,0,0,0.2); }
.modal-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; }
.modal-header h2 { margin: 0; color: #2c3e50; }
.close-btn { padding: 8px 14px; border: none; border-radius: 6px; background: #909399; color: #fff; cursor: pointer; }

/* 卡片区 */
.filter-card, .edit-card, .list-card { border: 1px solid #ebeef5; border-radius: 10px; padding: 16px; background: #fafafa; margin-bottom: 16px; }

/* 表单（两列） */
.filter-grid, .form-grid { display: grid; grid-template-columns: repeat(2, minmax(260px, 1fr)); gap: 14px; margin-bottom: 14px; }

/* 收藏列表 */
.favorite-list { display: flex; flex-direction: column; gap: 12px; }
.favorite-item { border: 1px solid #ebeef5; border-radius: 8px; padding: 12px; background: #fff; }
.favorite-top { display: flex; justify-content: space-between; gap: 12px; align-items: flex-start; }
.favorite-title-row { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
.favorite-name { font-size: 15px; font-weight: 700; color: #2c3e50; }
.pinned-tag { padding: 2px 8px; border-radius: 999px; background: #e6a23c; color: #fff; font-size: 12px; }
.favorite-actions { display: flex; gap: 8px; }
.mini-btn { font-size: 12px; }
.favorite-meta { margin-top: 10px; display: flex; gap: 12px; flex-wrap: wrap; font-size: 12px; color: #909399; }
.favorite-remark { margin-top: 10px; color: #666; font-size: 13px; }
.favorite-sql { margin-top: 10px; padding: 12px; border-radius: 8px; background: #f5f7fa; white-space: pre-wrap; word-break: break-word; font-size: 12px; color: #333; max-height: 180px; overflow: auto; }

/* 按钮 */
.use-btn { background: #67c23a; }
.edit-btn { background: #409eff; }
.delete-btn { background: #f56c6c; }
</style>