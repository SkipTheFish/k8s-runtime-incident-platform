import { client } from "./client";
import type { TimelineEvent, TimelineResponse } from "../types/incident";

export async function fetchIncidentTimeline(
  id: number | string
): Promise<TimelineEvent[]> {
  const { data } = await client.get<TimelineResponse>(
    `/incidents/${id}/timeline`
  );

  return data.items;
}