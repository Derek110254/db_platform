<script setup lang="ts">
/**
 * QueryHistoryPanel.vue
 * ------------------------------------------------------------------
 * 该组件负责右侧“查询历史”展示。
 *
 * 主要功能：
 * 1. 展示查询历史列表
 * 2. 点击历史项回填到查询框
 * 3. 删除某条历史记录
 */

interface QueryHistoryItem {
  id: string
  connectionName: string
  dbType: 'mysql' | 'oracle'
  sql: string
  createdAt: string
}

const props = defineProps<{
  historyList: QueryHistoryItem[]
}>()

const emit = defineEmits<{
  (e: 'apply-history', item: QueryHistoryItem): void
  (e: 'delete-history', id: string): void
}>()
</script>

<template>
  <div class="card side-card">
    <h2>查询历史</h2>

    <div v-if="props.historyList.length === 0" class="empty-tip">
      暂无查询历史
    </div>

    <div v-else class="history-list">
      <div
        v-for="item in props.historyList"
        :key="item.id"
        class="history-item"
      >
        <div class="history-top">
          <div class="history-meta">
            <span class="db-type">{{ item.dbType }}</span>
            <span class="conn-name">{{ item.connectionName }}</span>
          </div>
          <div class="history-actions">
            <button class="mini-btn use-btn" @click="emit('apply-history', item)" type="button">
              使用
            </button>
            <button class="mini-btn delete-btn" @click="emit('delete-history', item.id)" type="button">
              删除
            </button>
          </div>
        </div>

        <div class="history-time">{{ item.createdAt }}</div>
        <pre class="history-sql">{{ item.sql }}</pre>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 组件特有样式（公共样式见 global.css） */

/* 侧边栏卡片 */
.side-card { height: clamp(620px, calc(100vh - 230px), 900px); min-height: clamp(620px, calc(100vh - 230px), 900px); overflow: auto; }

/* 历史列表 */
.history-list { display: flex; flex-direction: column; gap: 12px; }
.history-item { border: 1px solid #ebeef5; border-radius: 8px; padding: 12px; background: #fafafa; }
.history-top { display: flex; justify-content: space-between; gap: 8px; align-items: flex-start; }
.history-meta { display: flex; gap: 8px; flex-wrap: wrap; }
.db-type { padding: 2px 8px; border-radius: 999px; background: #eef2ff; color: #333; font-size: 12px; }
.conn-name { font-size: 12px; color: #666; }
.history-actions { display: flex; gap: 6px; }
.history-time { margin-top: 8px; font-size: 12px; color: #999; }
.history-sql { margin: 8px 0 0; font-size: 12px; white-space: pre-wrap; word-break: break-word; color: #333; }

/* 按钮 */
.mini-btn { font-size: 12px; }
.use-btn { background: #409eff; }
.delete-btn { background: #f56c6c; }
</style>