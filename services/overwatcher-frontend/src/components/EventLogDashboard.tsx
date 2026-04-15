import { useEffect, useState } from "react";
import type { EventLog } from "../types/event_log";
import { fetchEvents } from "../api/events";

function timeAgo(dateStr: string): string {
  const seconds = Math.floor(
    (Date.now() - new Date(dateStr).getTime()) / 1000
  );
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

const eventTypeColors: Record<string, string> = {
  push: "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200",
  ping: "bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400",
  pull_request:
    "bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200",
  issues:
    "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200",
};

const defaultColor =
  "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200";

export function EventLogDashboard() {
  const [events, setEvents] = useState<EventLog[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let active = true;

    async function poll() {
      try {
        const data = await fetchEvents(100);
        if (active) {
          setEvents(data.events ?? []);
          setError(null);
          setLoading(false);
        }
      } catch (err) {
        if (active) {
          setError(err instanceof Error ? err.message : "Failed to fetch");
          setLoading(false);
        }
      }
    }

    poll();
    const id = setInterval(poll, 10_000);
    return () => {
      active = false;
      clearInterval(id);
    };
  }, []);

  if (loading) {
    return (
      <div className="max-w-4xl mx-auto text-center py-12 text-gray-400 dark:text-gray-500">
        Loading...
      </div>
    );
  }

  return (
    <div className="max-w-4xl mx-auto">
      {error && (
        <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400">
          {error}
        </div>
      )}

      <div className="mb-6 text-sm text-gray-600 dark:text-gray-400">
        <span className="font-semibold text-gray-900 dark:text-gray-100">
          {events.length}
        </span>{" "}
        event{events.length !== 1 && "s"}
      </div>

      {events.length === 0 && !error && (
        <div className="text-center py-12 text-gray-400 dark:text-gray-500">
          No events received yet
        </div>
      )}

      {events.length > 0 && (
        <div className="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr className="text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                <th className="px-4 py-3">Event</th>
                <th className="px-4 py-3">Repository</th>
                <th className="px-4 py-3">Sender</th>
                <th className="px-4 py-3">Summary</th>
                <th className="px-4 py-3">Time</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 bg-white dark:divide-gray-700 dark:bg-gray-900">
              {events.map((e) => (
                <tr key={e.id}>
                  <td className="px-4 py-3">
                    <span
                      className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${eventTypeColors[e.event_type] ?? defaultColor}`}
                    >
                      {e.event_type}
                    </span>
                  </td>
                  <td className="px-4 py-3 font-mono text-gray-900 dark:text-gray-100">
                    {e.repo || "-"}
                  </td>
                  <td className="px-4 py-3 text-gray-700 dark:text-gray-300">
                    {e.sender || "-"}
                  </td>
                  <td className="px-4 py-3 text-gray-700 dark:text-gray-300">
                    {e.summary}
                  </td>
                  <td className="px-4 py-3 text-gray-500 dark:text-gray-400 whitespace-nowrap">
                    {timeAgo(e.created_at)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
