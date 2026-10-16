import { client } from "./client";
import type { TimelineEvent } from "../types/incident";

interface TimelineAPIEvent {
  id: number;
  type: string;
  actor?: string;
  message: string;
  occurredAt: string;
}

interface TimelineAPIResponse {
  items: TimelineAPIEvent[];
}

function normalizeTimelineEvent(item: TimelineAPIEvent): TimelineEvent {
  return {
    id: item.id,
    event_type: item.type,
    timeline_type: item.actor === "system" ? "SYSTEM" : "RUNTIME",
    severity: "info",
    message: item.message,
    occurred_at: item.occurredAt,
  };
}

export async function fetchIncidentTimeline(
  id: number | string
): Promise<TimelineEvent[]> {
  const { data } = await client.get<TimelineAPIResponse>(
    `/incidents/${id}/timeline`
  );

  return data.items.map(normalizeTimelineEvent);
}
