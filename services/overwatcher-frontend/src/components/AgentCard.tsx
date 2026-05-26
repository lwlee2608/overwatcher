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
  onDelete?: (agent: AgentStatus) => void;
}

export function AgentCard({ agent, onDelete }: AgentCardProps) {
  const canDelete = onDelete && !agent.project_id;
  return (
    <div className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
            {agent.name}
          </h3>
          {agent.type && <AgentTypeBadge type={agent.type} />}
        </div>
        <div className="flex items-center gap-2">
          <StatusBadge status={agent.status} />
          {canDelete && (
            <button
              type="button"
              onClick={() => onDelete(agent)}
              aria-label={`Delete agent ${agent.name}`}
              title="Delete agent"
              className="rounded p-1 text-gray-400 hover:bg-red-50 hover:text-red-600 dark:text-gray-500 dark:hover:bg-red-900/30 dark:hover:text-red-400"
            >
              <svg
                className="h-4 w-4"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
                aria-hidden="true"
              >
                <path d="M3 6h18" />
                <path d="M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
                <path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" />
                <path d="M10 11v6" />
                <path d="M14 11v6" />
              </svg>
            </button>
          )}
        </div>
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
