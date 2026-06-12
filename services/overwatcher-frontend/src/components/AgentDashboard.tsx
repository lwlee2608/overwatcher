import { useEffect, useRef, useState } from "react";
import type { AgentStatus, ConnectionStatus } from "../types/agent";
import { deleteAgent, fetchAgents } from "../api/agents";
import { fetchVersion } from "../api/version";
import { AgentCard } from "./AgentCard";
import { InstallAgentCard } from "./InstallAgentCard";

const statusOptions: { value: ConnectionStatus; label: string }[] = [
  { value: "connected", label: "Connected" },
  { value: "stale", label: "Stale" },
  { value: "disconnected", label: "Disconnected" },
  { value: "lost", label: "Lost" },
];

export function AgentDashboard() {
  const [agents, setAgents] = useState<AgentStatus[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [showInstall, setShowInstall] = useState(false);
  const [selected, setSelected] = useState<Set<ConnectionStatus>>(
    new Set(["connected", "stale", "disconnected"])
  );
  const [filterOpen, setFilterOpen] = useState(false);
  const [releaseTag, setReleaseTag] = useState<string | null>(null);
  const filterRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    fetchVersion()
      .then((v) => setReleaseTag(v.release_tag))
      .catch(() => setReleaseTag(null));
  }, []);

  useEffect(() => {
    if (!filterOpen) return;
    function onDocClick(e: MouseEvent) {
      if (!filterRef.current?.contains(e.target as Node)) setFilterOpen(false);
    }
    document.addEventListener("mousedown", onDocClick);
    return () => document.removeEventListener("mousedown", onDocClick);
  }, [filterOpen]);

  function toggleStatus(s: ConnectionStatus) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(s)) next.delete(s);
      else next.add(s);
      return next;
    });
  }

  useEffect(() => {
    let active = true;

    async function poll() {
      try {
        const data = await fetchAgents();
        if (active) {
          setAgents(data.agents ?? []);
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

  async function handleDelete(agent: AgentStatus) {
    if (!window.confirm(`Delete agent ${agent.name}?`)) return;
    try {
      await deleteAgent(agent.id);
      setAgents((prev) => prev.filter((a) => a.id !== agent.id));
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Delete failed");
    }
  }

  const statusRank: Record<AgentStatus["status"], number> = {
    connected: 0,
    stale: 1,
    disconnected: 2,
    lost: 3,
  };
  const filtered =
    selected.size === 0
      ? []
      : agents.filter((a) => selected.has(a.status));
  const sorted = [...filtered].sort((a, b) => {
    const r = statusRank[a.status] - statusRank[b.status];
    if (r !== 0) return r;
    return a.name.localeCompare(b.name);
  });

  const counts: Record<ConnectionStatus, number> = {
    connected: 0,
    stale: 0,
    disconnected: 0,
    lost: 0,
  };
  for (const a of agents) counts[a.status]++;

  const filterLabel =
    selected.size === 0
      ? "None"
      : selected.size === statusOptions.length
        ? "All"
        : selected.size === statusOptions.length - 1 && !selected.has("lost")
          ? "All except Lost"
          : statusOptions
              .filter((s) => selected.has(s.value))
              .map((s) => s.label)
              .join(", ");

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

      <div className="mb-6 flex flex-wrap items-center justify-between gap-4">
        <div ref={filterRef} className="relative">
          <button
            type="button"
            aria-label="Filter agents by status"
            aria-haspopup="true"
            aria-expanded={filterOpen}
            onClick={() => setFilterOpen((v) => !v)}
            className="inline-flex items-center gap-1.5 rounded-md border border-gray-200 bg-white pl-2 pr-2 py-1 text-xs text-gray-900 transition-colors hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100 dark:hover:bg-gray-700"
          >
            <svg
              className="h-3.5 w-3.5 text-gray-500 dark:text-gray-400"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              aria-hidden="true"
            >
              <path d="M22 3H2l8 9.46V19l4 2v-8.54L22 3z" />
            </svg>
            <span className="max-w-[14rem] truncate">
              {filterLabel} ({sorted.length})
            </span>
            <svg
              className="h-3 w-3 text-gray-500 dark:text-gray-400"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              aria-hidden="true"
            >
              <polyline points="6 9 12 15 18 9" />
            </svg>
          </button>
          {filterOpen && (
            <div
              role="menu"
              className="absolute left-0 z-10 mt-1 w-48 rounded-md border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-700 dark:bg-gray-800"
            >
              {statusOptions.map((s) => {
                const checked = selected.has(s.value);
                return (
                  <label
                    key={s.value}
                    className="flex cursor-pointer items-center justify-between gap-2 px-3 py-1.5 text-xs text-gray-900 hover:bg-gray-50 dark:text-gray-100 dark:hover:bg-gray-700"
                  >
                    <span className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        checked={checked}
                        onChange={() => toggleStatus(s.value)}
                        className="h-3.5 w-3.5 cursor-pointer accent-blue-600"
                      />
                      {s.label}
                    </span>
                    <span className="text-gray-500 dark:text-gray-400">
                      ({counts[s.value]})
                    </span>
                  </label>
                );
              })}
            </div>
          )}
        </div>
        {!showInstall && (
          <button
            type="button"
            onClick={() => setShowInstall(true)}
            className="rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-500 dark:bg-blue-500 dark:hover:bg-blue-400"
          >
            + New agent
          </button>
        )}
      </div>

      {showInstall && (
        <InstallAgentCard
          onClose={() => setShowInstall(false)}
          onCreated={() => {
            fetchAgents()
              .then((data) => setAgents(data.agents ?? []))
              .catch(() => {});
          }}
        />
      )}

      {agents.length === 0 && !error && (
        <div className="text-center py-12 text-gray-400 dark:text-gray-500">
          No agents registered yet
        </div>
      )}

      {agents.length > 0 && sorted.length === 0 && (
        <div className="text-center py-12 text-gray-400 dark:text-gray-500">
          No agents match this filter
        </div>
      )}

      <div className="grid gap-4 sm:grid-cols-2">
        {sorted.map((agent) => (
          <AgentCard
            key={agent.name}
            agent={agent}
            onDelete={handleDelete}
            releaseTag={releaseTag}
          />
        ))}
      </div>
    </div>
  );
}
