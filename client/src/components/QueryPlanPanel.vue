<script setup lang="ts">
/**
 * QueryPlanPanel.vue
 * ------------------------------------------------------------------
 * 负责执行计划查询页的中间区域：
 * 1. 数据库类型与连接选择
 * 2. SQL 编辑器 (Codemirror)
 * 3. 原始执行计划展示 (兼容 Oracle 树状图 和 MySQL 表格)
 * 4. AI 深度解读展示 (置于最底部)
 */

import { computed, ref, onMounted } from 'vue'
import { Codemirror } from 'vue-codemirror'
import { sql as sqlLang } from '@codemirror/lang-sql'
import { autocompletion, type Completion, type CompletionContext } from '@codemirror/autocomplete'
import { EditorView } from '@codemirror/view'
import { format as formatSQL } from 'sql-formatter'

type DBType = 'mysql' | 'oracle'

interface QueryConnectionInfo {
  name: string
  label: string
  dbType: string
  host: string
  port: number
  database: string
  serviceName: string
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
  queryMessage: string
  queryColumns: string[] // 接收列名
  queryRows: any[]       // 接收行数据
  metadataTables: QueryMetadataTable[]
  metadataColumns: QueryMetadataColumn[]
  queryScore: number // 接收后端传来的分数
}>()

const emit = defineEmits<{
  (e: 'update:db-type', value: DBType): void
  (e: 'update:connection-name', value: string): void
  (e: 'update:sql-text', value: string): void
  (e: 'explain'): void
  (e: 'cancel'): void
  (e: 'refresh-metadata'): void
  (e: 'message', value: string): void
  (e: 'open-audit-history'): void
}>()

const queryEditorView = ref<SimpleEditorView | null>(null)

