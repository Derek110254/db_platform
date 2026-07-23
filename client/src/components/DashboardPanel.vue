<script setup lang="ts">
/**
 * DashboardPanel.vue
 * ------------------------------------------------------------------
 * 该组件是「年度工作量看板」页面。
 *
 * 主要功能：
 * 1. 展示选定年度的统计卡片（SQL 审核、变更申请、数据同步、告警处理、运维变更等数量）。
 * 2. 月度趋势表（按月份×分类聚合的工作量）。
 * 3. 各分类分布（变更类型 / 数据库类型 / 操作类型 / 告警分类等）。
 * 4. 工作量排行榜（按提交人/处理人排序）。
 *
 * 关键接口：
 * - GET /api/dashboard/yearly  获取年度看板数据
 */

import { onMounted, ref, computed } from 'vue'

interface Cards {
  sqlAuditCount: number
  auditSubmitCount: number
  auditPassedCount: number
  changeReqCount: number
  verifiedChangeCount: number
  syncReqCount: number
  alertHandleCount: number
  opsChangeCount: number
}

interface MonthItem {
  month: string
  sqlAudit: number
  changeReq: number
  alert: number
  opsChange: number
}

interface NameValue {
  name: string
  value: number
}

const loading = ref(true)
const cards = ref<Cards>({
  sqlAuditCount: 0, auditSubmitCount: 0, auditPassedCount: 0,
  changeReqCount: 0, verifiedChangeCount: 0, syncReqCount: 0,
  alertHandleCount: 0, opsChangeCount: 0,
})
const monthly = ref<MonthItem[]>([])
const alertCategories = ref<NameValue[]>([])
const opsChangeTypes = ref<NameValue[]>([])
const opsChangeResults = ref<NameValue[]>([])
const topUsers = ref<NameValue[]>([])

const currentYear = new Date().getFullYear()

const auditPassRate = computed(() => {
  if (cards.value.auditSubmitCount === 0) return 0
  return Math.round((cards.value.auditPassedCount / cards.value.auditSubmitCount) * 100)
})

const maxMonthly = computed(() => {
  let max = 0
  for (const m of monthly.value) {
    const total = m.sqlAudit + m.changeReq + m.alert + m.opsChange
    if (total > max) max = total
  }
  return max || 1
})

const loadData = async () => {
  loading.value = true
  try {
    const res = await fetch('/api/dashboard/yearly', { credentials: 'include' })
    const data = await res.json()
    if (data.ok) {
      cards.value = data.cards || cards.value
      monthly.value = data.monthly || []
      alertCategories.value = data.alertCategories || []
      opsChangeTypes.value = data.opsChangeTypes || []
      opsChangeResults.value = data.opsChangeResults || []
      topUsers.value = data.topUsers || []
    }
  } catch {
    // 静默失败
  } finally {
    loading.value = false
  }
}

const maxAlertCat = computed(() => Math.max(1, ...alertCategories.value.map(a => a.value)))
const maxOpsType = computed(() => Math.max(1, ...opsChangeTypes.value.map(a => a.value)))
const maxOpsResult = computed(() => Math.max(1, ...opsChangeResults.value.map(a => a.value)))
const maxTopUser = computed(() => Math.max(1, ...topUsers.value.map(a => a.value)))

const colors = ['#409eff', '#67c23a', '#e6a23c', '#f56c6c', '#909399', '#8e44ad', '#20b2aa', '#ff8c00']

onMounted(() => {
  loadData()
})
</script>

