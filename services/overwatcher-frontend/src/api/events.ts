import type { EventLogListResponse } from "../types/event_log";
import { apiJSON } from "./client";

export async function fetchEvents(
  limit: number = 50
): Promise<EventLogListResponse> {
  return apiJSON<EventLogListResponse>(`/api/v1/events?limit=${limit}`);
}
