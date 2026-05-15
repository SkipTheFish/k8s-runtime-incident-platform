<script setup lang="ts">
import { computed } from "vue";
import type { IncidentStatus } from "../../types/incident";

const props = defineProps<{
  status: IncidentStatus;
}>();

const label = computed(() => {
  return props.status === "open"
    ? "处理中"
    : "已恢复";
});
</script>

<template>
  <n-tag
    size="small"
    round
    :bordered="false"
    :type="status === 'open' ? 'error' : 'success'"
  >
    {{ label }}
  </n-tag>
</template>