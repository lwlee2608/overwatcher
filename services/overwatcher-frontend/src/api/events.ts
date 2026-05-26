import type { EventLogListResponse } from "../types/event_log";
import { apiJSON } from "./client";

export interface EventFilter {
  event_type?: string;
  repo?: string;
  sender?: string;
}

export async function fetchEvents(
  page: number = 1,
  pageSize: number = 25,
  filter: EventFilter = {},
): Promise<EventLogListResponse> {
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  });
  for (const [k, v] of Object.entries(filter)) {
    if (v) params.set(k, v);
  }
  return apiJSON<EventLogListResponse>(`/api/v1/events?${params}`);
}
