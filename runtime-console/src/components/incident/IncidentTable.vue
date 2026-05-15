<script setup lang="ts">
import { h } from "vue";
import { useRouter } from "vue-router";
import { NDataTable, NSpace, NText, NTag } from "naive-ui";
import type { DataTableColumns } from "naive-ui";
import type { Incident } from "../../types/incident";
import SeverityBadge from "./SeverityBadge.vue";
import IncidentStatusBadge from "./IncidentStatusBadge.vue";

const props = defineProps<{
  incidents: Incident[];
  loading: boolean;
}>();

const router = useRouter();

function formatTime(value?: string) {
  if (!value) return "-";
  return new Date(value).toLocaleString();
}

function formatDuration(value?: string) {
  if (!value) return "-";

  const start = new Date(value).getTime();
  const now = Date.now();
  const diff = Math.max(0, now - start);

  const minutes = Math.floor(diff / 1000 / 60);
  const hours = Math.floor(minutes / 60);

  if (minutes < 1) return "刚刚";
  if (minutes < 60) return `${minutes} 分钟`;
  if (hours < 24) return `${hours} 小时`;

  return `${Math.floor(hours / 24)} 天`;
}

const columns: DataTableColumns<Incident> = [
  {
    title: "严重级别",
    key: "severity",
    width: 130,

    render(row) {
      return h(SeverityBadge, {
        severity: row.severity,
      });
    },
  },

  {
    title: "资源",
    key: "resource",

    render(row) {
      return h(
        NSpace,
        {
          vertical: true,
          size: 2,
        },

        {
          default: () => [
            h(
              "div",
              {
                class: "resource-name",
              },
              `${row.resource_kind}/${row.resource_name}`
            ),

            h(
              "div",
              {
                class: "resource-meta",
              },
              [
                h(
                  NTag,
                  {
                    size: "small",
                    round: true,
                    bordered: false,
                  },
                  {
                    default: () => row.namespace || "default",
                  }
                ),

                row.policy_name
                  ? h(
                      NText,
                      {
                        depth: 3,
                      },
                      {
                        default: () => `Policy: ${row.policy_name}`,
                      }
                    )
                  : null,
              ]
            ),
          ],
        }
      );
    },
  },

  {
    title: "状态",
    key: "status",
    width: 120,

    render(row) {
      return h(IncidentStatusBadge, {
        status: row.status,
      });
    },
  },

  {
    title: "持续时间",
    key: "duration",
    width: 120,

    render(row) {
      return formatDuration(row.started_at);
    },
  },

  {
    title: "开始时间",
    key: "started_at",
    width: 220,

    render(row) {
      return formatTime(row.started_at);
    },
  },
];

function rowProps(row: Incident) {
  return {
    class: "clickable-row",

    onClick: () => {
      router.push(`/incidents/${row.id}`);
    },
  };
}
</script>

<template>
  <n-data-table
    :columns="columns"
    :data="props.incidents"
    :loading="props.loading"
    :row-props="rowProps"
    :bordered="false"
    :single-line="false"
  />
</template>

<style scoped>
.resource-meta {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-top: 4px;
}
</style>