<template>
  <div class="dashboard-page">
    <div v-if="loading" class="loading-tip">加载中...</div>

    <template v-else>
      <!-- 标题 -->
      <div class="dash-title">
        <h2>{{ currentYear }} 年统计</h2>
      </div>

      <!-- 数字卡片 -->
      <div class="cards-grid">
        <div class="dash-card" style="--c: #2e4f70">
          <div class="dash-num">{{ cards.sqlAuditCount }}</div>
          <div class="dash-label">SQL 性能检测</div>
        </div>
        <div class="dash-card" style="--c: #67c23a">
          <div class="dash-num">{{ cards.auditSubmitCount }}</div>
          <div class="dash-label">SQL 审核提交</div>
        </div>
        <div class="dash-card" style="--c: #67c23a">
          <div class="dash-num">{{ cards.auditPassedCount }}</div>
          <div class="dash-label">SQL 审核通过</div>
          <div class="dash-sub">通过率 {{ auditPassRate }}%</div>
        </div>
        <div class="dash-card" style="--c: #e6a23c">
          <div class="dash-num">{{ cards.changeReqCount }}</div>
          <div class="dash-label">数据库变更申请</div>
          <div class="dash-sub">已验证 {{ cards.verifiedChangeCount }}</div>
        </div>
        <div class="dash-card" style="--c: #8e44ad">
          <div class="dash-num">{{ cards.syncReqCount }}</div>
          <div class="dash-label">数据同步申请</div>
        </div>
        <div class="dash-card" style="--c: #f56c6c">
          <div class="dash-num">{{ cards.alertHandleCount }}</div>
          <div class="dash-label">数据库告警处理</div>
        </div>
        <div class="dash-card" style="--c: #909399">
          <div class="dash-num">{{ cards.opsChangeCount }}</div>
          <div class="dash-label">运维变更记录</div>
        </div>
        <div class="dash-card" style="--c: #20b2aa">
          <div class="dash-num">{{ topUsers[0]?.value ?? 0 }}</div>
          <div class="dash-label">最多检测用户</div>
          <div class="dash-sub">{{ topUsers[0]?.name ?? '-' }}</div>
        </div>
      </div>

      <!-- 月度趋势 -->
      <div class="section-card">
        <h3>月度趋势</h3>
        <div class="monthly-table-wrap">
          <table class="monthly-table">
            <thead>
              <tr>
                <th>月份</th>
                <th>SQL检测</th>
                <th>变更申请</th>
                <th>告警处理</th>
                <th>运维变更</th>
                <th>合计</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(m, i) in monthly" :key="i">
                <td class="month-cell">{{ m.month }}</td>
                <td>
                  <span class="bar-cell" :style="{ width: (maxMonthly > 0 ? (m.sqlAudit / maxMonthly * 100) : 0) + '%' }"></span>
                  {{ m.sqlAudit }}
                </td>
                <td>
                  <span class="bar-cell bar-blue" :style="{ width: (maxMonthly > 0 ? (m.changeReq / maxMonthly * 100) : 0) + '%' }"></span>
                  {{ m.changeReq }}
                </td>
                <td>
                  <span class="bar-cell bar-red" :style="{ width: (maxMonthly > 0 ? (m.alert / maxMonthly * 100) : 0) + '%' }"></span>
                  {{ m.alert }}
                </td>
                <td>
                  <span class="bar-cell bar-green" :style="{ width: (maxMonthly > 0 ? (m.opsChange / maxMonthly * 100) : 0) + '%' }"></span>
                  {{ m.opsChange }}
                </td>
                <td class="total-cell">{{ m.sqlAudit + m.changeReq + m.alert + m.opsChange }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- 分类统计 -->
      <div class="two-col-grid">
        <!-- 告警分类 -->
        <div class="section-card">
          <h3>告警分类分布</h3>
          <div v-if="alertCategories.length === 0" class="empty-tip">暂无数据</div>
          <div v-else class="dist-list">
            <div v-for="(item, i) in alertCategories" :key="i" class="dist-item">
              <span class="dist-dot" :style="{ background: colors[i % colors.length] }"></span>
              <span class="dist-name">{{ item.name }}</span>
              <span class="dist-bar-wrap">
                <span class="dist-bar" :style="{ width: (item.value / maxAlertCat * 100) + '%', background: colors[i % colors.length] }"></span>
              </span>
              <span class="dist-value">{{ item.value }}</span>
            </div>
          </div>
        </div>

        <!-- 运维变更类型 -->
        <div class="section-card">
          <h3>运维变更类型分布</h3>
          <div v-if="opsChangeTypes.length === 0" class="empty-tip">暂无数据</div>
          <div v-else class="dist-list">
            <div v-for="(item, i) in opsChangeTypes" :key="i" class="dist-item">
              <span class="dist-dot" :style="{ background: colors[i % colors.length] }"></span>
              <span class="dist-name">{{ item.name }}</span>
              <span class="dist-bar-wrap">
                <span class="dist-bar" :style="{ width: (item.value / maxOpsType * 100) + '%', background: colors[i % colors.length] }"></span>
              </span>
              <span class="dist-value">{{ item.value }}</span>
            </div>
          </div>
        </div>

        <!-- 运维变更结果 -->
        <div class="section-card">
          <h3>运维变更结果分布</h3>
          <div v-if="opsChangeResults.length === 0" class="empty-tip">暂无数据</div>
          <div v-else class="dist-list">
            <div v-for="(item, i) in opsChangeResults" :key="i" class="dist-item">
              <span class="dist-dot" :style="{ background: colors[i % colors.length] }"></span>
              <span class="dist-name">{{ item.name }}</span>
              <span class="dist-bar-wrap">
                <span class="dist-bar" :style="{ width: (item.value / maxOpsResult * 100) + '%', background: colors[i % colors.length] }"></span>
              </span>
              <span class="dist-value">{{ item.value }}</span>
            </div>
          </div>
        </div>

        <!-- 用户排名 -->
        <div class="section-card">
          <h3>SQL 检测次数 TOP5</h3>
          <div v-if="topUsers.length === 0" class="empty-tip">暂无数据</div>
          <div v-else class="rank-list">
            <div v-for="(item, i) in topUsers" :key="i" class="rank-item">
              <span :class="['rank-num', `rank-${i + 1}`]">{{ i + 1 }}</span>
              <span class="rank-name">{{ item.name }}</span>
              <span class="rank-bar-wrap">
                <span class="rank-bar" :style="{ width: (item.value / maxTopUser * 100) + '%' }"></span>
              </span>
              <span class="rank-value">{{ item.value }} 次</span>
            </div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.dashboard-page { width: 100%; }
.loading-tip { text-align: center; padding: 40px; color: #909399; }

.dash-title { margin-bottom: 16px; }
.dash-title h2 { margin: 0; font-size: 20px; color: #303133; }

/* 数字卡片 */
.cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 16px;
  margin-bottom: 20px;
}
.dash-card {
  background: #fff;
  border: 1px solid #ebeef5;
  border-left: 4px solid var(--c, #409eff);
  border-radius: 8px;
  padding: 20px 16px;
  text-align: center;
}
.dash-card:hover { box-shadow: 0 4px 12px rgba(0,0,0,0.08); }
.dash-num { font-size: 32px; font-weight: bold; color: var(--c, #409eff); line-height: 1.2; }
.dash-label { font-size: 14px; color: #909399; margin-top: 8px; }
.dash-sub { font-size: 12px; color: #c0c4cc; margin-top: 4px; }

/* 区块卡片 */
.section-card {
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 8px;
  padding: 20px;
  margin-bottom: 16px;
}
.section-card h3 { margin: 0 0 16px; font-size: 16px; color: #303133; }

/* 两列布局 */
.two-col-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

/* 月度趋势表格 */
.monthly-table-wrap { overflow-x: auto; }
.monthly-table { width: 100%; border-collapse: collapse; font-size: 13px; }
.monthly-table th, .monthly-table td {
  border: 1px solid #ebeef5;
  padding: 8px 10px;
  text-align: left;
}
.monthly-table th { background: #f5f7fa; color: #909399; font-weight: bold; }
.monthly-table tbody tr:hover { background: #fafafa; }
.month-cell { font-weight: 600; color: #606266; width: 60px; }
.total-cell { font-weight: bold; color: #409eff; }
.monthly-table td { position: relative; }
.bar-cell {
  display: inline-block;
  height: 16px;
  background: #409eff;
  opacity: 0.2;
  border-radius: 3px;
  vertical-align: middle;
  margin-right: 6px;
  min-width: 2px;
}
.bar-blue { background: #e6a23c; }
.bar-red { background: #f56c6c; }
.bar-green { background: #67c23a; }

/* 分类分布列表 */
.dist-list { display: flex; flex-direction: column; gap: 12px; }
.dist-item { display: flex; align-items: center; gap: 8px; font-size: 13px; }
.dist-dot { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; }
.dist-name { min-width: 80px; color: #606266; }
.dist-bar-wrap { flex: 1; height: 18px; background: #f5f7fa; border-radius: 4px; overflow: hidden; }
.dist-bar { display: block; height: 100%; border-radius: 4px; transition: width 0.3s; }
.dist-value { min-width: 30px; text-align: right; font-weight: 600; color: #303133; }

/* 排名列表 */
.rank-list { display: flex; flex-direction: column; gap: 12px; }
.rank-item { display: flex; align-items: center; gap: 10px; font-size: 13px; }
.rank-num {
  width: 24px; height: 24px;
  border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  font-size: 12px; font-weight: bold; color: #fff;
  background: #c0c4cc; flex-shrink: 0;
}
.rank-1 { background: #f56c6c; }
.rank-2 { background: #e6a23c; }
.rank-3 { background: #67c23a; }
.rank-name { min-width: 80px; color: #606266; }
.rank-bar-wrap { flex: 1; height: 18px; background: #f5f7fa; border-radius: 4px; overflow: hidden; }
.rank-bar { display: block; height: 100%; background: #409eff; border-radius: 4px; }
.rank-value { min-width: 50px; text-align: right; font-weight: 600; color: #303133; }

.empty-tip { text-align: center; color: #c0c4cc; padding: 24px; font-size: 13px; }
</style>
