<script setup lang="ts">
/**
 * MetadataPanel.vue
 * ------------------------------------------------------------------
 * 该组件负责查询页左侧的“表名 / 字段提示”区域：
 * 1. 搜索表名或字段名
 * 2. 展示匹配到的表
 * 3. 展示当前选中表的字段
 * 4. 点击插入表名 / 字段名
 *
 * 该组件不管理真实查询逻辑，只负责 UI 展示与事件抛出。
 */

interface QueryMetadataTable {
  name: string
  comment: string
}

interface QueryMetadataColumn {
  tableName: string
  columnName: string
  columnType: string
  comment: string
}

const props = defineProps<{
  loading: boolean
  keyword: string
  tables: QueryMetadataTable[]
  selectedTableName: string
  currentTableColumns: QueryMetadataColumn[]
}>()

const emit = defineEmits<{
  (e: 'update:keyword', value: string): void
  (e: 'search'): void
  (e: 'select-table', tableName: string): void
  (e: 'insert-table-name', tableName: string): void
  (e: 'insert-column-name', columnFullName: string): void
}>()

/**
 * 输入框变化
 */
const handleKeywordInput = (event: Event) => {
  const value = (event.target as HTMLInputElement).value
  emit('update:keyword', value)
}
</script>

<template>
  <div class="card side-card narrow-card">
    <h2>表名 / 字段提示</h2>

    <div class="metadata-search-row">
      <input
        class="metadata-search-input"
        :value="props.keyword"
        placeholder="输入表名或字段名关键字"
        @input="handleKeywordInput"
        @keyup.enter="emit('search')"
      />
      <button class="action-btn primary-btn small-btn" @click="emit('search')" type="button">
        搜索
      </button>
    </div>

    <div class="metadata-split-grid">
      <div class="meta-split-col">
        <h3>表</h3>
        <div v-if="props.loading" class="empty-tip">加载中...</div>
        <div v-else-if="props.tables.length === 0" class="empty-tip">暂无匹配表</div>

        <div v-else class="meta-list split-list">
          <div
            v-for="item in props.tables"
            :key="item.name"
            class="meta-item"
            :class="{ activeMetaItem: props.selectedTableName === item.name }"
            @click="emit('select-table', item.name)"
          >
            <div class="meta-title">{{ item.name }}</div>
            <div class="meta-comment">{{ item.comment || '无注释' }}</div>
            <div class="meta-actions">
              <button class="tiny-btn" @click.stop="emit('insert-table-name', item.name)" type="button">
                插入表名
              </button>
            </div>
          </div>
        </div>
      </div>

      <div class="meta-split-col">
        <h3>字段</h3>

        <div v-if="!props.selectedTableName" class="empty-tip">
          请先选择一张表
        </div>

        <template v-else>
          <div class="selected-table-tip">
            当前已选表：<strong>{{ props.selectedTableName }}</strong>
          </div>

          <div v-if="props.currentTableColumns.length === 0" class="empty-tip">
            当前表暂无字段信息
          </div>

          <div v-else class="meta-list split-list">
            <div
              v-for="item in props.currentTableColumns"
              :key="`${item.tableName}.${item.columnName}`"
              class="meta-item"
              @click="emit('insert-column-name', `${item.tableName}.${item.columnName}`)"
            >
              <div class="meta-title">{{ item.columnName }}</div>
              <div class="meta-sub">{{ item.columnType }}</div>
              <div class="meta-comment">{{ item.comment || '无注释' }}</div>
            </div>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
.card {
  width: 100%;
  border: 1px solid #ddd;
  border-radius: 10px;
  padding: 24px;
  margin-bottom: 24px;
  background: #fff;
}

.side-card,
.narrow-card {
  height: clamp(620px, calc(100vh - 230px), 900px);
  min-height: clamp(620px, calc(100vh - 230px), 900px);
  display: flex;
  flex-direction: column;
}

h2,
h3 {
  color: #2c3e50;
}

h2 {
  margin-top: 0;
}

h3 {
  margin: 0 0 10px;
  font-size: 16px;
}

.metadata-search-row {
  display: flex;
  gap: 10px;
  margin-bottom: 16px;
}

.metadata-search-input {
  width: 100%;
  padding: 10px 12px;
  font-size: 16px;
  border: 1px solid #ccc;
  border-radius: 6px;
  background: #fff;
}

.metadata-split-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  flex: 1;
  min-height: 0;
}

.meta-split-col {
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.meta-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.split-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.meta-item {
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 10px 12px;
  background: #fafafa;
  cursor: pointer;
}

.meta-item:hover {
  background: #f0f7ff;
}

.activeMetaItem {
  border-color: #409eff;
  background: #ecf5ff;
}

.meta-title {
  font-weight: 600;
  color: #2c3e50;
  word-break: break-all;
}

.meta-sub {
  margin-top: 4px;
  font-size: 13px;
  color: #409eff;
}

.meta-comment {
  margin-top: 4px;
  font-size: 13px;
  color: #666;
  word-break: break-word;
}

.meta-actions {
  margin-top: 8px;
}

.selected-table-tip {
  margin-bottom: 10px;
  padding: 8px 10px;
  border-radius: 6px;
  background: #f5f7fa;
  color: #555;
  font-size: 13px;
}

.empty-tip {
  color: #999;
  font-size: 14px;
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

.small-btn {
  padding: 8px 14px;
  font-size: 14px;
}

.tiny-btn {
  padding: 6px 10px;
  font-size: 12px;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  background: #409eff;
  color: #fff;
}
</style>