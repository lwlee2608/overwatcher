import { useCallback, useEffect, useState } from "react";
import type { AgentStatus } from "../types/agent";
import type { DeployMappingResponse } from "../types/mapping";
import { fetchAgents } from "../api/agents";
import {
  fetchMappings,
  createMapping,
  updateMapping,
  deleteMapping,
} from "../api/mappings";

interface FormState {
  repo: string;
  agent_id: string;
  services: string;
  environment: string;
  image: string;
  tag: string;
  enabled: boolean;
}

const emptyForm: FormState = {
  repo: "",
  agent_id: "",
  services: "",
  environment: "production",
  image: "",
  tag: "latest",
  enabled: true,
};

export function MappingDashboard() {
  const [mappings, setMappings] = useState<DeployMappingResponse[]>([]);
  const [agents, setAgents] = useState<AgentStatus[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [form, setForm] = useState<FormState>(emptyForm);
  const [saving, setSaving] = useState(false);

  const loadData = useCallback(async () => {
    try {
      const [m, a] = await Promise.all([fetchMappings(), fetchAgents()]);
      setMappings(m.mappings ?? []);
      setAgents(a.agents ?? []);
      setError(null);
      setLoading(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to fetch");
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadData();
    const id = setInterval(loadData, 10_000);
    return () => clearInterval(id);
  }, [loadData]);

  function openCreate() {
    setEditingId(null);
    setForm(emptyForm);
    setShowForm(true);
  }

  function openEdit(m: DeployMappingResponse) {
    setEditingId(m.id);
    setForm({
      repo: m.repo,
      agent_id: m.agent_id,
      services: m.services.join(", "),
      environment: m.environment,
      image: m.image,
      tag: m.tag,
      enabled: m.enabled,
    });
    setShowForm(true);
  }

  function closeForm() {
    setShowForm(false);
    setEditingId(null);
    setForm(emptyForm);
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);

    const services = form.services
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);

    try {
      if (editingId) {
        await updateMapping(editingId, {
          repo: form.repo,
          agent_id: form.agent_id,
          services,
          environment: form.environment || "production",
          image: form.image,
          tag: form.tag || "latest",
          enabled: form.enabled,
        });
      } else {
        await createMapping({
          repo: form.repo,
          agent_id: form.agent_id,
          services,
          environment: form.environment || "production",
          image: form.image,
          tag: form.tag || "latest",
          enabled: form.enabled,
        });
      }
      closeForm();
      await loadData();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed");
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(id: string) {
    if (!window.confirm("Delete this mapping?")) return;
    try {
      await deleteMapping(id);
      await loadData();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Delete failed");
    }
  }

  if (loading) {
    return (
      <div className="max-w-4xl mx-auto text-center py-12 text-gray-400 dark:text-gray-500">
        Loading...
      </div>
    );
  }

  return (
    <div className="max-w-4xl mx-auto">
      {error && (
        <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400">
          {error}
        </div>
      )}

      <div className="mb-6 flex items-center justify-between">
        <div className="text-sm text-gray-600 dark:text-gray-400">
          <span className="font-semibold text-gray-900 dark:text-gray-100">
            {mappings.length}
          </span>{" "}
          mapping{mappings.length !== 1 && "s"}
        </div>
        {!showForm && (
          <button
            onClick={openCreate}
            className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
          >
            Add mapping
          </button>
        )}
      </div>

      {showForm && (
        <form
          onSubmit={handleSubmit}
          className="mb-6 rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800"
        >
          <h3 className="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-4">
            {editingId ? "Edit mapping" : "New mapping"}
          </h3>
          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
                Repository
              </label>
              <input
                type="text"
                required
                placeholder="owner/repo"
                value={form.repo}
                onChange={(e) => setForm({ ...form, repo: e.target.value })}
                className="w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
                Agent
              </label>
              <select
                required
                value={form.agent_id}
                onChange={(e) => setForm({ ...form, agent_id: e.target.value })}
                className="w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
              >
                <option value="">Select an agent</option>
                {agents.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
                Services
              </label>
              <input
                type="text"
                placeholder="app, worker (comma-separated, empty = all)"
                value={form.services}
                onChange={(e) => setForm({ ...form, services: e.target.value })}
                className="w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
                Environment
              </label>
              <input
                type="text"
                placeholder="production"
                value={form.environment}
                onChange={(e) =>
                  setForm({ ...form, environment: e.target.value })
                }
                className="w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
                Image
              </label>
              <input
                type="text"
                required
                placeholder="ghcr.io/owner/repo"
                value={form.image}
                onChange={(e) => setForm({ ...form, image: e.target.value })}
                className="w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
                Tag
              </label>
              <input
                type="text"
                placeholder="latest"
                value={form.tag}
                onChange={(e) => setForm({ ...form, tag: e.target.value })}
                className="w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
              />
            </div>
          </div>
          <div className="mt-4 flex items-center gap-4">
            <label className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input
                type="checkbox"
                checked={form.enabled}
                onChange={(e) => setForm({ ...form, enabled: e.target.checked })}
                className="rounded border-gray-300 dark:border-gray-600"
              />
              Enabled
            </label>
          </div>
          <div className="mt-4 flex gap-2">
            <button
              type="submit"
              disabled={saving}
              className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
            >
              {saving ? "Saving..." : editingId ? "Update" : "Create"}
            </button>
            <button
              type="button"
              onClick={closeForm}
              className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
            >
              Cancel
            </button>
          </div>
        </form>
      )}

      {mappings.length === 0 && !error && (
        <div className="text-center py-12 text-gray-400 dark:text-gray-500">
          No mappings configured yet
        </div>
      )}

      {mappings.length > 0 && (
        <div className="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr className="text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                <th className="px-4 py-3">Repository</th>
                <th className="px-4 py-3">Agent</th>
                <th className="px-4 py-3">Services</th>
                <th className="px-4 py-3">Image</th>
                <th className="px-4 py-3">Tag</th>
                <th className="px-4 py-3">Environment</th>
                <th className="px-4 py-3">Status</th>
                <th className="px-4 py-3"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 bg-white dark:divide-gray-700 dark:bg-gray-900">
              {mappings.map((m) => (
                <tr key={m.id}>
                  <td className="px-4 py-3 font-mono text-gray-900 dark:text-gray-100">
                    {m.repo}
                  </td>
                  <td className="px-4 py-3 text-gray-700 dark:text-gray-300">
                    {m.agent_name}
                  </td>
                  <td className="px-4 py-3">
                    {m.services.length > 0 ? (
                      <div className="flex gap-1 flex-wrap">
                        {m.services.map((s) => (
                          <span
                            key={s}
                            className="inline-block rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-800 dark:bg-blue-900 dark:text-blue-200"
                          >
                            {s}
                          </span>
                        ))}
                      </div>
                    ) : (
                      <span className="text-gray-400 dark:text-gray-500">
                        all
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-3 font-mono text-gray-700 dark:text-gray-300 text-xs">
                    {m.image}
                  </td>
                  <td className="px-4 py-3 font-mono text-gray-700 dark:text-gray-300 text-xs">
                    {m.tag}
                  </td>
                  <td className="px-4 py-3 text-gray-700 dark:text-gray-300">
                    {m.environment}
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${
                        m.enabled
                          ? "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"
                          : "bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400"
                      }`}
                    >
                      {m.enabled ? "enabled" : "disabled"}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-right">
                    <button
                      onClick={() => openEdit(m)}
                      className="text-sm text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300 mr-3"
                    >
                      Edit
                    </button>
                    <button
                      onClick={() => handleDelete(m.id)}
                      className="text-sm text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300"
                    >
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
