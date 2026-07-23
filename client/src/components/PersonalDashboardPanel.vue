<script setup lang="ts">
/**
 * PersonalDashboardPanel.vue
 * ------------------------------------------------------------------
 * 该组件是「个人看板」页面。
 *
 * 主要功能：
 * 1. 展示当前登录用户的工作量卡片（作为申请人 / 处理人 / 操作人 / 审核人的各类数量）。
 * 2. 展示待办事项（待发布变更、待验证变更、待执行数据同步、待复核运维变更、待审核 SQL 等）。
 * 3. 双击待办卡片可跳转到对应管理页面（通过 emit('navigate') 通知父组件路由切换）。
 *
 * 关键接口：
 * - GET /api/dashboard/personal  获取个人看板数据
 */

import { onMounted, ref, computed } from 'vue'

const emit = defineEmits<{
  (e: 'navigate', page: string): void
}>()

const loading = ref(true)

const stats = ref({
  sqlAuditCount: 0,
  changeProdCount: 0,
  syncHandleCount: 0,
  alertHandleCount: 0,
  opsChangeCount: 0,
  pendingAudit: 0,
  pendingChange: 0,
  pendingSync: 0,
  pendingAlert: 0,
  pendingOpsReview: 0,
})

const currentYear = new Date().getFullYear()

const totalWorkload = computed(() =>
  stats.value.sqlAuditCount +
  stats.value.changeProdCount +
  stats.value.syncHandleCount +
  stats.value.alertHandleCount +
  stats.value.opsChangeCount
)

const totalPending = computed(() =>
  stats.value.pendingAudit +
  stats.value.pendingChange +
  stats.value.pendingSync +
  stats.value.pendingAlert +
  stats.value.pendingOpsReview
)

const loadData = async () => {
  loading.value = true
  try {
    const res = await fetch('/api/dashboard/personal', { credentials: 'include' })
    const data = await res.json()
    if (data.ok) {
      stats.value = { ...stats.value, ...data }
    }
  } catch {
    // 静默失败
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadData()
})
</script>

<template>
  <div class="personal-dashboard">
    <div v-if="loading" class="loading-tip">加载中...</div>

    <template v-else>
      <!-- 标题 -->
      <div class="pd-title">
        <h2>{{ currentYear }} 年个人统计</h2>
      </div>

      <!-- 工作量卡片 -->
      <div class="pd-section">
        <h3 class="section-title">当年工作量 <span class="section-total">合计 {{ totalWorkload }} 项</span></h3>
        <div class="cards-grid">
          <div class="work-card" style="--c: #409eff">
            <div class="work-num">{{ stats.sqlAuditCount }}</div>
            <div class="work-label">SQL 审核次数</div>
          </div>
          <div class="work-card" style="--c: #67c23a">
            <div class="work-num">{{ stats.changeProdCount }}</div>
            <div class="work-label">变更处理次数（生产）</div>
          </div>
          <div class="work-card" style="--c: #8e44ad">
            <div class="work-num">{{ stats.syncHandleCount }}</div>
            <div class="work-label">数据同步处理次数</div>
          </div>
          <div class="work-card" style="--c: #f56c6c">
            <div class="work-num">{{ stats.alertHandleCount }}</div>
            <div class="work-label">告警处理次数</div>
          </div>
          <div class="work-card" style="--c: #e6a23c">
            <div class="work-num">{{ stats.opsChangeCount }}</div>
            <div class="work-label">运维变更次数</div>
          </div>
        </div>
      </div>

      <!-- 待办事项 -->
      <div class="pd-section">
        <h3 class="section-title">
          待办事项
          <span v-if="totalPending > 0" class="badge">{{ totalPending }}</span>
          <span v-else class="section-total">暂无待办</span>
        </h3>
        <div class="todo-grid">
          <div :class="['todo-card', stats.pendingAudit > 0 ? 'todo-active' : 'todo-clickable']" @dblclick="stats.pendingAudit > 0 && emit('navigate', 'admin-audits')">
            <div class="todo-num">{{ stats.pendingAudit }}</div>
            <div class="todo-label">待审核的 SQL</div>
          </div>
          <div :class="['todo-card', stats.pendingChange > 0 ? 'todo-active' : 'todo-clickable']" @dblclick="stats.pendingChange > 0 && emit('navigate', 'admin-db-change-release')">
            <div class="todo-num">{{ stats.pendingChange }}</div>
            <div class="todo-label">待处理的数据库变更</div>
          </div>
          <div :class="['todo-card', stats.pendingSync > 0 ? 'todo-active' : 'todo-clickable']" @dblclick="stats.pendingSync > 0 && emit('navigate', 'admin-db-data-sync-requests')">
            <div class="todo-num">{{ stats.pendingSync }}</div>
            <div class="todo-label">待处理的数据库同步</div>
          </div>
          <div :class="['todo-card', stats.pendingAlert > 0 ? 'todo-active' : 'todo-clickable']" @dblclick="stats.pendingAlert > 0 && emit('navigate', 'admin-db-alert-handles')">
            <div class="todo-num">{{ stats.pendingAlert }}</div>
            <div class="todo-label">待处理的告警</div>
          </div>
          <div :class="['todo-card', stats.pendingOpsReview > 0 ? 'todo-active' : 'todo-clickable']" @dblclick="stats.pendingOpsReview > 0 && emit('navigate', 'admin-ops-change-records')">
            <div class="todo-num">{{ stats.pendingOpsReview }}</div>
            <div class="todo-label">待复核的运维变更</div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.personal-dashboard { width: 100%; }
.loading-tip { text-align: center; padding: 40px; color: #909399; }

.pd-title { margin-bottom: 16px; }
.pd-title h2 { margin: 0; font-size: 20px; color: #303133; }

.pd-section { margin-bottom: 24px; }
.section-title {
  margin: 0 0 12px;
  font-size: 16px;
  color: #303133;
  display: flex;
  align-items: center;
  gap: 8px;
}
.section-total {
  font-size: 13px;
  color: #909399;
  font-weight: normal;
}
.badge {
  display: inline-block;
  min-width: 20px;
  padding: 2px 8px;
  border-radius: 10px;
  background: #f56c6c;
  color: #fff;
  font-size: 12px;
  font-weight: bold;
  text-align: center;
}

/* 工作量卡片 */
.cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 16px;
}
.work-card {
  background: #fff;
  border: 1px solid #ebeef5;
  border-left: 4px solid var(--c, #409eff);
  border-radius: 8px;
  padding: 20px 16px;
  text-align: center;
}
.work-card:hover { box-shadow: 0 4px 12px rgba(0,0,0,0.08); }
.work-num { font-size: 32px; font-weight: bold; color: var(--c, #409eff); line-height: 1.2; }
.work-label { font-size: 14px; color: #909399; margin-top: 8px; }

/* 待办事项卡片 */
.todo-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 16px;
}
.todo-card {
  background: #fdf6ec;
  border: 1px solid #f5dab1;
  border-top: 4px solid #e6a23c;
  border-radius: 8px;
  padding: 20px 16px;
  text-align: center;
  opacity: 0.6;
}
.todo-card.todo-active {
  opacity: 1;
  border-color: #e6a23c;
  border-top: 4px solid #f56c6c;
  background: #fef0f0;
}
.todo-card:hover { box-shadow: 0 4px 12px rgba(0,0,0,0.08); }
.todo-num { font-size: 32px; font-weight: bold; color: #303133; line-height: 1.2; }
.todo-card.todo-active .todo-num { color: #e6a23c; }
.todo-label { font-size: 14px; color: #909399; margin-top: 8px; }
</style>
