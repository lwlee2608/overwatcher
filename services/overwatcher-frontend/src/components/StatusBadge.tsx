import type { ConnectionStatus } from "../types/agent";

interface StatusBadgeProps {
  status: ConnectionStatus;
}

const dotClass: Record<ConnectionStatus, string> = {
  connected: "bg-green-500",
  stale: "bg-amber-500",
  disconnected: "bg-red-500",
};

const label: Record<ConnectionStatus, string> = {
  connected: "Connected",
  stale: "Stale",
  disconnected: "Disconnected",
};

export function StatusBadge({ status }: StatusBadgeProps) {
  return (
    <span className="inline-flex items-center gap-1.5 text-sm font-medium text-gray-900 dark:text-gray-100">
      <span className={`h-2.5 w-2.5 rounded-full ${dotClass[status]}`} />
      {label[status]}
    </span>
  );
}
