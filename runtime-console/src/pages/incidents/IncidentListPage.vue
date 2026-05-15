<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useIncidentStore } from "../../stores/incident";
import IncidentTable from "../../components/incident/IncidentTable.vue";

const store = useIncidentStore();

const keyword = ref("");
const severity = ref<string | null>(null);
const status = ref<string | null>(null);

onMounted(() => {
  store.loadIncidents();
});

const severityOptions = [
  {
    label: "全部级别",
    value: "all",
  },
  {
    label: "严重",
    value: "critical",
  },
  {
    label: "警告",
    value: "warning",
  },
  {
    label: "信息",
    value: "info",
  },
];

const statusOptions = [
  {
    label: "全部状态",
    value: "all",
  },
  {
    label: "处理中",
    value: "open",
  },
  {
    label: "已恢复",
    value: "resolved",
  },
];

const filteredIncidents = computed(() => {
  return store.incidents.filter((item) => {
    const text = [
      item.resource_kind,
      item.resource_name,
      item.namespace,
      item.policy_name,
      item.status,
      item.severity,
    ]
      .filter(Boolean)
      .join(" ")
      .toLowerCase();

    const keywordMatched =
      keyword.value.trim() === "" ||
      text.includes(keyword.value.trim().toLowerCase());

    const severityMatched =
      !severity.value ||
      severity.value === "all" ||
      item.severity === severity.value;

    const statusMatched =
      !status.value ||
      status.value === "all" ||
      item.status === status.value;

    return keywordMatched && severityMatched && statusMatched;
  });
});

function resetFilters() {
  keyword.value = "";
  severity.value = null;
  status.value = null;
}
</script>

<template>
  <div class="page">
    <div class="page-header">
      <h1 class="page-title">
        运行时事故
      </h1>

      <p class="page-description">
        基于 Kubernetes Warning Event 与 Prometheus Alert 关联生成的运行时事故流。
        点击任意事故可查看完整 Timeline Narrative。
      </p>
    </div>

    <n-alert
      v-if="store.error"
      type="error"
      :bordered="false"
      style="margin-bottom: 16px"
    >
      {{ store.error }}
    </n-alert>

    <div class="filter-card">
      <div class="filter-left">
        <n-input
          v-model:value="keyword"
          clearable
          placeholder="搜索资源名、Namespace、Policy..."
          class="filter-search"
        />

        <n-select
          v-model:value="severity"
          clearable
          placeholder="严重级别"
          :options="severityOptions"
          class="filter-select"
        />

        <n-select
          v-model:value="status"
          clearable
          placeholder="事故状态"
          :options="statusOptions"
          class="filter-select"
        />
      </div>

      <div class="filter-right">
        <span class="result-count">
          共 {{ filteredIncidents.length }} 条事故
        </span>

        <n-button quaternary @click="resetFilters">
          重置
        </n-button>
      </div>
    </div>

    <div class="console-card">
      <IncidentTable
        :incidents="filteredIncidents"
        :loading="store.loadingList"
      />
    </div>
  </div>
</template>

<style scoped>
.filter-card {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: center;
  margin-bottom: 16px;
  padding: 16px;
  border-radius: 18px;
  border: 1px solid rgba(148, 163, 184, 0.16);
  background: rgba(15, 23, 42, 0.72);
}

.filter-left {
  display: flex;
  gap: 12px;
  align-items: center;
  flex: 1;
}

.filter-search {
  max-width: 360px;
}

.filter-select {
  width: 150px;
}

.filter-right {
  display: flex;
  gap: 12px;
  align-items: center;
}

.result-count {
  color: #94a3b8;
  font-size: 13px;
}

@media (max-width: 860px) {
  .filter-card {
    align-items: stretch;
    flex-direction: column;
  }

  .filter-left {
    align-items: stretch;
    flex-direction: column;
  }

  .filter-search,
  .filter-select {
    width: 100%;
    max-width: none;
  }

  .filter-right {
    justify-content: space-between;
  }
}
</style>