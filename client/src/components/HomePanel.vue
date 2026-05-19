<script setup lang="ts">
/**
 * HomePanel.vue
 * ------------------------------------------------------------------
 * 该组件负责首页的全部逻辑：
 * 1. SQL 风险监测
 * 2. DDL 规范检查
 * 3. SQL / DDL 文件上传
 * 4. SQL 格式化
 *
 * 由于首页逻辑与查询页逻辑独立，因此适合做成自包含组件。
 */

import { computed, ref } from 'vue'
import { Codemirror } from 'vue-codemirror'
import { sql as sqlLang } from '@codemirror/lang-sql'
import { EditorView } from '@codemirror/view'
import { format as formatSQL } from 'sql-formatter'

/**
 * 支持的数据库类型
 */
type DBType = 'mysql' | 'oracle'

/**
 * 首页模式：
 * sql = DML 风险监测
 * ddl = DDL 规范检查
 */
type ModeType = 'sql' | 'ddl'

/**
 * SQL 语法错误项
 */
interface SyntaxIssue {
  line: number
  message: string
  sql: string
}

/**
 * SQL 风险项
 */
interface RiskIssue {
  statementNo: number
  line: number
  ruleID: string
  name: string
  severity: string
  description: string
  suggestion: string
  sql: string
}

/**
 * DDL 检查项
 */
interface DDLIssue {
  objectType: string
  objectName: string
  description: string
  suggestion: string
}

/**
 * 最小编辑器接口
 */
interface SimpleEditorView {
  focus: () => void
}

/**
 * 当前功能模式
 */
const activeMode = ref<ModeType>('sql')

/**
 * 当前数据库类型
 */
const dbType = ref<DBType>('mysql')

/**
 * 文本内容
 */
const sqlText = ref('')
const ddlText = ref('')
const fileName = ref('')

/**
 * SQL 检测结果
 */
const syntaxMessage = ref('还没有检测 SQL')
const syntaxErrors = ref<SyntaxIssue[]>([])
const riskMessage = ref('还没有进行风险检测')
const riskLevel = ref('')
const riskScore = ref(0)
const riskItems = ref<RiskIssue[]>([])
const sqlLoading = ref(false)

/**
 * DDL 检查结果
 */
const ddlMessage = ref('还没有进行 DDL 规范检查')
const ddlIssues = ref<DDLIssue[]>([])
const ddlLoading = ref(false)

/**
 * 编辑器实例
 */
const sqlEditorView = ref<SimpleEditorView | null>(null)
const ddlEditorView = ref<SimpleEditorView | null>(null)

/**
 * SQL 编辑器扩展
 */
const homeSqlEditorExtensions = computed(() => [
  sqlLang(),
  EditorView.lineWrapping,
])

/**
 * DDL 编辑器扩展
 */
const homeDdlEditorExtensions = computed(() => [
  sqlLang(),
  EditorView.lineWrapping,
])

/**
 * SQL 编辑器 ready
 */
const handleHomeSqlEditorReady = (payload: { view: unknown }) => {
  sqlEditorView.value = payload.view as SimpleEditorView
}

/**
 * DDL 编辑器 ready
 */
const handleHomeDdlEditorReady = (payload: { view: unknown }) => {
  ddlEditorView.value = payload.view as SimpleEditorView
}

/**
 * SQL 格式化
 */
const formatHomeSQL = () => {
  try {
    sqlText.value = formatSQL(sqlText.value, {
      language: dbType.value === 'mysql' ? 'mysql' : 'plsql',
    })
    syntaxMessage.value = 'SQL 格式化完成'
  } catch (err) {
    console.error(err)
    syntaxMessage.value = 'SQL 格式化失败，请检查语法'
  }
}

/**
 * 执行 SQL 风险检测
 */
const checkSQL = async () => {
  sqlLoading.value = true
  syntaxErrors.value = []
  riskItems.value = []

  try {
    const res = await fetch('/api/check-sql', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ dbType: dbType.value, sql: sqlText.value }),
    })

    const data = await res.json()
    syntaxMessage.value = data.syntaxMessage || '检测完成'
    syntaxErrors.value = data.syntaxErrors || []
    riskMessage.value = data.riskMessage || '风险检测完成'
    riskLevel.value = data.riskLevel || ''
    riskScore.value = data.riskScore || 0
    riskItems.value = data.riskItems || []
  } catch (err) {
    console.error(err)
    syntaxMessage.value = '检测失败，请检查后端是否启动'
    syntaxErrors.value = []
    riskMessage.value = '未完成风险检测'
    riskItems.value = []
  } finally {
    sqlLoading.value = false
  }
}

/**
 * 执行 DDL 规范检查
 */
