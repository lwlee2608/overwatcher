import type { ReactElement } from "react";
import type { CloudProvider } from "../types/agent";

interface CloudProviderBadgeProps {
  provider: CloudProvider;
}

const LABELS: Record<CloudProvider, string> = {
  aws: "AWS",
  gcp: "Google Cloud",
  alibaba: "Alibaba Cloud",
  azure: "Azure",
};

const ICONS: Record<CloudProvider, ReactElement> = {
  aws: <AwsIcon />,
  gcp: <GcpIcon />,
  alibaba: <AlibabaIcon />,
  azure: <AzureIcon />,
};

export function CloudProviderBadge({ provider }: CloudProviderBadgeProps) {
  const icon = ICONS[provider];
  if (!icon) return null;
  return (
    <span
      className="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 dark:bg-gray-700"
      role="img"
      title={LABELS[provider]}
      aria-label={LABELS[provider]}
    >
      {icon}
    </span>
  );
}

function AwsIcon() {
  return (
    <svg viewBox="0 0 32 32" className="h-3.5 w-3.5 shrink-0" aria-hidden="true">
      <path
        fill="#f90"
        fillRule="evenodd"
        d="M15.63 31.388l-7.135-2.56V18.373l7.135 2.43zm1.3 0l7.135-2.56V18.373l-7.135 2.432zm-7.7-13.8l7.2-2.033 6.696 2.16-6.696 2.273zm-2.092-.8L0 14.22V3.75l7.135 2.43zm1.307 0l7.135-2.56V3.75L8.443 6.192zm-7.7-13.8l7.2-2.043 6.696 2.16-6.696 2.273zm23.052 13.8l-7.135-2.56V3.75l7.135 2.43zm1.3 0l7.135-2.56V3.75l-7.135 2.43zm-7.7-13.8l7.2-2.033 6.696 2.16-6.696 2.273z"
      />
    </svg>
  );
}

function GcpIcon() {
  return (
    <svg viewBox="0 0 64 64" className="h-3.5 w-3.5 shrink-0" aria-hidden="true">
      <path
        fill="#ea4335"
        d="M40.728 20.488l2.05.035 5.57-5.57.27-2.36C44.2 8.657 38.367 6.26 31.993 6.26c-11.54 0-21.28 7.852-24.163 18.488.608-.424 1.908-.106 1.908-.106l11.13-1.83s.572-.947.862-.9A13.88 13.88 0 0 1 32 17.375c3.3.007 6.34 1.173 8.728 3.102z"
      />
      <path
        fill="#4285f4"
        d="M56.17 24.77c-1.293-4.77-3.958-8.982-7.555-12.177l-7.887 7.887c3.16 2.55 5.187 6.452 5.187 10.82v1.392c3.837 0 6.954 3.124 6.954 6.954 0 3.837-3.124 6.954-6.954 6.954H32.007L30.615 48v8.346l1.392 1.385h13.908A18.11 18.11 0 0 0 64 39.647c-.007-6.155-3.1-11.6-7.83-14.876z"
      />
      <path
        fill="#34a853"
        d="M18.085 57.74h13.9V46.6h-13.9a6.89 6.89 0 0 1-2.862-.622l-2.007.615-5.57 5.57-.488 1.88a18 18 0 0 0 10.926 3.689z"
      />
      <path
        fill="#fbbc05"
        d="M18.085 21.57A18.11 18.11 0 0 0 0 39.654c0 5.873 2.813 11.095 7.166 14.403l8.064-8.064a6.96 6.96 0 0 1-4.099-6.339c0-3.837 3.124-6.954 6.954-6.954 2.82 0 5.244 1.7 6.34 4.1l8.064-8.064c-3.307-4.353-8.53-7.166-14.403-7.166z"
      />
    </svg>
  );
}

function AzureIcon() {
  return (
    <svg
      viewBox="0 0 32 32"
      className="h-3.5 w-3.5 shrink-0"
      fill="#0078D4"
      fillRule="evenodd"
      aria-hidden="true"
    >
      <path d="M19.867 7.282l-4.733 9.533 8.333 9.66L8 28.23l24 .25zm-.934-3.762L8.067 12.613 0 26.223l6.867-.7z" />
    </svg>
  );
}

function AlibabaIcon() {
  return (
    <svg
      viewBox="-22.98 -3.25 64 64"
      className="h-3.5 w-3.5 shrink-0"
      aria-hidden="true"
    >
      <path
        fill="#FF6A00"
        d="M-14.278 9.1c-2.917.6-6.05 2.93-7.352 5.484s-1.35 3.182-1.35 13.933c0 5.737.096 10.462.24 11.2.856 4.376 4.278 7.798 8.654 8.654 1.326.24 18.585.362 18.585.096 0-.193-1.47-5.978-1.543-6.05-.024-.048-3.086-.723-6.798-1.52-8.654-1.88-8.4-1.808-9.112-2.676l-.627-.747v-17.5l.627-.76c.723-.868.458-.795 9.112-2.676l6.798-1.52C3.03 14.968 4.5 9.182 4.5 9c0-.23-17.62-.12-18.778.12zm27.938.036c.024.434 1.374 5.737 1.52 5.882.048.072 3.35.82 7.304 1.663s7.473 1.7 7.786 1.88c.424.246.78.594 1.037 1.012.386.675.434 1.398.434 9.16s-.048 8.485-.434 9.16a3.17 3.17 0 0 1-1.037 1.012c-.313.17-3.8 1.012-7.786 1.88L15.18 42.46c-.145.145-1.495 5.448-1.52 5.882 0 .265 1.736.313 9.353.24 10.5-.096 10.63-.12 13.306-1.928 1.922-1.323 3.374-3.222 4.146-5.424.554-1.615.554-1.76.554-12.487s0-10.872-.554-12.487a11.01 11.01 0 0 0-4.146-5.424C33.643 9.026 33.523 9 23.013 8.905c-7.617-.072-9.353-.036-9.353.24zM2.9 28.238a1.74 1.74 0 0 0-.048.795c.072.4.506.434 6.17.506l6.075.048V27.9H9.08c-5.038 0-6.075.048-6.17.337z"
      />
    </svg>
  );
}
