import { useEffect, useState } from "react";
import type { AgentStatus, ConnectionStatus } from "../types/agent";
import { fetchAgents } from "../api/agents";
import { AgentCard } from "./AgentCard";
import { InstallAgentCard } from "./InstallAgentCard";

type StatusFilter = "all" | ConnectionStatus;

const filterOptions: { value: StatusFilter; label: string; dot?: string }[] = [
  { value: "all", label: "All" },
  { value: "connected", label: "Connected", dot: "bg-green-500" },
  { value: "stale", label: "Stale", dot: "bg-amber-500" },
  { value: "disconnected", label: "Disconnected", dot: "bg-red-500" },
  { value: "lost", label: "Lost", dot: "bg-gray-400" },
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

  const connected = agents.filter((a) => a.status === "connected").length;

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

      <div className="mb-6 flex items-center justify-between gap-4">
        <div className="flex items-center gap-4 text-sm text-gray-600 dark:text-gray-400">
          <span>
            <span className="font-semibold text-gray-900 dark:text-gray-100">
              {connected}
            </span>{" "}
            connected
          </span>
          <span>
            <span className="font-semibold text-gray-900 dark:text-gray-100">
              {agents.length}
            </span>{" "}
            total
          </span>
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

      {agents.length > 0 && (
        <div className="mb-4 flex flex-wrap gap-2">
          {filterOptions.map((opt) => {
            const active = filter === opt.value;
            return (
              <button
                key={opt.value}
                type="button"
                onClick={() => setFilter(opt.value)}
                className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium transition-colors ${
                  active
                    ? "border-blue-500 bg-blue-50 text-blue-700 dark:border-blue-400 dark:bg-blue-900/30 dark:text-blue-300"
                    : "border-gray-200 bg-white text-gray-600 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400 dark:hover:bg-gray-700"
                }`}
              >
                {opt.dot && (
                  <span className={`h-2 w-2 rounded-full ${opt.dot}`} />
                )}
                {opt.label}
                <span className="text-gray-400 dark:text-gray-500">
                  {counts[opt.value]}
                </span>
              </button>
            );
          })}
        </div>
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
          <AgentCard key={agent.name} agent={agent} />
        ))}
      </div>
    </div>
  );
}
