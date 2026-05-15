import {
  client,
  demoMode,
} from "./client";

import { mockIncidents } from "./mock";

import type {
  Incident,
  IncidentListResponse,
} from "../types/incident";

export async function fetchIncidents(): Promise<Incident[]> {
  if (demoMode) {
    return mockIncidents;
  }

  const { data } =
    await client.get<IncidentListResponse>(
      "/incidents"
    );

  return data.items;
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
    await client.get<IncidentListResponse>(
      "/incidents/open"
    );

  return data.items;
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
    await client.get<Incident>(
      `/incidents/${id}`
    );

  return data;
}