<script setup lang="ts">
/**
 * QueryPanel.vue
 * ------------------------------------------------------------------
 * 该组件负责查询页中间区域：
 * 1. 数据库类型选择
 * 2. 连接配置选择
 * 3. SQL 编辑器
 * 4. SQL 格式化
 * 5. 刷新表字段提示
 * 6. 执行查询 / 中止查询 / 导出 Excel / 清空查询
 * 7. 新增：收藏 SQL / 查看收藏
 *
 * 注意：
 * SQL 收藏列表不在右侧展示，而是通过弹窗组件 SqlFavoritePanel.vue 展示。
 */

import { computed, ref } from 'vue'
import { Codemirror } from 'vue-codemirror'
import { sql as sqlLang } from '@codemirror/lang-sql'
import { autocompletion, type Completion, type CompletionContext } from '@codemirror/autocomplete'
import { EditorView } from '@codemirror/view'
import { format as formatSQL } from 'sql-formatter'

type DBType = 'mysql' | 'oracle'

interface QueryConnectionInfo {
  name: string
  dbType: string
  host: string
  port: number
  database: string
  serviceName: string
  canConnect: number
}

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

interface SimpleEditorView {
  state: {
    selection: {
      main: {
        from: number
        to: number
      }
    }
  }
  dispatch: (spec: {
    changes: {
      from: number
      to: number
      insert: string
    }
    selection: {
      anchor: number
    }
  }) => void
  focus: () => void
}

const props = defineProps<{
  dbType: DBType
  connectionName: string
  sqlText: string
  connections: QueryConnectionInfo[]
  selectedConnection: QueryConnectionInfo | null
  loading: boolean
  exportLoading: boolean
  queryMessage: string
  metadataTables: QueryMetadataTable[]
  metadataColumns: QueryMetadataColumn[]
}>()

const emit = defineEmits<{
  (e: 'update:db-type', value: DBType): void
  (e: 'update:connection-name', value: string): void
  (e: 'update:sql-text', value: string): void
  (e: 'execute'): void
  (e: 'cancel'): void
  (e: 'export'): void
  (e: 'clear'): void
  (e: 'refresh-metadata'): void
  (e: 'message', value: string): void
  (e: 'open-favorites'): void
  (e: 'favorite-current-sql'): void
}>()

const queryEditorView = ref<SimpleEditorView | null>(null)

const canConnect = computed(() => {
  return props.selectedConnection?.canConnect !== 0
})

const metadataTableNameSet = computed(() => {
  return new Set(props.metadataTables.map((item) => item.name.toUpperCase()))
})

