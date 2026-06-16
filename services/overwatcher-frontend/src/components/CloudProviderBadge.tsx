import type { CloudProvider } from "../types/agent";

interface CloudProviderBadgeProps {
  provider: CloudProvider;
}

const META: Record<CloudProvider, { label: string; color: string }> = {
  aws: { label: "AWS", color: "#FF9900" },
  gcp: { label: "GCP", color: "#4285F4" },
  alibaba: { label: "Alibaba", color: "#FF6A00" },
  azure: { label: "Azure", color: "#0078D4" },
};

export function CloudProviderBadge({ provider }: CloudProviderBadgeProps) {
  const meta = META[provider];
  if (!meta) return null;
  return (
    <span className="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700 dark:bg-gray-700 dark:text-gray-300">
      <span
        className="h-2 w-2 shrink-0 rounded-full"
        style={{ backgroundColor: meta.color }}
        aria-hidden="true"
      />
      {meta.label}
    </span>
  );
}
