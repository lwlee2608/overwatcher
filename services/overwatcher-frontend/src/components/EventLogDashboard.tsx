import { useCallback, useEffect, useRef, useState } from "react";
import type { EventLog } from "../types/event_log";
import { fetchEvents, type EventFilter } from "../api/events";
import { timeAgo } from "../utils/time";
import { Pagination } from "./Pagination";

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

const inputClass =
  "rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 px-2 py-1 text-sm text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-1 focus:ring-blue-500";

export function EventLogDashboard() {
  const [events, setEvents] = useState<EventLog[]>([]);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(25);
  const [filter, setFilter] = useState<EventFilter>({});
  const [repoInput, setRepoInput] = useState("");
  const [senderInput, setSenderInput] = useState("");

  const refresh = useCallback(async () => {
    try {
      const data = await fetchEvents(page, pageSize, filter);
      setEvents(data.events ?? []);
      setTotal(data.total ?? 0);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to fetch");
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, filter]);

  useEffect(() => {
    refresh();
    const id = setInterval(refresh, 10_000);
    return () => clearInterval(id);
  }, [refresh]);

  // Debounce repo + sender text inputs so typing doesn't fire a request per
  // keystroke.
  const debounceRef = useRef<number | undefined>(undefined);
  useEffect(() => {
    window.clearTimeout(debounceRef.current);
    debounceRef.current = window.setTimeout(() => {
      setFilter((f) => ({
        ...f,
        repo: repoInput || undefined,
        sender: senderInput || undefined,
      }));
      setPage(1);
    }, 300);
    return () => window.clearTimeout(debounceRef.current);
  }, [repoInput, senderInput]);

  // Surface event types we've actually seen on this page so the dropdown
  // stays helpful even if a deployment introduces a new type.
  const eventTypeOptions = Array.from(
    new Set([...Object.keys(eventTypeColors), ...events.map((e) => e.event_type)]),
  )
    .filter(Boolean)
    .sort();

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

      <div className="mb-4 flex flex-wrap items-center gap-2">
        <select
          value={filter.event_type ?? ""}
          onChange={(e) => {
            setFilter((f) => ({
              ...f,
              event_type: e.target.value || undefined,
            }));
            setPage(1);
          }}
          className={inputClass}
        >
          <option value="">All event types</option>
          {eventTypeOptions.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </select>
        <input
          type="text"
          value={repoInput}
          onChange={(e) => setRepoInput(e.target.value)}
          placeholder="repo contains…"
          className={inputClass}
        />
        <input
          type="text"
          value={senderInput}
          onChange={(e) => setSenderInput(e.target.value)}
          placeholder="sender"
          className={inputClass}
        />
        {(filter.event_type || filter.repo || filter.sender) && (
          <button
            type="button"
            onClick={() => {
              setFilter({});
              setRepoInput("");
              setSenderInput("");
              setPage(1);
            }}
            className="text-xs text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300"
          >
            Clear filters
          </button>
        )}
      </div>

      {events.length === 0 && !error ? (
        <div className="text-center py-12 text-gray-400 dark:text-gray-500">
          {total === 0 ? "No events received yet" : "No events match"}
        </div>
      ) : (
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

      <Pagination
        page={page}
        pageSize={pageSize}
        total={total}
        onPageChange={setPage}
        onPageSizeChange={(s) => {
          setPageSize(s);
          setPage(1);
        }}
      />
    </div>
  );
}
