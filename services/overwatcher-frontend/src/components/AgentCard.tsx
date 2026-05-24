import type { AgentStatus } from "../types/agent";
import { AgentTypeBadge } from "./AgentTypeBadge";
import { StatusBadge } from "./StatusBadge";

function timeAgo(isoString: string): string {
  const seconds = Math.floor(
    (Date.now() - new Date(isoString).getTime()) / 1000
  );
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

interface AgentCardProps {
  agent: AgentStatus;
}

export function AgentCard({ agent }: AgentCardProps) {
  return (
    <div className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
            {agent.name}
          </h3>
          {agent.type && <AgentTypeBadge type={agent.type} />}
        </div>
        <StatusBadge connected={agent.connected} />
      </div>

      <div className="space-y-2 text-sm text-gray-600 dark:text-gray-400">
        <div className="flex items-center gap-2">
          <span className="font-medium text-gray-500 dark:text-gray-500">
            IP
          </span>
          <span className="font-mono">{agent.remote_ip}</span>
        </div>

        <div className="flex items-center gap-2">
          <span className="font-medium text-gray-500 dark:text-gray-500">
            Last seen
          </span>
          <span>{timeAgo(agent.last_seen)}</span>
        </div>

        <div className="flex items-center gap-2">
          <span className="font-medium text-gray-500 dark:text-gray-500">
            Version
          </span>
          {agent.version ? (
            <span className="font-mono">{agent.version}</span>
          ) : (
            <span className="italic text-gray-400 dark:text-gray-600">
              unknown
            </span>
          )}
        </div>
      </div>
    </div>
  );
}
