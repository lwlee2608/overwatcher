import { useEffect, useState } from "react";
import type { AgentStatus, ConnectionStatus } from "../types/agent";
import { deleteAgent, fetchAgents } from "../api/agents";
import { AgentCard } from "./AgentCard";
import { InstallAgentCard } from "./InstallAgentCard";

type StatusFilter = "all" | ConnectionStatus;

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
  const [filter, setFilter] = useState<StatusFilter>("all");

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
    filter === "all" ? agents : agents.filter((a) => a.status === filter);
  const sorted = [...filtered].sort((a, b) => {
    const r = statusRank[a.status] - statusRank[b.status];
    if (r !== 0) return r;
    return a.name.localeCompare(b.name);
  });

  const counts: Record<StatusFilter, number> = {
    all: agents.length,
    connected: 0,
    stale: 0,
    disconnected: 0,
    lost: 0,
  };
  for (const a of agents) counts[a.status]++;

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
        <div className="inline-flex items-center gap-1 rounded-md border border-gray-200 bg-white pl-2 transition-colors hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:hover:bg-gray-700">
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
          <select
            aria-label="Filter agents by status"
            value={filter}
            onChange={(e) => setFilter(e.target.value as StatusFilter)}
            className="cursor-pointer border-0 bg-transparent py-0.5 pr-1 text-xs text-gray-900 focus:outline-none focus:ring-0 dark:text-gray-100"
          >
            <option
              value="all"
              className="bg-white text-gray-900 dark:bg-gray-800 dark:text-gray-100"
            >
              All ({agents.length})
            </option>
            {statusOptions.map((s) => (
              <option
                key={s.value}
                value={s.value}
                className="bg-white text-gray-900 dark:bg-gray-800 dark:text-gray-100"
              >
                {s.label} ({counts[s.value]})
              </option>
            ))}
          </select>
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
        <InstallAgentCard onClose={() => setShowInstall(false)} />
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
          <AgentCard key={agent.name} agent={agent} onDelete={handleDelete} />
        ))}
      </div>
    </div>
  );
}
