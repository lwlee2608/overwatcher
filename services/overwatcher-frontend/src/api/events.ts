import type { EventLogListResponse } from "../types/event_log";

export async function fetchEvents(
  limit: number = 50
): Promise<EventLogListResponse> {
  const res = await fetch(`/api/v1/events?limit=${limit}`);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}