const checkDDL = async () => {
  ddlLoading.value = true
  ddlIssues.value = []

  try {
    const res = await fetch('/api/check-ddl', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ dbType: dbType.value, sql: ddlText.value }),
    })

    const data = await res.json()
    ddlMessage.value = data.ddlMessage || 'DDL 规范检查完成'
    ddlIssues.value = data.issues || []
  } catch (err) {
    console.error(err)
    ddlMessage.value = 'DDL 规范检查失败，请检查后端是否启动'
    ddlIssues.value = []
  } finally {
    ddlLoading.value = false
  }
}

/**
 * 文件上传处理
 */
const handleFileUpload = (event: Event) => {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return

  const lowerName = file.name.toLowerCase()
  if (!lowerName.endsWith('.sql') && !lowerName.endsWith('.txt')) {
    if (activeMode.value === 'sql') {
      syntaxMessage.value = '只支持上传 .sql 或 .txt 文件'
    } else {
      ddlMessage.value = '只支持上传 .sql 或 .txt 文件'
    }
    fileName.value = ''
    return
  }

  fileName.value = file.name
  const reader = new FileReader()

  reader.onload = (e) => {
    const content = String(e.target?.result || '')
    if (activeMode.value === 'sql') {
      sqlText.value = content
      syntaxMessage.value = `文件 ${file.name} 已加载，可以开始检测 SQL 风险`
      syntaxErrors.value = []
      riskItems.value = []
    } else {
      ddlText.value = content
      ddlMessage.value = `文件 ${file.name} 已加载，可以开始进行 DDL 规范检查`
      ddlIssues.value = []
    }
  }

  reader.onerror = () => {
    if (activeMode.value === 'sql') {
      syntaxMessage.value = '文件读取失败'
    } else {
      ddlMessage.value = '文件读取失败'
    }
  }

  reader.readAsText(file, 'utf-8')
}

/**
 * 清空当前模式下的内容与结果
 */
const clearCurrent = () => {
  fileName.value = ''
  if (activeMode.value === 'sql') {
    sqlText.value = ''
    syntaxMessage.value = '已清空 SQL 内容'
    syntaxErrors.value = []
    riskMessage.value = '还没有进行风险检测'
    riskLevel.value = ''
    riskScore.value = 0
    riskItems.value = []
  } else {
    ddlText.value = ''
    ddlMessage.value = '已清空 DDL 内容'
    ddlIssues.value = []
  }
}

/**
 * 切换模式
 */
const switchMode = (mode: ModeType) => {
  activeMode.value = mode
  fileName.value = ''
}
</script>

