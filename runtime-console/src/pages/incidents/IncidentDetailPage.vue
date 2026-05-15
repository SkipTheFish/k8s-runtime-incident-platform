<script setup lang="ts">
import { computed, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useIncidentStore } from "../../stores/incident";

import SeverityBadge from "../../components/incident/SeverityBadge.vue";
import IncidentStatusBadge from "../../components/incident/IncidentStatusBadge.vue";
import TimelineList from "../../components/timeline/TimelineList.vue";
import { buildIncidentStory } from "../../utils/timeline";

const route = useRoute();
const router = useRouter();
const store = useIncidentStore();

const incidentId = computed(() => {
  return String(route.params.id);
});

onMounted(async () => {
  await Promise.all([
    store.loadIncidentDetail(incidentId.value),
    store.loadTimeline(incidentId.value),
  ]);
});

function formatTime(value?: string) {
  if (!value) return "-";
  return new Date(value).toLocaleString();
}

const resourceTitle = computed(() => {
  const item = store.currentIncident;

  if (!item) return "-";

  return `${item.resource_kind}/${item.resource_name}`;
});

const story = computed(() => {
  return buildIncidentStory(store.timeline);
});

const timelineCount = computed(() => {
  return store.timeline.length;
});

const runtimeEventCount = computed(() => {
  return store.timeline.filter((item) => item.timeline_type === "RUNTIME").length;
});

const systemEventCount = computed(() => {
  return store.timeline.filter((item) => item.timeline_type === "SYSTEM").length;
});
</script>

<template>
  <div class="page">
    <div class="detail-topbar">
      <n-button
        quaternary
        @click="router.push('/incidents')"
      >
        ← 返回事故列表
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

    <n-spin :show="store.loadingDetail">
      <div
        v-if="store.currentIncident"
        class="incident-hero console-card"
      >
        <div class="hero-main">
          <div class="hero-kicker">
            运行时事故 / Runtime Incident
          </div>

          <h1 class="hero-title">
            {{ resourceTitle }}
          </h1>

          <p class="hero-desc">
            该事故由后端 Correlation Engine 根据 Kubernetes Runtime Event
            与 Prometheus Alert 自动关联生成。
          </p>

          <div class="hero-meta">
            <SeverityBadge
              :severity="store.currentIncident.severity"
            />

            <IncidentStatusBadge
              :status="store.currentIncident.status"
            />

            <n-tag
              size="small"
              round
              :bordered="false"
            >
              Incident #{{ store.currentIncident.id }}
            </n-tag>

            <n-tag
              v-if="store.currentIncident.namespace"
              size="small"
              round
              :bordered="false"
            >
              Namespace: {{ store.currentIncident.namespace }}
            </n-tag>
          </div>
        </div>

        <div class="hero-stats">
          <div class="stat-card">
            <div class="stat-label">
              开始时间
            </div>

            <div class="stat-value">
              {{ formatTime(store.currentIncident.started_at) }}
            </div>
          </div>

          <div class="stat-card">
            <div class="stat-label">
              更新时间
            </div>

            <div class="stat-value">
              {{ formatTime(store.currentIncident.updated_at) }}
            </div>
          </div>

          <div class="stat-card">
            <div class="stat-label">
              告警策略
            </div>

            <div class="stat-value">
              {{ store.currentIncident.policy_name || "-" }}
            </div>
          </div>
        </div>
      </div>
    </n-spin>

    <div class="story-card console-card">
      <div class="story-left">
        <div class="story-kicker">
          Incident Narrative
        </div>

        <h2>
          事故叙事摘要
        </h2>

        <p>
          {{ story }}
        </p>
      </div>

      <div class="story-metrics">
        <div class="story-metric">
          <div class="story-number">
            {{ timelineCount }}
          </div>
          <div class="story-label">
            时间线事件
          </div>
        </div>

        <div class="story-metric">
          <div class="story-number">
            {{ runtimeEventCount }}
          </div>
          <div class="story-label">
            Runtime 事件
          </div>
        </div>

        <div class="story-metric">
          <div class="story-number">
            {{ systemEventCount }}
          </div>
          <div class="story-label">
            System 事件
          </div>
        </div>
      </div>
    </div>

    <div class="section-header">
      <h2 style="color: #000000;">事故时间线</h2>

      <p>
        基于 Alert 与 Kubernetes Runtime Event 自动生成的事故叙事时间线。
      </p>
    </div>

    <div class="console-card timeline-card">
      <TimelineList
        :items="store.timeline"
        :loading="store.loadingTimeline"
      />
    </div>
  </div>
</template>

<style scoped>
.detail-topbar {
  margin-bottom: 16px;
}

.incident-hero {
  display: flex;
  justify-content: space-between;
  gap: 28px;
  padding: 28px;
}

.hero-main {
  flex: 1;
}

.hero-kicker {
  margin-bottom: 8px;
  font-size: 13px;
  color: #94a3b8;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.hero-title {
  margin: 0;
  font-size: 34px;
  color: #f8fafc;
}

.hero-desc {
  max-width: 720px;
  margin: 12px 0 0;
  color: #94a3b8;
  line-height: 1.7;
}

.hero-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 18px;
}

.hero-stats {
  min-width: 320px;
  display: grid;
  grid-template-columns: 1fr;
  gap: 12px;
}

.stat-card {
  padding: 14px 16px;
  border-radius: 14px;
  background: rgba(30, 41, 59, 0.72);
  border: 1px solid rgba(148, 163, 184, 0.14);
}

.stat-label {
  font-size: 12px;
  color: #94a3b8;
}

.stat-value {
  margin-top: 6px;
  font-size: 14px;
  font-weight: 700;
  color: #f8fafc;
  word-break: break-all;
}

.story-card {
  display: flex;
  justify-content: space-between;
  gap: 28px;
  margin-top: 22px;
  padding: 24px 28px;
  /* 修改这里：将深色放前面，浅色放后面 */
  background: 
    linear-gradient(
      135deg, 
      rgba(15, 23, 42, 0.9) 0%,   /* 左上角：深色（增加到了0.9不透明度） */
      rgba(59, 130, 246, 0.12) 100% /* 右下角：浅蓝渐变 */
    );
}

.story-left {
  flex: 1;
}

.story-kicker {
  font-size: 12px;
  color: #93c5fd;
  text-transform: uppercase;
  letter-spacing: 0.12em;
}

.story-left h2 {
  margin: 8px 0 0;
  color: #f8fafc;
  font-size: 22px;
}

.story-left p {
  margin: 12px 0 0;
  color: #cbd5e1;
  line-height: 1.8;
}

.story-metrics {
  min-width: 330px;
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}

.story-metric {
  padding: 16px;
  border-radius: 14px;
  background: rgba(15, 23, 42, 0.62);
  border: 1px solid rgba(148, 163, 184, 0.14);
  text-align: center;
}

.story-number {
  font-size: 28px;
  font-weight: 800;
  color: #f8fafc;
}

.story-label {
  margin-top: 4px;
  font-size: 12px;
  color: #94a3b8;
}

.section-header {
  margin: 28px 0 14px;
}

.section-header h2 {
  margin: 0;
  color: #f8fafc;
  font-size: 22px;
}

.section-header p {
  margin: 6px 0 0;
  color: #94a3b8;
}

.timeline-card {
  padding: 20px 24px;
}

@media (max-width: 900px) {
  .incident-hero,
  .story-card {
    flex-direction: column;
  }

  .hero-stats,
  .story-metrics {
    min-width: 0;
  }
}

@media (max-width: 640px) {
  .story-metrics {
    grid-template-columns: 1fr;
  }
}
</style>