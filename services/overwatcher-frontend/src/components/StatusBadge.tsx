interface StatusBadgeProps {
  connected: boolean;
}

export function StatusBadge({ connected }: StatusBadgeProps) {
  return (
    <span className="inline-flex items-center gap-1.5 text-sm font-medium">
      <span
        className={`h-2.5 w-2.5 rounded-full ${
          connected ? "bg-green-500" : "bg-red-500"
        }`}
      />
      {connected ? "Connected" : "Disconnected"}
    </span>
  );
}
