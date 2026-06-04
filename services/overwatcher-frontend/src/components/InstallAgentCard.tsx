import { useEffect, useMemo, useState } from "react";
import { createAgent } from "../api/agents";
import { fetchVersion } from "../api/version";

type Mode = "systemd" | "docker";

interface InstallAgentCardProps {
  onClose: () => void;
  onCreated?: () => void;
}

export function InstallAgentCard({ onClose, onCreated }: InstallAgentCardProps) {
  const [name, setName] = useState("");
  const [mode, setMode] = useState<Mode>("systemd");
  const [copied, setCopied] = useState(false);
  const [revealed, setRevealed] = useState(false);
  const [token, setToken] = useState("");
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [releaseTag, setReleaseTag] = useState<string | null>(null);

  const coordinatorURL = useMemo(() => window.location.origin, []);
  const installURL = `${coordinatorURL}/install.sh`;

  useEffect(() => {
    fetchVersion()
      .then((v) => setReleaseTag(v.release_tag))
      .catch(() => setReleaseTag(null));
  }, []);

  // Pin the Docker image to the coordinator's agent release; fall back to
  // latest until the tag loads (or if the lookup fails).
  const imageTag = releaseTag ?? "latest";

  async function handleCreate() {
    const trimmed = name.trim();
    if (!trimmed || creating) return;
    setCreating(true);
    setError(null);
    try {
      const res = await createAgent(trimmed);
      setToken(res.agent_token);
      setRevealed(true);
      onCreated?.();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create agent");
    } finally {
      setCreating(false);
    }
  }

  // Wrap in single quotes for POSIX shells: '...' is literal, no metachar
  // expansion. Only ' itself needs the close-escape-reopen dance.
  const sh = (v: string) => `'${v.replace(/'/g, "'\\''")}'`;
  const masked = "owa_••••••••••••••••";

  const buildSystemd = (t: string) => `curl -fsSL ${installURL} | \\
sudo AGENT_TOKEN=${sh(t)} \\
bash`;

  const buildDocker = (t: string) => `docker run -d --name overwatcher-agent --restart unless-stopped \\
  -e AGENT_TOKEN=${sh(t)} \\
  -e AGENT_COORDINATOR_URL=${sh(coordinatorURL)} \\
  -v /var/run/docker.sock:/var/run/docker.sock \\
  -v /path/to/your/deployment:/opt/stacks/my-stack \\
  -v ~/.docker/config.json:/root/.docker/config.json:ro \\
  lwlee2608/agent:${imageTag}`;

  const build = mode === "systemd" ? buildSystemd : buildDocker;
  const command = build(token);
  const displayCommand = build(revealed ? token : masked);

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
        <div className="flex items-baseline gap-2">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
            Install a new agent
          </h3>
          {releaseTag && (
            <span className="font-mono text-xs text-gray-500 dark:text-gray-400">
              Agent release: {releaseTag}
            </span>
          )}
        </div>
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

      {!token ? (
        <>
          <p className="mb-3 text-sm text-gray-600 dark:text-gray-400">
            Name the agent, then generate a one-time install command. A unique
            token is minted for this agent — it's shown once and never stored.
          </p>
          <label className="mb-3 block text-sm">
            <span className="mb-1 block font-medium text-gray-700 dark:text-gray-300">
              Agent name
            </span>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") handleCreate();
              }}
              placeholder="my-agent"
              className="w-full rounded border border-gray-300 bg-white px-3 py-2 text-sm font-mono dark:border-gray-600 dark:bg-gray-900 dark:text-gray-100"
            />
          </label>
          {error && (
            <div className="mb-3 rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400">
              {error}
            </div>
          )}
          <button
            type="button"
            onClick={handleCreate}
            disabled={!name.trim() || creating}
            className="rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-500 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-blue-500 dark:hover:bg-blue-400"
          >
            {creating ? "Generating…" : "Generate install command"}
          </button>
        </>
      ) : (
        <>
          <div className="mb-3 inline-flex rounded-lg bg-gray-100 p-1 dark:bg-gray-900">
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

          <div className="mb-3 rounded border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:border-amber-800/60 dark:bg-amber-900/20 dark:text-amber-300">
            This token is shown once. Copy the command now — if you lose it,
            re-issue a new token from the agent's menu.
          </div>

          {mode === "systemd" ? (
            <p className="mb-3 text-sm text-gray-600 dark:text-gray-400">
              Run this on the VM you want to deploy to. The installer adds a
              system user, drops the binary at{" "}
              <code>/usr/local/bin/overwatcher-agent</code>, and enables a
              systemd unit.
            </p>
          ) : (
            <p className="mb-3 text-sm text-gray-600 dark:text-gray-400">
              Run the agent as a container. Mount the host Docker socket so the
              agent can run <code>docker compose</code> on the host, plus your
              deployment directory and registry credentials.
            </p>
          )}

          <div className="relative">
            <pre className="overflow-x-auto rounded bg-gray-900 p-3 pr-20 text-xs text-gray-100 dark:bg-gray-950">
              <code>{displayCommand}</code>
            </pre>
            <div className="absolute right-2 top-2 flex gap-1">
              <button
                type="button"
                onClick={() => setRevealed((v) => !v)}
                aria-label={revealed ? "Hide token" : "Show token"}
                title={revealed ? "Hide token" : "Show token"}
                className="inline-flex h-6 w-6 items-center justify-center rounded bg-gray-700 text-gray-100 hover:bg-gray-600"
              >
                {revealed ? (
                  <svg
                    aria-hidden="true"
                    viewBox="0 0 20 20"
                    fill="currentColor"
                    className="h-3.5 w-3.5"
                  >
                    <path d="M3.28 2.22a.75.75 0 0 0-1.06 1.06l14.5 14.5a.75.75 0 1 0 1.06-1.06l-1.745-1.745A10.029 10.029 0 0 0 19.336 10.59a1.65 1.65 0 0 0 0-1.18C17.857 6.066 14.208 3.5 10 3.5a9.92 9.92 0 0 0-4.512 1.072L3.28 2.22ZM7.752 6.69 9.34 8.28a3 3 0 0 1 3.38 3.38l1.59 1.59A4 4 0 0 0 7.752 6.69ZM10.748 13.93l2.523 2.523a9.987 9.987 0 0 1-3.27.547c-4.208 0-7.858-2.566-9.337-5.91a1.65 1.65 0 0 1 0-1.18 10.014 10.014 0 0 1 2.341-3.272l3.305 3.305a4 4 0 0 0 4.438 4.438Z" />
                  </svg>
                ) : (
                  <svg
                    aria-hidden="true"
                    viewBox="0 0 20 20"
                    fill="currentColor"
                    className="h-3.5 w-3.5"
                  >
                    <path
                      fillRule="evenodd"
                      d="M10 12.5a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5Z"
                      clipRule="evenodd"
                    />
                    <path
                      fillRule="evenodd"
                      d="M.664 10.59a1.65 1.65 0 0 1 0-1.18C2.143 6.066 5.793 3.5 10 3.5c4.208 0 7.857 2.566 9.336 5.91a1.65 1.65 0 0 1 0 1.18C17.857 13.934 14.208 16.5 10 16.5c-4.207 0-7.857-2.566-9.336-5.91ZM14 10a4 4 0 1 1-8 0 4 4 0 0 1 8 0Z"
                      clipRule="evenodd"
                    />
                  </svg>
                )}
              </button>
              <button
                type="button"
                onClick={handleCopy}
                aria-label={copied ? "Copied" : "Copy install command"}
                title={copied ? "Copied" : "Copy install command"}
                className="inline-flex h-6 w-6 items-center justify-center rounded bg-gray-700 text-gray-100 hover:bg-gray-600"
              >
                {copied ? (
                  <svg
                    aria-hidden="true"
                    viewBox="0 0 20 20"
                    fill="currentColor"
                    className="h-3.5 w-3.5 text-green-400"
                  >
                    <path
                      fillRule="evenodd"
                      d="M16.704 5.29a1 1 0 0 1 .006 1.414l-7.25 7.313a1 1 0 0 1-1.42.002L3.29 9.23a1 1 0 1 1 1.42-1.408l4.04 4.072 6.54-6.598a1 1 0 0 1 1.414-.006Z"
                      clipRule="evenodd"
                    />
                  </svg>
                ) : (
                  <svg
                    aria-hidden="true"
                    viewBox="0 0 20 20"
                    fill="currentColor"
                    className="h-3.5 w-3.5"
                  >
                    <path d="M7 3.5A1.5 1.5 0 0 1 8.5 2h6A1.5 1.5 0 0 1 16 3.5v8a1.5 1.5 0 0 1-1.5 1.5h-6A1.5 1.5 0 0 1 7 11.5v-8Z" />
                    <path d="M4 6.5A1.5 1.5 0 0 1 5.5 5H6v6.5A2.5 2.5 0 0 0 8.5 14H13v.5a1.5 1.5 0 0 1-1.5 1.5h-6A1.5 1.5 0 0 1 4 14.5v-8Z" />
                  </svg>
                )}
              </button>
            </div>
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

          <div className="mt-4">
            <button
              type="button"
              onClick={onClose}
              className="rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-500 dark:bg-blue-500 dark:hover:bg-blue-400"
            >
              Done
            </button>
          </div>
        </>
      )}
    </div>
  );
}