const isMounted = ref(false)
onMounted(() => {
  isMounted.value = true
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

// 判断是否为 Oracle 的执行计划（包含 PLAN_TABLE_OUTPUT 列）
const isOraclePlan = computed(() => {
  return props.queryColumns && (
    props.queryColumns.includes('PLAN_TABLE_OUTPUT') || 
    props.queryColumns.includes('plan_table_output')
  )
})

// 根据分数返回对应的颜色
const getScoreColor = (score: number) => {
  if (score >= 90) return '#67C23A' // 绿色
  if (score >= 80) return '#409EFF' // 蓝色
  if (score >= 70) return '#E6A23C' // 黄色
  return '#F56C6C' // 红色
}
</script>



<template>
  <div class="query-plan-panel-root">
    <div class="card center-card wide-center-card">
      <h2>SQL 执行性能检测</h2>

      <div class="module-tip">
        该页面仅用于获取查询语句的执行计划，评估 SQL 性能，不会真实执行数据查询操作。
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
              {{ item.label || item.name }}
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
      </div>

      <div class="editor-toolbar">
        <button class="action-btn primary-btn small-btn" @click="formatCurrentSQL" type="button">
          SQL 格式化
        </button>
        <button class="action-btn secondary-btn small-btn" @click="emit('refresh-metadata')" type="button">
          刷新表字段提示
        </button>
      </div>

      <div class="query-editor-outer">
        <div class="query-editor-wrap">
          <Codemirror
            :model-value="props.sqlText"
            :extensions="queryEditorExtensions"
            :style="{ height: '100%' }"
            placeholder="请输入需要检测的查询语句"
            @update:model-value="handleSqlTextUpdate"
            @ready="handleQueryEditorReady"
          />
        </div>
      </div>

      <div class="btn-row query-btn-row">
        <button class="action-btn primary-btn" @click="emit('explain')" :disabled="props.loading" type="button">
          {{ props.loading ? '检测中...' : '获取执行计划' }}
        </button>

        <button class="action-btn purple-btn" @click="emit('open-audit-history')" type="button">
          审核历史
        </button>

        <button
          class="action-btn danger-main-btn"
          @click="emit('cancel')"
          :disabled="!props.loading"
          type="button"
        >
          中止检测
        </button>
      </div>

      <div v-if="props.queryMessage && (!props.queryRows || props.queryRows.length === 0)" class="status-message">
        {{ props.queryMessage }}
      </div>

      <div class="query-panel-bottom-space"></div>
    </div>

    <!-- 执行计划与 AI 解析结果框 -->
    <Teleport to="#plan-result-teleport-target" v-if="isMounted">
      <div class="card plan-result-card" v-if="props.queryRows?.length > 0 || (props.queryMessage && props.queryMessage !== '执行成功' && props.queryMessage !== '获取执行计划成功' && props.queryMessage !== '检测中...' && props.queryMessage !== '正在获取执行计划...')">
        
        <div v-if="props.queryRows && props.queryRows.length > 0" class="plan-result-section">
          <div class="section-title">原始执行计划</div>

          <div v-if="isOraclePlan" class="oracle-plan-container">
            <pre class="oracle-pre"><template v-for="(row, idx) in props.queryRows" :key="idx">{{ row.PLAN_TABLE_OUTPUT || row.plan_table_output }}<br/></template></pre>
          </div>

          <div v-else class="mysql-table-wrapper table-wrap">
            <table class="result-table">
              <thead>
                <tr>
                  <th v-for="col in props.queryColumns" :key="col">{{ col }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(row, idx) in props.queryRows" :key="idx">
                  <td v-for="col in props.queryColumns" :key="col" :title="String(row[col] ?? '')">
                    {{ row[col] }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <div v-if="props.queryMessage && props.queryMessage !== '执行成功' && props.queryMessage !== '获取执行计划成功' && props.queryMessage !== '检测中...' && props.queryMessage !== '正在获取执行计划...'" class="ai-interpretation-section">
          <div class="ai-header">
            <span class="ai-icon">🤖 AI 性能解读分析</span> 
          </div>
          <div v-if="props.queryScore > 0" class="score-badge" :style="{ backgroundColor: getScoreColor(props.queryScore) }">
            性能评分：{{ props.queryScore }}
          </div>
          <div class="ai-content">
            {{ props.queryMessage }}
          </div>
        </div>

      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.query-plan-panel-root {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.query-plan-panel-root > .card.center-card {
  flex: 1;
}

.card {
  width: 100%;
  border: 1px solid #ddd;
  border-radius: 10px;
  padding: 24px;
  margin-bottom: 24px;
  background: #fff;
}

.center-card,
.wide-center-card {
  min-height: 600px;
  height: auto;
  display: flex;
  flex-direction: column;
  max-height: calc(100vh - 150px); /* 放大可视区域 */
  overflow-y: auto; /* 允许整个面板滚动 */
}

/* ================= 结果展示区 CSS ================= */
.plan-result-section {
  margin-top: 24px;
  border-top: 1px solid #ebeef5;
  padding-top: 20px;
  flex: 0 0 auto;
}

.section-title {
  font-size: 16px;
  font-weight: bold;
  color: #303133;
  margin-bottom: 15px;
}

/* Oracle 专属纯文本展示区 (关键) */
.oracle-plan-container {
  background-color: #f8f9fa;
  border: 1px solid #dcdfe6;
  border-radius: 6px;
  padding: 15px;
  overflow-x: auto; /* 允许横向滚动以展示完整树状图 */
  margin-bottom: 20px;
}

.oracle-pre {
  margin: 0;
  /* 必须使用等宽字体对齐 Oracle 的 ASCII 画 */
  font-family: Consolas, Monaco, "Courier New", monospace;
  font-size: 13px;
  line-height: 1.5;
  color: #333;
  white-space: pre; 
}

/* MySQL 表格区 */
.mysql-table-wrapper {
  margin-bottom: 20px;
}

.table-wrap {
  width: 100%;
  overflow-x: auto;
}

.result-table {
  width: 100%;
  border-collapse: collapse;
  background: #fff;
  font-size: 14px;
}

.result-table th,
.result-table td {
  border: 1px solid #ddd;
  padding: 8px 10px;
  text-align: left;
  white-space: nowrap;
}

.result-table th {
  background: #f5f7fa;
  font-weight: bold;
  color: #333;
}

/* AI 解读区域样式 */
.ai-interpretation-section {
  border: 1px solid #c6e2ff;
  border-radius: 8px;
  background-color: #ecf5ff;
  overflow: hidden;
  margin-top: 10px;
}

.ai-header {
  background-color: #d9ecff;
  padding: 12px 16px;
  font-weight: bold;
  color: #409eff;
  display: flex;
  align-items: center;
  gap: 8px;
}

.ai-icon {
  font-size: 18px;
}

.ai-content {
  padding: 16px;
  line-height: 1.8;
  color: #303133;
  /* 核心：确保 AI 文本正确换行并保留 Markdown 段落感 */
  white-space: pre-wrap; 
  word-break: break-all;
  font-size: 14px;
  background-color: #fff;
}
/* ================================================= */

.center-card {
  min-width: 0;
  width: 100%;
}

h2 {
  margin-top: 0;
  color: #2c3e50;
  flex: 0 0 auto;
}

.module-tip {
  margin-bottom: 12px;
  padding: 12px 14px;
  line-height: 1.8;
  background: #f5f7fa;
  border-radius: 8px;
  color: #666;
}

.query-form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
  margin-bottom: 12px;
  width: 100%;
}

.form-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.form-item label {
  color: #333;
  font-size: 14px;
}

select {
  width: 100%;
  padding: 10px 12px;
  font-size: 16px;
  border: 1px solid #ccc;
  border-radius: 6px;
  background: #fff;
}

.conn-tip {
  display: flex;
  flex-wrap: wrap;
  gap: 18px;
  margin-bottom: 12px;
  padding: 12px 14px;
  background: #f5f7fa;
  border-radius: 8px;
  color: #666;
}

.editor-toolbar {
  margin-bottom: 10px;
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.query-editor-outer {
  width: 100%;
  min-width: 0;
  margin-bottom: 12px;
  flex: 1;
  min-height: 200px;
  display: flex;
  flex-direction: column;
}

.query-editor-wrap {
  width: 100%;
  flex: 1;
  min-height: 0;
  border: 1px solid #dcdfe6;
  border-radius: 6px;
  background: #fff;
}

.btn-row {
  margin-top: 12px;
  display: flex;
  gap: 12px;
}

.query-btn-row {
  justify-content: flex-start;
  align-items: center;
}

.query-panel-bottom-space {
  height: 40px;
}

.action-btn {
  padding: 10px 20px;
  font-size: 16px;
  cursor: pointer;
  border: none;
  border-radius: 6px;
  color: #fff;
  white-space: nowrap;
}

.small-btn {
  padding: 8px 14px;
  font-size: 14px;
}

.primary-btn {
  background: #409eff;
}

.secondary-btn {
  background: #909399;
}

.purple-btn {
  background: #8e44ad;
}

.purple-btn:hover {
  background: #9b59b6;
}

.danger-main-btn {
  background: #f56c6c;
}

.danger-main-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.center-card :deep(.cm-editor) {
  height: 100%;
  width: 100%;
}

.center-card :deep(.cm-scroller) {
  overflow: auto;
}

.center-card :deep(.cm-gutters) {
  border-right: 1px solid #ebeef5;
  background: #f8f9fb;
}

/* 样式补充 */
.ai-header {
  display: flex;
  justify-content: space-between; /* 标题靠左，分数靠右 */
  align-items: center;
  background-color: #f5f7fa;
  padding: 10px 16px;
  border-bottom: 1px solid #dcdfe6;
}

.score-badge {
  padding: 4px 12px;
  border-radius: 20px;
  color: #fff;
  font-size: 13px;
  font-weight: bold;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
}

@media (max-width: 768px) {
  .query-editor-wrap {
    height: 200px;
  }
}
</style>