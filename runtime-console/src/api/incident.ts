import {
  client,
  demoMode,
} from "./client";

import { mockIncidents } from "./mock";

import type {
  Incident,
} from "../types/incident";

interface IncidentAPIResponse {
  id: number;
  title?: string;
  status: Incident["status"];
  severity: Incident["severity"];
  namespace?: string;
  policy?: string;
  category?: string;
  fingerprint?: string;
  rootResource?: {
    kind?: string;
    name?: string;
    namespace?: string;
  };
  startedAt?: string;
  updatedAt?: string;
}

interface IncidentAPIListResponse {
  items: IncidentAPIResponse[];
}

function normalizeIncident(item: IncidentAPIResponse): Incident {
  return {
    id: item.id,
    title: item.title,
    status: item.status,
    severity: item.severity,
    category: item.category,
    fingerprint: item.fingerprint,
    resource_kind: item.rootResource?.kind || "Unknown",
    resource_name: item.rootResource?.name || item.title || `Incident #${item.id}`,
    namespace: item.namespace || item.rootResource?.namespace,
    policy_name: item.policy,
    started_at: item.startedAt,
    updated_at: item.updatedAt,
  };
}

export async function fetchIncidents(): Promise<Incident[]> {
  if (demoMode) {
    return mockIncidents;
  }

  const { data } =
    await client.get<IncidentAPIListResponse>(
      "/incidents"
    );

  return data.items.map(normalizeIncident);
}

export async function fetchOpenIncidents(): Promise<
  Incident[]
> {
  if (demoMode) {
    return mockIncidents.filter(
      (item) =>
        item.status === "open"
    );
  }

  const { data } =
    await client.get<IncidentAPIListResponse>(
      "/incidents/open"
    );

  return data.items.map(normalizeIncident);
}

export async function fetchIncidentDetail(
  id: number | string
): Promise<Incident> {
  if (demoMode) {
    const incident =
      mockIncidents.find(
        (item) =>
          String(item.id) ===
          String(id)
      );

    if (!incident) {
      throw new Error(
        "Incident not found"
      );
    }

    return incident;
  }

  const { data } =
    await client.get<IncidentAPIResponse>(
      `/incidents/${id}`
    );

  return normalizeIncident(data);
}
