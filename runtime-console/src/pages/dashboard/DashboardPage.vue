<script setup lang="ts">
import { computed, onMounted } from "vue";
import { useRouter } from "vue-router";
import { useIncidentStore } from "../../stores/incident";
import SeverityBadge from "../../components/incident/SeverityBadge.vue";
import IncidentStatusBadge from "../../components/incident/IncidentStatusBadge.vue";

const router = useRouter();
const store = useIncidentStore();

onMounted(() => {
  store.loadIncidents();
});

const openCount = computed(() => {
  return store.incidents.filter((item) => item.status === "open").length;
});

const criticalCount = computed(() => {
  return store.incidents.filter((item) => item.severity === "critical").length;
});

const warningCount = computed(() => {
  return store.incidents.filter((item) => item.severity === "warning").length;
});

const resolvedCount = computed(() => {
  return store.incidents.filter((item) => item.status === "resolved").length;
});

const recentIncidents = computed(() => {
  return [...store.incidents]
    .sort((a, b) => {
      const ta = new Date(a.started_at || 0).getTime();
      const tb = new Date(b.started_at || 0).getTime();
      return tb - ta;
    })
    .slice(0, 5);
});

function formatTime(value?: string) {
  if (!value) return "-";
  return new Date(value).toLocaleString();
}

function goDetail(id: number) {
  router.push(`/incidents/${id}`);
}
</script>

<template>
  <div class="page">
    <div class="page-header dashboard-header">
      <div>
        <h1 class="page-title">运行时可观测性总览</h1>
        <p class="page-description">
          聚合 Kubernetes Runtime Event、Prometheus Alert 与 Incident Correlation 的实时事故视图。
        </p>
      </div>

      <n-button type="primary" @click="router.push('/incidents')">
        查看事故列表
      </n-button>
    </div>

    <n-alert
      v-if="store.error"
      type="error"
      :bordered="false"
      style="margin-bottom: 16px"
    >
      {{ store.error }}
    </n-alert>

    <n-spin :show="store.loadingList">
      <div class="metric-grid">
        <div class="metric-card danger">
          <div class="metric-label">处理中事故</div>
          <div class="metric-value">{{ openCount }}</div>
          <div class="metric-desc">当前仍未恢复的 Runtime Incident</div>
        </div>

        <div class="metric-card critical">
          <div class="metric-label">严重事故</div>
          <div class="metric-value">{{ criticalCount }}</div>
          <div class="metric-desc">Severity 为 critical 的事故数量</div>
        </div>

        <div class="metric-card warning">
          <div class="metric-label">警告事故</div>
          <div class="metric-value">{{ warningCount }}</div>
          <div class="metric-desc">Severity 为 warning 的事故数量</div>
        </div>

        <div class="metric-card success">
          <div class="metric-label">已恢复事故</div>
          <div class="metric-value">{{ resolvedCount }}</div>
          <div class="metric-desc">已进入 resolved 状态的事故</div>
        </div>
      </div>

      <div class="dashboard-section">
        <div class="section-title-row">
          <div>
            <h2>最近触发的事故</h2>
            <p>按开始时间倒序展示最新 Runtime Incident。</p>
          </div>
        </div>

        <div class="console-card recent-card">
          <n-empty
            v-if="recentIncidents.length === 0"
            description="暂无事故数据"
          />

          <div
            v-for="item in recentIncidents"
            :key="item.id"
            class="recent-item"
            @click="goDetail(item.id)"
          >
            <div class="recent-main">
              <div class="recent-resource">
                {{ item.resource_kind }}/{{ item.resource_name }}
              </div>

              <div class="recent-sub">
                {{ item.namespace || "default" }} · {{ formatTime(item.started_at) }}
              </div>
            </div>

            <div class="recent-badges">
              <SeverityBadge :severity="item.severity" />
              <IncidentStatusBadge :status="item.status" />
            </div>
          </div>
        </div>
      </div>
    </n-spin>
  </div>
</template>

<style scoped>
.dashboard-header {
  display: flex;
  justify-content: space-between;
  gap: 24px;
  align-items: center;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 18px;
}

.metric-card {
  padding: 22px;
  border-radius: 18px;
  border: 1px solid rgba(148, 163, 184, 0.16);
  background: rgba(15, 23, 42, 0.72);
  box-shadow: 0 20px 70px rgba(0, 0, 0, 0.22);
}

.metric-label {
  font-size: 13px;
  color: #94a3b8;
}

.metric-value {
  margin-top: 12px;
  font-size: 38px;
  font-weight: 800;
  color: #f8fafc;
}

.metric-desc {
  margin-top: 8px;
  font-size: 13px;
  color: #94a3b8;
}

.metric-card.danger {
  box-shadow: 0 0 40px rgba(239, 68, 68, 0.14);
}

.metric-card.critical {
  box-shadow: 0 0 40px rgba(248, 113, 113, 0.12);
}

.metric-card.warning {
  box-shadow: 0 0 40px rgba(245, 158, 11, 0.12);
}

.metric-card.success {
  box-shadow: 0 0 40px rgba(34, 197, 94, 0.1);
}

.dashboard-section {
  margin-top: 28px;
}

.section-title-row {
  margin-bottom: 14px;
}

.section-title-row h2 {
  margin: 0;
  color: #f8fafc;
  font-size: 22px;
}

.section-title-row p {
  margin: 6px 0 0;
  color: #94a3b8;
}

.recent-card {
  padding: 8px;
}

.recent-item {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: center;
  padding: 16px 18px;
  border-radius: 14px;
  cursor: pointer;
}

.recent-item:hover {
  background: rgba(30, 41, 59, 0.72);
}

.recent-resource {
  color: #f8fafc;
  font-weight: 700;
}

.recent-sub {
  margin-top: 4px;
  color: #94a3b8;
  font-size: 13px;
}

.recent-badges {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

@media (max-width: 980px) {
  .metric-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .dashboard-header {
    align-items: flex-start;
    flex-direction: column;
  }
}

@media (max-width: 640px) {
  .metric-grid {
    grid-template-columns: 1fr;
  }

  .recent-item {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>