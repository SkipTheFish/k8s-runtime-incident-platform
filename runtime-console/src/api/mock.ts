import type { Incident, TimelineEvent } from "../types/incident";

export const mockIncidents: Incident[] = [
  {
    id: 33,
    status: "open",
    severity: "critical",
    namespace: "default",
    policy_name: "image-pull-backoff-policy",
    resource_kind: "Pod",
    resource_name: "bad-image",
    started_at: new Date(Date.now() - 1000 * 60 * 8).toISOString(),
    updated_at: new Date().toISOString(),
  },
  {
    id: 34,
    status: "open",
    severity: "warning",
    namespace: "payment",
    policy_name: "pod-restart-policy",
    resource_kind: "Pod",
    resource_name: "payment-api-7d9c",
    started_at: new Date(Date.now() - 1000 * 60 * 42).toISOString(),
    updated_at: new Date().toISOString(),
  },
  {
    id: 35,
    status: "resolved",
    severity: "info",
    namespace: "observability",
    policy_name: "node-pressure-policy",
    resource_kind: "Node",
    resource_name: "worker-01",
    started_at: new Date(Date.now() - 1000 * 60 * 60 * 5).toISOString(),
    updated_at: new Date().toISOString(),
  },
];

export function getMockIncidentTimeline(id: number | string): TimelineEvent[] {
  const incident = mockIncidents.find((item) => String(item.id) === String(id));

  const resourceKind = incident?.resource_kind || "Pod";
  const resourceName = incident?.resource_name || "bad-image";

  return [
    {
      id: 1,
      incident_id: Number(id),
      event_type: "alert.triggered",
      timeline_type: "SYSTEM",
      severity: "info",
      message: "Alert entered firing state",
      resource_kind: resourceKind,
      resource_name: resourceName,
      occurred_at: new Date(Date.now() - 1000 * 60 * 8).toISOString(),
    },
    {
      id: 2,
      incident_id: Number(id),
      event_type: "kubernetes.warning",
      timeline_type: "RUNTIME",
      severity: "warning",
      message: "Kubernetes Warning Event detected: ErrImagePull",
      resource_kind: resourceKind,
      resource_name: resourceName,
      occurred_at: new Date(Date.now() - 1000 * 60 * 7).toISOString(),
    },
    {
      id: 3,
      incident_id: Number(id),
      event_type: "runtime.image_pull_backoff",
      timeline_type: "RUNTIME",
      severity: "error",
      message: "ImagePullBackOff detected on target pod",
      resource_kind: resourceKind,
      resource_name: resourceName,
      occurred_at: new Date(Date.now() - 1000 * 60 * 6).toISOString(),
    },
  ];
}