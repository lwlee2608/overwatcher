export interface EventLog {
  id: string;
  delivery_id: string;
  event_type: string;
  repo: string;
  sender: string;
  summary: string;
  created_at: string;
}

export interface EventLogListResponse {
  events: EventLog[];
}
