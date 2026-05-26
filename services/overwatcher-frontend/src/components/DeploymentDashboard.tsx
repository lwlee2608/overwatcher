import { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import type { Deployment } from "../types/deployment";
import type { ProjectResponse } from "../types/project";
import {
  fetchDeployments,
  redeployDeployment,
  type DeploymentFilter,
} from "../api/deployments";
import { fetchProjects } from "../api/projects";
import { timeAgo } from "../utils/time";
import { Pagination } from "./Pagination";

const statusColors: Record<string, string> = {
  created:
    "bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400",
  dispatched:
    "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200",
  succeeded:
    "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200",
  failed:
    "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200",
  permanently_failed:
    "bg-red-200 text-red-900 dark:bg-red-950 dark:text-red-300",
};

const defaultStatusColor =
  "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200";

const STATUS_OPTIONS = [
  "created",
  "dispatched",
  "succeeded",
  "failed",
  "permanently_failed",
];

const inputClass =
  "rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 px-2 py-1 text-sm text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-1 focus:ring-blue-500";

export function DeploymentDashboard() {
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [total, setTotal] = useState(0);
  const [projects, setProjects] = useState<ProjectResponse[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [redeployingId, setRedeployingId] = useState<string | null>(null);

  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(25);
  const [filter, setFilter] = useState<DeploymentFilter>({});
  // Repo filter is debounced so typing doesn't fire a request per keystroke.
  const [repoInput, setRepoInput] = useState("");

  const refresh = useCallback(async () => {
    try {
      const [d, p] = await Promise.all([
        fetchDeployments(page, pageSize, filter),
        fetchProjects(),
      ]);
      setDeployments(d.deployments ?? []);
      setTotal(d.total ?? 0);
      setProjects(p.projects ?? []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to fetch");
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, filter]);

  const projectById = new Map(projects.map((p) => [p.id, p]));

  useEffect(() => {
    refresh();
    const id = setInterval(refresh, 10_000);
    return () => clearInterval(id);
  }, [refresh]);

  // Debounce repo text input by 300ms before pushing it into the filter state.
  const debounceRef = useRef<number | undefined>(undefined);
  useEffect(() => {
    window.clearTimeout(debounceRef.current);
    debounceRef.current = window.setTimeout(() => {
      setFilter((f) => ({ ...f, repo: repoInput || undefined }));
      setPage(1);
    }, 300);
    return () => window.clearTimeout(debounceRef.current);
  }, [repoInput]);

  const updateFilter = (patch: Partial<DeploymentFilter>) => {
    setFilter((f) => ({ ...f, ...patch }));
    setPage(1);
  };

  const handleRedeploy = async (id: string) => {
    setRedeployingId(id);
    try {
      await redeployDeployment(id);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to redeploy");
    } finally {
      setRedeployingId(null);
    }
  };

  const environmentOptions = Array.from(
    new Set(deployments.map((d) => d.environment).filter(Boolean)),
  ).sort();

  if (loading) {
    return (
      <div className="max-w-7xl mx-auto text-center py-12 text-gray-400 dark:text-gray-500">
        Loading...
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto">
      {error && (
        <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400">
          {error}
        </div>
      )}

      <div className="mb-4 flex flex-wrap items-center gap-2">
        <select
          value={filter.status ?? ""}
          onChange={(e) =>
            updateFilter({ status: e.target.value || undefined })
          }
          className={inputClass}
        >
          <option value="">All statuses</option>
          {STATUS_OPTIONS.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </select>
        <select
          value={filter.project_id ?? ""}
          onChange={(e) =>
            updateFilter({ project_id: e.target.value || undefined })
          }
          className={inputClass}
        >
          <option value="">All projects</option>
          {projects.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>
        <input
          type="text"
          value={repoInput}
          onChange={(e) => setRepoInput(e.target.value)}
          placeholder="repo contains…"
          className={inputClass}
        />
        <select
          value={filter.environment ?? ""}
          onChange={(e) =>
            updateFilter({ environment: e.target.value || undefined })
          }
          className={inputClass}
        >
          <option value="">All envs</option>
          {environmentOptions.map((e) => (
            <option key={e} value={e}>
              {e}
            </option>
          ))}
        </select>
        {(filter.status ||
          filter.project_id ||
          filter.repo ||
          filter.environment) && (
          <button
            type="button"
            onClick={() => {
              setFilter({});
              setRepoInput("");
              setPage(1);
            }}
            className="text-xs text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300"
          >
            Clear filters
          </button>
        )}
      </div>

      {deployments.length === 0 && !error ? (
        <div className="text-center py-12 text-gray-400 dark:text-gray-500">
          {total === 0 ? "No deployments yet" : "No deployments match"}
        </div>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr className="text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                <th className="px-4 py-3">Status</th>
                <th className="px-4 py-3">Project</th>
                <th className="px-4 py-3">Repository</th>
                <th className="px-4 py-3">SHA</th>
                <th className="px-4 py-3">Agent</th>
                <th className="px-4 py-3">Services</th>
                <th className="px-4 py-3">Env</th>
                <th className="px-4 py-3">Time</th>
                <th className="px-4 py-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 bg-white dark:divide-gray-700 dark:bg-gray-900">
              {deployments.map((d) => {
                const project = d.project_id
                  ? projectById.get(d.project_id)
                  : undefined;
                return (
                  <tr key={d.id}>
                    <td className="px-4 py-3">
                      <span
                        className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${statusColors[d.status] ?? defaultStatusColor}`}
                      >
                        {d.status}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-gray-700 dark:text-gray-300">
                      {project ? (
                        <Link
                          to={`/projects/${project.id}`}
                          className="text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300"
                        >
                          {project.name}
                        </Link>
                      ) : (
                        <span className="text-gray-400 dark:text-gray-500">
                          —
                        </span>
                      )}
                    </td>
                    <td className="px-4 py-3 font-mono text-gray-900 dark:text-gray-100">
                      {d.repo}
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-gray-700 dark:text-gray-300">
                      {d.sha.slice(0, 7)}
                    </td>
                    <td className="px-4 py-3 text-gray-700 dark:text-gray-300">
                      {d.stack}
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-gray-700 dark:text-gray-300">
                      {d.services.length === 0 ? (
                        <span className="text-gray-400 dark:text-gray-500">
                          —
                        </span>
                      ) : (
                        <div className="flex flex-col gap-0.5">
                          {d.services.map((s, i) => {
                            const fullImage = `${s.image}:${s.tag}`;
                            const lastSlash = s.image.lastIndexOf("/");
                            const shortImage =
                              lastSlash >= 0
                                ? s.image.slice(lastSlash + 1)
                                : s.image;
                            return (
                              <div key={i} title={fullImage}>
                                {s.name && (
                                  <span className="mr-1 text-gray-500 dark:text-gray-400">
                                    {s.name}:
                                  </span>
                                )}
                                {shortImage}:{s.tag}
                              </div>
                            );
                          })}
                        </div>
                      )}
                    </td>
                    <td className="px-4 py-3 text-gray-700 dark:text-gray-300">
                      {d.environment}
                    </td>
                    <td className="px-4 py-3 text-gray-500 dark:text-gray-400 whitespace-nowrap">
                      {timeAgo(d.created_at)}
                    </td>
                    <td className="px-4 py-3 text-right whitespace-nowrap">
                      <button
                        onClick={() => handleRedeploy(d.id)}
                        disabled={redeployingId === d.id}
                        className="text-sm text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300 disabled:opacity-50 disabled:cursor-not-allowed"
                      >
                        {redeployingId === d.id ? "Redeploying…" : "Redeploy"}
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <Pagination
        page={page}
        pageSize={pageSize}
        total={total}
        onPageChange={setPage}
        onPageSizeChange={(s) => {
          setPageSize(s);
          setPage(1);
        }}
      />
    </div>
  );
}
