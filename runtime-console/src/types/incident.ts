export type IncidentStatus = "open" | "resolved";

export type IncidentSeverity = "critical" | "warning" | "info";

export type TimelineType = "SYSTEM" | "RUNTIME";

export interface Incident {
  id: number;
  title?: string;
  status: IncidentStatus;
  severity: IncidentSeverity;
  category?: string;
  fingerprint?: string;
  resource_kind: string;
  resource_name: string;
  namespace?: string;
  policy_name?: string;
  started_at?: string;
  updated_at?: string;
}

export interface IncidentListResponse {
  items: Incident[];
}

export interface TimelineEvent {
  id: number;
  incident_id?: number;
  event_type: string;
  timeline_type: TimelineType;
  severity: IncidentSeverity | "error";
  message: string;
  resource_kind?: string;
  resource_name?: string;
  occurred_at: string;
}

export interface TimelineResponse {
  items: TimelineEvent[];
}
