<script setup lang="ts">
import type { IncidentSeverity } from "../../types/incident";

const props = defineProps<{
  severity: IncidentSeverity | "error";
}>();

const config = {
  critical: {
    type: "error",
    text: "严重",
  },

  warning: {
    type: "warning",
    text: "警告",
  },

  info: {
    type: "info",
    text: "信息",
  },

  error: {
    type: "error",
    text: "错误",
  },
} as const;
</script>

<template>
  <n-tag
    size="small"
    round
    :bordered="false"
    :type="config[props.severity]?.type || 'default'"
  >
    {{ config[props.severity]?.text || props.severity }}
  </n-tag>
</template>