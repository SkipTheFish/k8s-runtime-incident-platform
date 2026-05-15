<script setup lang="ts">
import type { TimelineEvent } from "../../types/incident";
import TimelineItem from "./TimelineItem.vue";

const props = defineProps<{
  items: TimelineEvent[];
  loading: boolean;
}>();
</script>

<template>
  <div class="timeline-wrapper">
    <n-spin :show="props.loading">
      <n-empty
        v-if="!props.loading && props.items.length === 0"
        description="暂无时间线事件"
      />

      <TimelineItem
        v-for="(item, index) in props.items"
        :key="item.id"
        :item="item"
        :is-last="index === props.items.length - 1"
      />
    </n-spin>
  </div>
</template>

<style scoped>
.timeline-wrapper {
  padding: 8px 4px 4px;
}
</style>