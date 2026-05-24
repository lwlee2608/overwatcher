import { useEffect, useState } from "react";
import type { AgentStatus } from "../types/agent";
import { fetchAgents } from "../api/agents";
import { AgentCard } from "./AgentCard";
import { InstallAgentCard } from "./InstallAgentCard";

export function AgentDashboard() {
  const [agents, setAgents] = useState<AgentStatus[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

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

  const connected = agents.filter((a) => a.connected).length;

  const sorted = [...agents].sort((a, b) => {
    if (a.connected !== b.connected) return a.connected ? -1 : 1;
    return a.name.localeCompare(b.name);
  });

  if (loading) {
    return (
      <div className="max-w-4xl mx-auto text-center py-12 text-gray-400 dark:text-gray-500">
        Loading...
      </div>
    );
  }

  return (
    <div className="max-w-4xl mx-auto">
      <InstallAgentCard />
      {error && (
        <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400">
          {error}
        </div>
      )}

      <div className="mb-6 flex items-center gap-4 text-sm text-gray-600 dark:text-gray-400">
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

      {sorted.length === 0 && !error && (
        <div className="text-center py-12 text-gray-400 dark:text-gray-500">
          No agents registered yet
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
