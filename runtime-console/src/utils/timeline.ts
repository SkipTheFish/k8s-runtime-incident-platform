import type { TimelineEvent } from "../types/incident";

export function getTimelineEventLabel(eventType: string) {
  if (eventType.includes("alert")) return "告警触发";
  if (eventType.includes("kubernetes")) return "Kubernetes 事件";
  if (eventType.includes("image")) return "镜像拉取异常";
  if (eventType.includes("runtime")) return "运行时事件";
  if (eventType.includes("resolved")) return "事故恢复";

  return "系统事件";
}

export function getTimelineEventIcon(eventType: string) {
  if (eventType.includes("alert")) return "🚨";
  if (eventType.includes("kubernetes")) return "☸️";
  if (eventType.includes("image")) return "🧱";
  if (eventType.includes("runtime")) return "⚙️";
  if (eventType.includes("resolved")) return "✅";

  return "📌";
}

export function getTimelineTypeLabel(type: string) {
  if (type === "SYSTEM") return "系统";
  if (type === "RUNTIME") return "运行时";

  return type;
}

export function buildIncidentStory(items: TimelineEvent[]) {
  if (items.length === 0) {
    return "当前事故暂无时间线事件，后端尚未写入 Timeline Narrative。";
  }

  const hasAlert = items.some((item) => item.event_type.includes("alert"));
  // const hasRuntime = items.some(
  //   (item) =>
  //     item.timeline_type === "RUNTIME" ||
  //     item.event_type.includes("runtime") ||
  //     item.event_type.includes("kubernetes")
  // );

  const hasRuntime = items.some(
    (item) =>
      item.timeline_type === "RUNTIME" ||
      item.event_type.includes("runtime") ||
      item.event_type.includes("kubernetes") ||
      item.message.toLowerCase().includes("imagepullbackoff") ||
      item.message.toLowerCase().includes("errimagepull")
  );

  const hasImageIssue = items.some(
    (item) =>
      item.event_type.toLowerCase().includes("image") ||
      item.message.toLowerCase().includes("imagepullbackoff") ||
      item.message.toLowerCase().includes("errimagepull")
  );

  const parts: string[] = [];

  if (hasAlert) {
    parts.push("Prometheus Alert 首先进入 firing 状态");
  }

  if (hasRuntime) {
    parts.push("随后系统关联到 Kubernetes Runtime Event");
  }

  if (hasImageIssue) {
    parts.push("事故表现为镜像拉取失败或 ImagePullBackOff");
  }

  if (parts.length === 0) {
    return "该事故由后端 Correlation Engine 自动归并，并形成运行时事故时间线。";
  }

  return `${parts.join("，")}。`;
}