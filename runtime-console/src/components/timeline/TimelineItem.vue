<script setup lang="ts">
import type { TimelineEvent } from "../../types/incident";
import SeverityBadge from "../incident/SeverityBadge.vue";
import {
  getTimelineEventIcon,
  getTimelineEventLabel,
  getTimelineTypeLabel,
} from "../../utils/timeline";

const props = defineProps<{
  item: TimelineEvent;
  isLast?: boolean;
}>();

function formatTime(value: string) {
  const date = new Date(value);

  return date.toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function formatDate(value: string) {
  const date = new Date(value);

  return date.toLocaleDateString();
}

function getDotClass(severity: string) {
  if (severity === "critical" || severity === "error") {
    return "dot-error";
  }

  if (severity === "warning") {
    return "dot-warning";
  }

  return "dot-info";
}
</script>

<template>
  <div class="timeline-item">
    <div class="timeline-left">
      <div class="timeline-time">
        <div class="time-main">
          {{ formatTime(props.item.occurred_at) }}
        </div>

        <div class="time-date">
          {{ formatDate(props.item.occurred_at) }}
        </div>
      </div>

      <div
        class="timeline-line"
        :class="{ last: props.isLast }"
      >
        <div
          class="timeline-dot"
          :class="getDotClass(props.item.severity)"
        >
          <span>
            {{ getTimelineEventIcon(props.item.event_type) }}
          </span>
        </div>
      </div>
    </div>

    <div class="timeline-body">
      <div class="timeline-top">
        <div>
          <div class="event-title">
            {{ getTimelineEventLabel(props.item.event_type) }}
          </div>

          <div class="event-type">
            {{ props.item.event_type }}
          </div>
        </div>

        <SeverityBadge :severity="props.item.severity" />
      </div>

      <div class="timeline-message">
        {{ props.item.message }}
      </div>

      <div class="timeline-footer">
        <n-tag size="small" round :bordered="false">
          {{ getTimelineTypeLabel(props.item.timeline_type) }}
        </n-tag>

        <n-tag
          v-if="props.item.resource_kind || props.item.resource_name"
          size="small"
          round
          :bordered="false"
        >
          {{ props.item.resource_kind }}/{{ props.item.resource_name }}
        </n-tag>
      </div>
    </div>
  </div>
</template>

<style scoped>
.timeline-item {
  display: grid;
  grid-template-columns: 140px 1fr;
  gap: 18px;
  padding: 18px 0;
}

.timeline-left {
  display: grid;
  grid-template-columns: 92px 1fr;
  gap: 14px;
}

.timeline-time {
  padding-top: 2px;
  text-align: right;
}

.time-main {
  font-size: 13px;
  font-weight: 700;
  color: #e5e7eb;
}

.time-date {
  margin-top: 4px;
  font-size: 12px;
  color: #64748b;
}

.timeline-line {
  position: relative;
  display: flex;
  justify-content: center;
}

.timeline-line::after {
  content: "";
  position: absolute;
  top: 34px;
  bottom: -26px;
  width: 1px;
  background: rgba(148, 163, 184, 0.24);
}

.timeline-line.last::after {
  display: none;
}

.timeline-dot {
  position: relative;
  z-index: 1;
  width: 34px;
  height: 34px;
  margin-top: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  box-shadow: 0 0 0 6px rgba(15, 23, 42, 1);
  font-size: 15px;
}

.dot-error {
  background: rgba(239, 68, 68, 0.18);
  border: 1px solid rgba(239, 68, 68, 0.6);
}

.dot-warning {
  background: rgba(245, 158, 11, 0.18);
  border: 1px solid rgba(245, 158, 11, 0.6);
}

.dot-info {
  background: rgba(59, 130, 246, 0.18);
  border: 1px solid rgba(59, 130, 246, 0.6);
}

.timeline-body {
  padding: 18px;
  border: 1px solid rgba(148, 163, 184, 0.16);
  border-radius: 16px;
  background:
    linear-gradient(
      135deg,
      rgba(30, 41, 59, 0.78),
      rgba(15, 23, 42, 0.82)
    );
}

.timeline-top {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
}

.event-title {
  font-size: 15px;
  font-weight: 800;
  color: #f8fafc;
}

.event-type {
  margin-top: 4px;
  font-size: 12px;
  color: #64748b;
  font-family:
    ui-monospace,
    SFMono-Regular,
    Menlo,
    Monaco,
    Consolas,
    "Liberation Mono",
    monospace;
}

.timeline-message {
  margin-top: 14px;
  font-size: 15px;
  line-height: 1.7;
  color: #cbd5e1;
}

.timeline-footer {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 14px;
}

@media (max-width: 720px) {
  .timeline-item {
    grid-template-columns: 1fr;
    gap: 10px;
  }

  .timeline-left {
    grid-template-columns: 92px 40px;
  }

  .timeline-time {
    text-align: left;
  }
}
</style>