const parseAliasTableMap = (sql: string): Record<string, string> => {
  const result: Record<string, string> = {}
  const text = sql || ''
  const tableSet = metadataTableNameSet.value
  const regex = /\b(?:from|join)\s+([A-Za-z0-9_.$"]+)(?:\s+(?:as\s+)?([A-Za-z0-9_]+))?/gi

  let match: RegExpExecArray | null
  while ((match = regex.exec(text)) !== null) {
    const rawTable = (match[1] || '').replace(/"/g, '')
    const rawAlias = match[2] || ''
    if (!rawTable) continue

    const normalizedTable = rawTable.split('.').pop() || rawTable
    const tableUpper = normalizedTable.toUpperCase()

    if (!tableSet.has(tableUpper)) continue
    if (rawAlias) {
      result[rawAlias.toUpperCase()] = tableUpper
    }
  }

  return result
}

const getColumnsByAlias = (alias: string): QueryMetadataColumn[] => {
  const aliasMap = parseAliasTableMap(props.sqlText)
  const tableName = aliasMap[alias.toUpperCase()]
  if (!tableName) return []

  return props.metadataColumns.filter(
    (item) => item.tableName.toUpperCase() === tableName,
  )
}

const queryMetadataCompletionSource = (context: CompletionContext) => {
  const word = context.matchBefore(/[\w$.]+/)
  if (!word && !context.explicit) return null

  const current = word?.text || ''
  const currentLower = current.toLowerCase()
  const options: Completion[] = []

  if (current.includes('.')) {
    const parts = current.split('.')
    const alias = parts[0] || ''
    const suffix = parts[1]?.toLowerCase() || ''

    if (alias) {
      const matchedColumns = getColumnsByAlias(alias)
      matchedColumns.forEach((item) => {
        if (!suffix || item.columnName.toLowerCase().includes(suffix)) {
          options.push({
            label: `${alias}.${item.columnName}`,
            type: 'property',
            detail: item.columnType || 'column',
            info: item.comment || '',
          })
        }
      })
    }

    if (options.length === 0) return null

    return {
      from: word ? word.from : context.pos,
      options,
    }
  }

  props.metadataTables.forEach((item) => {
    if (!currentLower || item.name.toLowerCase().includes(currentLower)) {
      options.push({
        label: item.name,
        type: 'keyword',
        detail: 'table',
        info: item.comment || '',
      })
    }
  })

  if (options.length === 0) return null

  return {
    from: word ? word.from : context.pos,
    options,
  }
}

const queryEditorExtensions = computed(() => [
  sqlLang(),
  EditorView.lineWrapping,
  autocompletion({
    override: [queryMetadataCompletionSource],
  }),
])

const handleQueryEditorReady = (payload: { view: unknown }) => {
  queryEditorView.value = payload.view as SimpleEditorView
}

const handleSqlTextUpdate = (value: string) => {
  emit('update:sql-text', value)
}

const handleDBTypeChange = (event: Event) => {
  const value = (event.target as HTMLSelectElement).value as DBType
  emit('update:db-type', value)
}

const handleConnectionChange = (event: Event) => {
  const value = (event.target as HTMLSelectElement).value
  emit('update:connection-name', value)
}

const formatCurrentSQL = () => {
  try {
    const formatted = formatSQL(props.sqlText, {
      language: props.dbType === 'mysql' ? 'mysql' : 'plsql',
    })
    emit('update:sql-text', formatted)
    emit('message', 'SQL 格式化完成')
  } catch (err) {
    console.error(err)
    emit('message', 'SQL 格式化失败，请检查语法')
  }
}
</script>

<template>
  <div class="card center-card wide-center-card">
    <h2>数据库查询</h2>

    <div class="module-tip">
      该页面仅允许执行查询语句。系统只支持 SELECT / WITH 查询，不允许 INSERT、UPDATE、DELETE、DDL、事务语句和多语句执行。
    </div>

    <div class="query-form-grid">
      <div class="form-item">
        <label>数据库类型</label>
        <select :value="props.dbType" @change="handleDBTypeChange">
          <option value="mysql">MySQL</option>
          <option value="oracle">Oracle</option>
        </select>
      </div>

      <div class="form-item">
        <label>连接配置</label>
        <select :value="props.connectionName" @change="handleConnectionChange">
          <option v-for="item in props.connections" :key="item.name" :value="item.name">
            {{ item.name }}
          </option>
        </select>
      </div>
    </div>

    <div v-if="props.selectedConnection" class="conn-tip">
      <span><strong>主机：</strong>{{ props.selectedConnection.host }}</span>
      <span><strong>端口：</strong>{{ props.selectedConnection.port }}</span>
      <span v-if="props.dbType === 'mysql'">
        <strong>数据库：</strong>{{ props.selectedConnection.database }}
      </span>
      <span v-if="props.dbType === 'oracle'">
        <strong>服务名：</strong>{{ props.selectedConnection.serviceName }}
      </span>
      <span v-if="!canConnect" class="conn-warning">该数据库不可查询</span>
    </div>

    <div class="editor-toolbar">
      <button class="action-btn primary-btn small-btn" @click="formatCurrentSQL" type="button">
        SQL 格式化
      </button>
      <button class="action-btn secondary-btn small-btn" @click="emit('refresh-metadata')" :disabled="!canConnect" type="button">
        刷新表字段提示
      </button>
    </div>

    <div class="query-editor-outer">
      <div class="query-editor-wrap">
        <Codemirror
          :model-value="props.sqlText"
          :extensions="queryEditorExtensions"
          :style="{ height: '100%' }"
          placeholder="请输入查询语句，仅支持 SELECT 或 WITH 开头的查询"
          @update:model-value="handleSqlTextUpdate"
          @ready="handleQueryEditorReady"
        />
      </div>
    </div>

    <div class="btn-row query-btn-row">
      <button class="action-btn primary-btn" @click="emit('execute')" :disabled="props.loading || !canConnect" type="button">
        {{ !canConnect ? '不可查询' : (props.loading ? '查询中...' : '执行查询') }}
      </button>

      <button
        class="action-btn danger-main-btn"
        @click="emit('cancel')"
        :disabled="!props.loading"
        type="button"
      >
        中止查询
      </button>

      <button
        class="action-btn success-btn"
        @click="emit('export')"
        :disabled="props.exportLoading || props.loading"
        type="button"
      >
        {{ props.exportLoading ? '导出中...' : '导出 Excel' }}
      </button>

      <button
        class="action-btn secondary-btn"
        @click="emit('clear')"
        :disabled="props.loading"
        type="button"
      >
        清空查询
      </button>

      <button
        class="action-btn favorite-btn"
        @click="emit('favorite-current-sql')"
        :disabled="props.loading"
        type="button"
      >
        收藏 SQL
      </button>

      <button
        class="action-btn favorite-list-btn"
        @click="emit('open-favorites')"
        :disabled="props.loading"
        type="button"
      >
        查看收藏
      </button>
    </div>

    <div class="query-panel-bottom-space"></div>
  </div>
</template>

<style scoped>
/* 组件特有样式（公共样式见 global.css） */

/* 卡片布局 */
.center-card, .wide-center-card { min-height: 600px; height: auto; display: flex; flex-direction: column; max-height: calc(100vh - 150px); overflow: hidden; }
.center-card { min-width: 0; width: 100%; overflow: hidden; }
h2 { flex: 0 0 auto; }
.module-tip { flex: 0 0 auto; }

/* 查询表单 */
.query-form-grid { flex: 0 0 auto; }

/* 连接信息 */
.conn-tip { flex: 0 0 auto; }
.conn-warning { color: #f56c6c; font-weight: bold; }

/* 编辑器 */
.editor-toolbar { flex: 0 0 auto; }
.query-editor-outer { flex: 1 1 auto; overflow: hidden; }
.query-editor-wrap { height: 100%; min-height: 200px; max-height: 100%; overflow: hidden; flex: 1 1 auto; }
.query-panel-bottom-space { display: none; }

/* 按钮 */
.btn-row { margin-top: 12px; flex: 0 0 auto; min-height: fit-content; width: 100%; }
.query-btn-row { width: 100%; }
.action-btn { white-space: nowrap; flex-shrink: 0; }
.favorite-btn { background: #8e44ad; }
.favorite-list-btn { background: #34495e; }
.danger-main-btn:disabled { opacity: 0.5; cursor: not-allowed; }

/* CodeMirror */
.center-card :deep(.cm-editor) { height: 100%; width: 100%; }
.center-card :deep(.cm-scroller) { overflow: auto; width: 100%; height: 100%; }
.center-card :deep(.cm-gutters) { flex-shrink: 0; border-right: 1px solid #ebeef5; background: #f8f9fb; }
.center-card :deep(.cm-lineNumbers) { min-width: 40px; }
.center-card :deep(.cm-content) { min-width: 0; padding-bottom: 8px; }
.center-card :deep(.cm-focused) { outline: none; }

/* 响应式 */
@media (max-width: 768px) {
  .center-card { max-height: calc(100vh - 100px); }
  .query-editor-wrap { min-height: 150px; }
  .query-form-grid { grid-template-columns: 1fr; }
  .btn-row { justify-content: center; }
}
</style>