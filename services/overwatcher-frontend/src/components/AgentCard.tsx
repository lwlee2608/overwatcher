import { useState } from "react";
import { Link } from "react-router-dom";
import type { AgentStatus } from "../types/agent";
import { AgentTypeBadge } from "./AgentTypeBadge";
import { CloudProviderBadge } from "./CloudProviderBadge";
import { CopyButton } from "./CopyButton";
import { StatusBadge } from "./StatusBadge";

function percentOf(used: number, total: number): number {
  return total > 0 ? (used / total) * 100 : 0;
}

function formatBytes(bytes: number): string {
  const gb = bytes / 1024 ** 3;
  if (gb >= 1) return `${gb.toFixed(1)} GB`;
  return `${(bytes / 1024 ** 2).toFixed(0)} MB`;
}

function MetricBar({ label, percent, detail }: { label: string; percent: number; detail: string }) {
  const clamped = Math.min(Math.max(percent, 0), 100);
  const barColor = clamped >= 90 ? "bg-red-500" : clamped >= 75 ? "bg-yellow-500" : "bg-blue-500";
  return (
    <div className="flex items-center gap-2">
      <span className="w-8 font-medium text-gray-500 dark:text-gray-500">
        {label}
      </span>
      <div className="h-1.5 w-20 shrink-0 rounded-full bg-gray-200 dark:bg-gray-700">
        <div
          className={`h-1.5 rounded-full ${barColor}`}
          style={{ width: `${clamped}%` }}
        />
      </div>
      <span className="text-xs tabular-nums">{detail}</span>
    </div>
  );
}

// The systemd installer re-reads the token from the existing env file, so the
// upgrade command needs no secrets. Docker agents were started with run args
// we don't know, so only the pull/remove steps can be given.
function upgradeCommand(agent: AgentStatus, releaseTag: string): string {
  if (agent.type === "docker") {
    return `docker pull lwlee2608/agent:${releaseTag.replace(/^v/, "")}
docker stop overwatcher-agent && docker rm overwatcher-agent
# re-run your original docker run command with the new image tag`;
  }
  return `TOKEN=$(sudo sed -n 's/^AGENT_TOKEN=//p' /etc/overwatcher-agent.env)
curl -fsSL ${window.location.origin}/install.sh | sudo AGENT_TOKEN="$TOKEN" bash`;
}

function UpgradeInstructions({ agent, releaseTag }: { agent: AgentStatus; releaseTag: string }) {
  const command = upgradeCommand(agent, releaseTag);

  return (
    <div>
      <p className="mb-1 text-xs text-gray-500 dark:text-gray-400">
        {agent.type === "docker"
          ? "Pull the new image and recreate the container on the host:"
          : "Run this on the agent's host — the binary is swapped in place and the unit restarted:"}
      </p>
      <div className="relative">
        <pre className="overflow-x-auto rounded bg-gray-900 p-3 pr-12 text-xs text-gray-100 dark:bg-gray-950">
          <code>{command}</code>
        </pre>
        <div className="absolute right-2 top-2">
          <CopyButton text={command} label="Copy upgrade command" />
        </div>
      </div>
    </div>
  );
}

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
  releaseTag?: string | null;
}

export function AgentCard({ agent, onDelete, releaseTag }: AgentCardProps) {
  const [showUpgrade, setShowUpgrade] = useState(false);
  const bound = Boolean(agent.project_id);
  const canDelete = Boolean(onDelete) && !bound;
  // Dev coordinator builds report release_tag "latest", which can't be
  // compared against agent versions — only flag against a real vX.Y.Z tag.
  const outdated = Boolean(
    releaseTag?.startsWith("v") && agent.version && agent.version !== releaseTag
  );
  return (
    <div className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
            {agent.name}
          </h3>
          {agent.type && <AgentTypeBadge type={agent.type} />}
        </div>
        <StatusBadge status={agent.status} />
      </div>

      <div className="space-y-2 text-sm text-gray-600 dark:text-gray-400">
        <div className="flex items-center gap-2">
          <span className="font-medium text-gray-500 dark:text-gray-500">
            IP
          </span>
          <span className="font-mono">{agent.remote_ip}</span>
          {agent.cloud_provider && (
            <CloudProviderBadge provider={agent.cloud_provider} />
          )}
        </div>

        <div className="flex items-center gap-2">
          <span className="font-medium text-gray-500 dark:text-gray-500">
            Last seen
          </span>
          <span>{agent.last_seen ? timeAgo(agent.last_seen) : "Never"}</span>
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
          {outdated && (
            <button
              type="button"
              onClick={() => setShowUpgrade((v) => !v)}
              className="rounded bg-amber-100 px-1.5 py-0.5 text-xs font-medium text-amber-800 hover:bg-amber-200 dark:bg-amber-900/30 dark:text-amber-300 dark:hover:bg-amber-900/50"
            >
              {releaseTag} available
            </button>
          )}
        </div>

        {outdated && showUpgrade && (
          <UpgradeInstructions agent={agent} releaseTag={releaseTag!} />
        )}

        {agent.metrics && (
          <div
            className={`space-y-2 rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-gray-700 dark:bg-gray-900/50 ${agent.status !== "connected" ? "opacity-50" : ""}`}
            title={
              agent.status !== "connected" && agent.last_seen
                ? `As of last heartbeat, ${timeAgo(agent.last_seen)}`
                : undefined
            }
          >
            <div className="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">
              System
            </div>
            <MetricBar
              label="CPU"
              percent={agent.metrics.cpu_percent}
              detail={`${agent.metrics.cpu_percent.toFixed(0)}%`}
            />
            <MetricBar
              label="RAM"
              percent={percentOf(agent.metrics.mem_used_bytes, agent.metrics.mem_total_bytes)}
              detail={`${formatBytes(agent.metrics.mem_used_bytes)} / ${formatBytes(agent.metrics.mem_total_bytes)}`}
            />
            <MetricBar
              label="Disk"
              percent={percentOf(agent.metrics.disk_used_bytes, agent.metrics.disk_total_bytes)}
              detail={`${formatBytes(agent.metrics.disk_used_bytes)} / ${formatBytes(agent.metrics.disk_total_bytes)}`}
            />
            {agent.metrics.swap_total_bytes > 0 && (
              <MetricBar
                label="Swap"
                percent={percentOf(agent.metrics.swap_used_bytes, agent.metrics.swap_total_bytes)}
                detail={`${formatBytes(agent.metrics.swap_used_bytes)} / ${formatBytes(agent.metrics.swap_total_bytes)}`}
              />
            )}
          </div>
        )}

        <div className="flex items-center gap-2">
          <span className="font-medium text-gray-500 dark:text-gray-500">
            Project
          </span>
          {bound ? (
            <Link
              to={`/projects/${agent.project_id}`}
              className="text-blue-600 hover:underline dark:text-blue-400"
            >
              {agent.project_name || agent.project_id}
            </Link>
          ) : (
            <span className="italic text-gray-400 dark:text-gray-600">
              Unbound
            </span>
          )}
          {canDelete && onDelete && (
            <button
              type="button"
              onClick={() => onDelete(agent)}
              aria-label={`Delete agent ${agent.name}`}
              title="Delete agent"
              className="ml-auto rounded p-1 text-gray-400 hover:bg-red-50 hover:text-red-600 dark:text-gray-500 dark:hover:bg-red-900/30 dark:hover:text-red-400"
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
    </div>
  );
}