<template>
  <div>
    <div class="mode-row">
      <button
        :class="['mode-btn', activeMode === 'sql' ? 'active' : '']"
        @click="switchMode('sql')"
        type="button"
      >
        DML 风险监测
      </button>
      <button
        :class="['mode-btn', activeMode === 'ddl' ? 'active' : '']"
        @click="switchMode('ddl')"
        type="button"
      >
        DDL 规范检查
      </button>
    </div>

    <div class="home-center-wrap">
      <div class="card home-center-card">
        <div class="db-row">
          <label for="dbType">数据库类型：</label>
          <select id="dbType" v-model="dbType">
            <option value="mysql">MySQL</option>
            <option value="oracle">Oracle</option>
          </select>
        </div>

        <div v-if="activeMode === 'sql'" class="module-tip">
          专注于 DML 语句的语法正确性校验及高风险操作（如无条件更新/删除等）的自动识别与拦截，在沿用原有核心检测规则的基础上，全面保障数据库变更的安全与稳定。
        </div>
        <div v-else class="module-tip">
          DDL 规范检查结果仅输出：对象类型、对象名称、说明、建议；同类问题只保留最早发现的一条。
        </div>

        <div class="upload-row">
          <label class="upload-label">
            选择 SQL 文件：
            <input type="file" accept=".sql,.txt" @change="handleFileUpload" />
          </label>
          <span v-if="fileName" class="file-name">已选择：{{ fileName }}</span>
        </div>

        <template v-if="activeMode === 'sql'">
          <div class="editor-toolbar">
            <button class="action-btn primary-btn small-btn" @click="formatHomeSQL" type="button">
              SQL 格式化
            </button>
          </div>

          <Codemirror
            v-model="sqlText"
            :extensions="homeSqlEditorExtensions"
            :style="{ height: '360px' }"
            placeholder="请输入 SQL，或者上传 .sql / .txt 文件"
            @ready="handleHomeSqlEditorReady"
          />
        </template>

        <template v-else>
          <Codemirror
            v-model="ddlText"
            :extensions="homeDdlEditorExtensions"
            :style="{ height: '360px' }"
            placeholder="请输入 CREATE TABLE / ALTER TABLE / CREATE INDEX 等 DDL，或者上传 .sql / .txt 文件"
            @ready="handleHomeDdlEditorReady"
          />
        </template>

        <div class="btn-row">
          <button
            v-if="activeMode === 'sql'"
            class="action-btn primary-btn"
            @click="checkSQL"
            :disabled="sqlLoading"
            type="button"
          >
            {{ sqlLoading ? '检测中...' : '检测 SQL' }}
          </button>

          <button
            v-else
            class="action-btn primary-btn"
            @click="checkDDL"
            :disabled="ddlLoading"
            type="button"
          >
            {{ ddlLoading ? '检测中...' : '检查 DDL 规范' }}
          </button>

          <button class="action-btn secondary-btn" @click="clearCurrent" type="button">
            清空内容
          </button>
        </div>
      </div>
    </div>

    <template v-if="activeMode === 'sql'">
      <div class="home-center-wrap">
        <div class="card home-center-card">
          <h2>语法检测结果</h2>
          <p class="result">{{ syntaxMessage }}</p>
          <div v-if="syntaxErrors.length > 0" class="check-list">
            <div v-for="(item, idx) in syntaxErrors" :key="idx" class="check-item error">
              <p v-if="item.line > 0"><strong>起始行号：</strong>{{ item.line }}</p>
              <p><strong>错误信息：</strong>{{ item.message }}</p>
              <pre v-if="item.sql">{{ item.sql }}</pre>
            </div>
          </div>
        </div>
      </div>

      <div class="home-center-wrap">
        <div class="card home-center-card">
          <h2>高风险检测结果</h2>
          <p class="result">{{ riskMessage }}</p>
          <p class="result">
            风险等级：<span :class="riskLevel">{{ riskLevel || '-' }}</span>，风险分数：{{ riskScore }}
          </p>
          <div v-if="riskItems.length > 0" class="check-list">
            <div v-for="(item, idx) in riskItems" :key="idx" class="check-item risk">
              <p v-if="item.line > 0"><strong>起始行号：</strong>{{ item.line }}</p>
              <p><strong>规则ID：</strong>{{ item.ruleID }}</p>
              <p><strong>风险名称：</strong>{{ item.name }}</p>
              <p><strong>严重级别：</strong>{{ item.severity }}</p>
              <p><strong>说明：</strong>{{ item.description }}</p>
              <p><strong>建议：</strong>{{ item.suggestion }}</p>
              <pre v-if="item.sql">{{ item.sql }}</pre>
            </div>
          </div>
        </div>
      </div>
    </template>

    <template v-else>
      <div class="home-center-wrap">
        <div class="card home-center-card">
          <h2>DDL 规范检查结果</h2>
          <p class="result">{{ ddlMessage }}</p>
          <div v-if="ddlIssues.length > 0" class="check-list">
            <div v-for="(item, idx) in ddlIssues" :key="idx" class="check-item ddl">
              <p><strong>对象类型：</strong>{{ item.objectType }}</p>
              <p><strong>对象名称：</strong>{{ item.objectName || '-' }}</p>
              <p><strong>说明：</strong>{{ item.description }}</p>
              <p><strong>建议：</strong>{{ item.suggestion }}</p>
            </div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.mode-row {
  display: flex;
  justify-content: center;
  gap: 12px;
  margin-bottom: 20px;
  flex-wrap: wrap;
}

.mode-btn {
  padding: 10px 20px;
  font-size: 16px;
  cursor: pointer;
  border: none;
  border-radius: 6px;
  background: #eef2ff;
  color: #333;
}

.mode-btn.active {
  background: #409eff;
  color: #fff;
}

.home-center-wrap {
  width: 100%;
  max-width: 980px;
  margin: 0 auto 24px;
}

.home-center-card {
  margin-bottom: 0;
}

.card {
  width: 100%;
  border: 1px solid #ddd;
  border-radius: 10px;
  padding: 24px;
  margin-bottom: 24px;
  background: #fff;
}

.db-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.module-tip {
  margin-bottom: 16px;
  padding: 12px 14px;
  line-height: 1.8;
  background: #f5f7fa;
  border-radius: 8px;
  color: #666;
}

select {
  width: 220px;
  padding: 10px 12px;
  font-size: 16px;
  border: 1px solid #ccc;
  border-radius: 6px;
  background: #fff;
}

.editor-toolbar {
  margin-bottom: 12px;
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.upload-row {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.upload-label {
  font-size: 15px;
}

.file-name {
  color: #666;
  font-size: 14px;
}

.btn-row {
  margin-top: 16px;
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

.result {
  margin-bottom: 12px;
  font-size: 15px;
}

.check-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.check-item {
  border-radius: 8px;
  padding: 16px;
  line-height: 1.8;
}

.error {
  background: #fff5f5;
  border: 1px solid #f5c2c7;
}

.risk {
  background: #fffaf0;
  border: 1px solid #f6d8a8;
}

.ddl {
  background: #f5faff;
  border: 1px solid #b6d7ff;
}

pre {
  margin-top: 10px;
  padding: 12px;
  background: #2d2d2d;
  color: #f8f8f2;
  border-radius: 6px;
  white-space: pre-wrap;
  word-break: break-word;
  overflow-x: auto;
}
</style>