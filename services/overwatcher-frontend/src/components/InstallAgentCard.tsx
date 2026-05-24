import { useMemo, useState } from "react";

// Global secret today — the UI doesn't know it; user pastes their own.
const SECRET_PLACEHOLDER = "<your-AGENT_SHARED_SECRET>";

type Mode = "systemd" | "docker";

interface InstallAgentCardProps {
  onClose: () => void;
}

export function InstallAgentCard({ onClose }: InstallAgentCardProps) {
  const [name, setName] = useState("");
  const [mode, setMode] = useState<Mode>("systemd");
  const [copied, setCopied] = useState(false);

  const coordinatorURL = useMemo(() => window.location.origin, []);
  const installURL = `${coordinatorURL}/install.sh`;

  const safeName = name.trim() || "<your-agent-name>";

  const systemdCommand = `curl -fsSL ${installURL} | \\
sudo AGENT_NAME="${safeName}" \\
AGENT_SHARED_SECRET="${SECRET_PLACEHOLDER}" \\
bash`;

  const dockerCommand = `docker run -d --name overwatcher-agent --restart unless-stopped \\
  -e AGENT_NAME="${safeName}" \\
  -e AGENT_SHARED_SECRET="${SECRET_PLACEHOLDER}" \\
  -e AGENT_COORDINATOR_URL="${coordinatorURL}" \\
  -v /var/run/docker.sock:/var/run/docker.sock \\
  -v /path/to/your/deployment:/opt/stacks/my-stack \\
  -v ~/.docker/config.json:/root/.docker/config.json:ro \\
  lwlee2608/agent:latest`;

  const command = mode === "systemd" ? systemdCommand : dockerCommand;

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // Clipboard fails in non-secure contexts; command is on-screen anyway.
    }
  };

  const tabBase =
    "px-3 py-1.5 text-sm font-medium rounded-md transition-colors";
  const tabActive =
    "bg-white text-gray-900 shadow-sm dark:bg-gray-700 dark:text-gray-100";
  const tabIdle =
    "text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-200";

  return (
    <div className="mb-6 rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
          Install a new agent
        </h3>
        <div className="flex items-center gap-4">
          <a
            className="text-xs text-blue-600 hover:underline dark:text-blue-400"
            href="https://github.com/lwlee2608/overwatcher/blob/main/docs/agent-systemd.md"
            target="_blank"
            rel="noreferrer"
          >
            Docs
          </a>
          <button
            type="button"
            onClick={onClose}
            className="text-xs text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
            aria-label="Close"
          >
            ✕
          </button>
        </div>
      </div>

      <div className="mb-4 inline-flex rounded-lg bg-gray-100 p-1 dark:bg-gray-900">
        <button
          type="button"
          onClick={() => setMode("systemd")}
          className={`${tabBase} ${mode === "systemd" ? tabActive : tabIdle}`}
        >
          systemd
        </button>
        <button
          type="button"
          onClick={() => setMode("docker")}
          className={`${tabBase} ${mode === "docker" ? tabActive : tabIdle}`}
        >
          Docker
        </button>
      </div>

      {mode === "systemd" ? (
        <p className="mb-3 text-sm text-gray-600 dark:text-gray-400">
          Run this on the VM you want to deploy to. The installer adds a system
          user, drops the binary at <code>/usr/local/bin/overwatcher-agent</code>,
          and enables a systemd unit.
        </p>
      ) : (
        <p className="mb-3 text-sm text-gray-600 dark:text-gray-400">
          Run the agent as a container. Mount the host Docker socket so the
          agent can run <code>docker compose</code> on the host, plus your
          deployment directory and registry credentials.
        </p>
      )}

      <label className="mb-3 block text-sm">
        <span className="mb-1 block font-medium text-gray-700 dark:text-gray-300">
          Agent name
        </span>
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="my-agent"
          className="w-full rounded border border-gray-300 bg-white px-3 py-2 text-sm font-mono dark:border-gray-600 dark:bg-gray-900 dark:text-gray-100"
        />
      </label>

      <div className="relative">
        <pre className="overflow-x-auto rounded bg-gray-900 p-3 pr-20 text-xs text-gray-100 dark:bg-gray-950">
          <code>{command}</code>
        </pre>
        <button
          type="button"
          onClick={handleCopy}
          className="absolute right-2 top-2 rounded bg-gray-700 px-2 py-1 text-xs text-gray-100 hover:bg-gray-600"
        >
          {copied ? "Copied" : "Copy"}
        </button>
      </div>

      {mode === "systemd" ? (
        <details className="mt-3 text-sm text-gray-600 dark:text-gray-400">
          <summary className="cursor-pointer select-none font-medium text-gray-700 dark:text-gray-300">
            After install
          </summary>
          <pre className="mt-2 overflow-x-auto rounded bg-gray-900 p-3 text-xs text-gray-100 dark:bg-gray-950">
            <code>{`# follow logs
journalctl -u overwatcher-agent -f

# status
systemctl status overwatcher-agent

# restart after editing /etc/overwatcher-agent.env
sudo systemctl restart overwatcher-agent`}</code>
          </pre>
        </details>
      ) : (
        <details className="mt-3 text-sm text-gray-600 dark:text-gray-400">
          <summary className="cursor-pointer select-none font-medium text-gray-700 dark:text-gray-300">
            After install
          </summary>
          <pre className="mt-2 overflow-x-auto rounded bg-gray-900 p-3 text-xs text-gray-100 dark:bg-gray-950">
            <code>{`# follow logs
docker logs -f overwatcher-agent

# restart
docker restart overwatcher-agent`}</code>
          </pre>
        </details>
      )}
    </div>
  );
}
