import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import type {
  ComposeServiceResponse,
  ProjectResponse,
} from "../types/project";
import { fetchProject, replaceProjectServices } from "../api/projects";

interface ServiceRow {
  name: string;
  repo: string;
  root_directory: string;
  branch: string;
  image: string;
  tag: string;
}

const emptyRow = (): ServiceRow => ({
  name: "",
  repo: "",
  root_directory: "/",
  branch: "",
  image: "",
  tag: "latest",
});

function toRow(s: ComposeServiceResponse): ServiceRow {
  return {
    name: s.name,
    repo: s.repo,
    root_directory: s.root_directory,
    branch: s.branch,
    image: s.image,
    tag: s.tag,
  };
}

export function ProjectDetail() {
  const { id } = useParams<{ id: string }>();
  const [project, setProject] = useState<ProjectResponse | null>(null);
  const [rows, setRows] = useState<ServiceRow[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);

  const load = useCallback(async () => {
    if (!id) return;
    try {
      const p = await fetchProject(id);
      setProject(p);
      setRows((p.services ?? []).map(toRow));
      setDirty(false);
      setError(null);
      setLoading(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to fetch");
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  function updateRow(i: number, patch: Partial<ServiceRow>) {
    setRows((rs) => rs.map((r, idx) => (idx === i ? { ...r, ...patch } : r)));
    setDirty(true);
  }

  function addRow() {
    setRows((rs) => [...rs, emptyRow()]);
    setDirty(true);
  }

  function removeRow(i: number) {
    setRows((rs) => rs.filter((_, idx) => idx !== i));
    setDirty(true);
  }

  function moveRow(i: number, dir: -1 | 1) {
    const j = i + dir;
    if (j < 0 || j >= rows.length) return;
    setRows((rs) => {
      const copy = rs.slice();
      [copy[i], copy[j]] = [copy[j], copy[i]];
      return copy;
    });
    setDirty(true);
  }

  async function handleSave() {
    if (!id) return;
    setSaving(true);
    setError(null);
    try {
      const services = rows.map((r, i) => ({
        name: r.name.trim(),
        repo: r.repo.trim(),
        root_directory: r.root_directory.trim() || "/",
        branch: r.branch.trim(),
        image: r.image.trim(),
        tag: r.tag.trim() || "latest",
        position: i,
      }));
      await replaceProjectServices(id, { services });
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed");
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return (
      <div className="max-w-5xl mx-auto text-center py-12 text-gray-400 dark:text-gray-500">
        Loading...
      </div>
    );
  }

  if (!project) {
    return (
      <div className="max-w-5xl mx-auto py-12">
        <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400">
          {error || "Project not found"}
        </div>
        <Link
          to="/projects"
          className="mt-4 inline-block text-sm text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300"
        >
          ← Back to projects
        </Link>
      </div>
    );
  }

  return (
    <div className="max-w-5xl mx-auto">
      <div className="mb-4">
        <Link
          to="/projects"
          className="text-sm text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300"
        >
          ← Projects
        </Link>
      </div>

      <div className="mb-6">
        <h1 className="text-xl font-semibold text-gray-900 dark:text-gray-100">
          {project.name}
        </h1>
        {project.description && (
          <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">
            {project.description}
          </p>
        )}
        <div className="mt-2 flex flex-wrap gap-3 text-xs text-gray-500 dark:text-gray-400">
          <span>
            Owner:{" "}
            <span className="font-mono text-gray-700 dark:text-gray-300">
              {project.user_email || project.user_id}
            </span>
          </span>
          <span>
            Env:{" "}
            <span className="font-mono text-gray-700 dark:text-gray-300">
              {project.environment}
            </span>
          </span>
          <span>
            Compose:{" "}
            <span className="font-mono text-gray-700 dark:text-gray-300">
              {project.compose_file}
            </span>
          </span>
          <span
            className={`inline-block rounded-full px-2 py-0.5 font-medium ${
              project.enabled
                ? "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"
                : "bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400"
            }`}
          >
            {project.enabled ? "enabled" : "disabled"}
          </span>
        </div>
      </div>

      {error && (
        <div className="mb-4 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400">
          {error}
        </div>
      )}

      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-gray-900 dark:text-gray-100">
          Services
        </h2>
        <div className="flex gap-2">
          <button
            onClick={addRow}
            className="rounded-lg border border-gray-300 px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
          >
            + Add service
          </button>
          <button
            onClick={handleSave}
            disabled={!dirty || saving}
            className="rounded-lg bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {saving ? "Saving..." : "Save services"}
          </button>
        </div>
      </div>

      {rows.length === 0 ? (
        <div className="rounded-lg border border-dashed border-gray-300 bg-white p-8 text-center text-sm text-gray-400 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-500">
          No services. Add one to start receiving deploys.
        </div>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr className="text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                <th className="px-3 py-2 w-8"></th>
                <th className="px-3 py-2">Name</th>
                <th className="px-3 py-2">Repo</th>
                <th className="px-3 py-2">Root dir</th>
                <th className="px-3 py-2">Branch</th>
                <th className="px-3 py-2">Image</th>
                <th className="px-3 py-2">Tag</th>
                <th className="px-3 py-2"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 bg-white dark:divide-gray-700 dark:bg-gray-900">
              {rows.map((r, i) => (
                <tr key={i}>
                  <td className="px-2 py-2 align-middle">
                    <div className="flex flex-col">
                      <button
                        type="button"
                        onClick={() => moveRow(i, -1)}
                        disabled={i === 0}
                        className="text-xs text-gray-400 hover:text-gray-700 disabled:opacity-30 dark:hover:text-gray-200"
                        aria-label="Move up"
                      >
                        ▲
                      </button>
                      <button
                        type="button"
                        onClick={() => moveRow(i, 1)}
                        disabled={i === rows.length - 1}
                        className="text-xs text-gray-400 hover:text-gray-700 disabled:opacity-30 dark:hover:text-gray-200"
                        aria-label="Move down"
                      >
                        ▼
                      </button>
                    </div>
                  </td>
                  <td className="px-2 py-2">
                    <input
                      type="text"
                      placeholder="web"
                      value={r.name}
                      onChange={(e) => updateRow(i, { name: e.target.value })}
                      className="w-full rounded-md border border-gray-300 bg-white px-2 py-1 text-sm text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
                    />
                  </td>
                  <td className="px-2 py-2">
                    <input
                      type="text"
                      placeholder="owner/repo"
                      value={r.repo}
                      onChange={(e) => updateRow(i, { repo: e.target.value })}
                      className="w-full rounded-md border border-gray-300 bg-white px-2 py-1 text-sm font-mono text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
                    />
                  </td>
                  <td className="px-2 py-2">
                    <input
                      type="text"
                      placeholder="/"
                      value={r.root_directory}
                      onChange={(e) =>
                        updateRow(i, { root_directory: e.target.value })
                      }
                      className="w-full rounded-md border border-gray-300 bg-white px-2 py-1 text-sm font-mono text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
                    />
                  </td>
                  <td className="px-2 py-2">
                    <input
                      type="text"
                      placeholder="main"
                      value={r.branch}
                      onChange={(e) => updateRow(i, { branch: e.target.value })}
                      className="w-full rounded-md border border-gray-300 bg-white px-2 py-1 text-sm font-mono text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
                    />
                  </td>
                  <td className="px-2 py-2">
                    <input
                      type="text"
                      placeholder="ghcr.io/owner/image"
                      value={r.image}
                      onChange={(e) => updateRow(i, { image: e.target.value })}
                      className="w-full rounded-md border border-gray-300 bg-white px-2 py-1 text-sm font-mono text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
                    />
                  </td>
                  <td className="px-2 py-2">
                    <input
                      type="text"
                      placeholder="latest"
                      value={r.tag}
                      onChange={(e) => updateRow(i, { tag: e.target.value })}
                      className="w-full rounded-md border border-gray-300 bg-white px-2 py-1 text-sm font-mono text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
                    />
                  </td>
                  <td className="px-2 py-2 text-right">
                    <button
                      type="button"
                      onClick={() => removeRow(i)}
                      className="rounded-md border border-gray-300 px-2 text-sm text-gray-600 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-700"
                      aria-label="Remove service"
                    >
                      ×
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {dirty && (
        <p className="mt-2 text-xs text-amber-600 dark:text-amber-400">
          Unsaved changes
        </p>
      )}
    </div>
  );
}
