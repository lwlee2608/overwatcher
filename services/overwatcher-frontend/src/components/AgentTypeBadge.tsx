import type { AgentType } from "../types/agent";

interface AgentTypeBadgeProps {
  type: AgentType;
}

export function AgentTypeBadge({ type }: AgentTypeBadgeProps) {
  return (
    <span className="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700 dark:bg-gray-700 dark:text-gray-300">
      {type === "docker" ? <DockerIcon /> : <SystemdIcon />}
      {type}
    </span>
  );
}

function DockerIcon() {
  return (
    <svg
      viewBox="0 0 24 24"
      className="h-3.5 w-3.5"
      fill="currentColor"
      aria-hidden="true"
    >
      <path d="M4 11h2.2V8.8H4V11Zm2.6 0h2.2V8.8H6.6V11Zm2.6 0h2.2V8.8H9.2V11Zm2.6 0h2.2V8.8h-2.2V11Zm-5.2-2.4h2.2V6.4H6.6v2.2Zm2.6 0h2.2V6.4H9.2v2.2Zm2.6 0h2.2V6.4h-2.2v2.2Zm0-2.6h2.2V3.8h-2.2v2.2ZM22 11.5c-.4-.3-1.2-.4-1.9-.3-.1-.7-.5-1.3-1.1-1.8l-.3-.2-.2.3c-.4.6-.6 1.5-.5 2.3 0 .3.2.6.3.9-.4.2-1 .5-2 .5H2.2c-.2 1.1-.2 4.5 2.1 7.1 1.7 1.9 4.3 2.9 7.7 2.9 7.3 0 12.7-3.4 15.2-9.5.9 0 3-.1 4-2l.2-.4-.3-.2-.1-.6Z" />
    </svg>
  );
}

function SystemdIcon() {
  return (
    <svg
      viewBox="0 0 24 24"
      className="h-3.5 w-3.5"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33h.01a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82v.01a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1Z" />
    </svg>
  );
}
