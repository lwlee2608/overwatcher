import { useCallback, useEffect, useState } from "react";
import type { AgentStatus } from "../types/agent";
import type {
  DeployMappingResponse,
  ServiceSpec,
} from "../types/mapping";
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
  services: ServiceSpec[];
  environment: string;
  enabled: boolean;
}

const emptyService = (): ServiceSpec => ({ name: "", image: "", tag: "latest" });

const emptyForm: FormState = {
  repo: "",
  agent_id: "",
  services: [emptyService()],
  environment: "production",
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
    setForm({ ...emptyForm, services: [emptyService()] });
    setShowForm(true);
  }

  function openEdit(m: DeployMappingResponse) {
    setEditingId(m.id);
    setForm({
      repo: m.repo,
      agent_id: m.agent_id,
      services:
        m.services.length > 0
          ? m.services.map((s) => ({ ...s }))
          : [emptyService()],
      environment: m.environment,
      enabled: m.enabled,
    });
    setShowForm(true);
  }

  function closeForm() {
    setShowForm(false);
    setEditingId(null);
    setForm(emptyForm);
  }

  function updateService(index: number, patch: Partial<ServiceSpec>) {
    setForm((f) => ({
      ...f,
      services: f.services.map((s, i) => (i === index ? { ...s, ...patch } : s)),
    }));
  }

  function addService() {
    setForm((f) => ({ ...f, services: [...f.services, emptyService()] }));
  }

  function removeService(index: number) {
    setForm((f) => ({
      ...f,
      services:
        f.services.length > 1
          ? f.services.filter((_, i) => i !== index)
          : f.services,
    }));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);

    const services = form.services
      .filter((s) => s.image.trim() !== "")
      .map((s) => ({
        name: s.name.trim(),
        image: s.image.trim(),
        tag: s.tag.trim() || "latest",
      }));
    if (services.length === 0) {
      setError("At least one service with an image is required");
      setSaving(false);
      return;
    }

    try {
      if (editingId) {
        await updateMapping(editingId, {
          repo: form.repo,
          agent_id: form.agent_id,
          services,
          environment: form.environment || "production",
          enabled: form.enabled,
        });
      } else {
        await createMapping({
          repo: form.repo,
          agent_id: form.agent_id,
          services,
          environment: form.environment || "production",
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
          </div>

          <div className="mt-5">
            <div className="flex items-center justify-between mb-2">
              <label className="block text-xs font-medium text-gray-500 dark:text-gray-400">
                Services
              </label>
              <button
                type="button"
                onClick={addService}
                className="text-xs text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300"
              >
                + Add service
              </button>
            </div>
            <div className="space-y-2">
              {form.services.map((svc, i) => (
                <div key={i} className="grid grid-cols-[1fr_2fr_1fr_auto] gap-2">
                  <input
                    type="text"
                    placeholder="name (blank = all)"
                    value={svc.name}
                    onChange={(e) => updateService(i, { name: e.target.value })}
                    className="rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
                  />
                  <input
                    type="text"
                    required
                    placeholder="ghcr.io/owner/image"
                    value={svc.image}
                    onChange={(e) => updateService(i, { image: e.target.value })}
                    className="rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
                  />
                  <input
                    type="text"
                    placeholder="latest"
                    value={svc.tag}
                    onChange={(e) => updateService(i, { tag: e.target.value })}
                    className="rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
                  />
                  <button
                    type="button"
                    onClick={() => removeService(i)}
                    disabled={form.services.length === 1}
                    className="rounded-md border border-gray-300 px-2 text-sm text-gray-600 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-40 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-700"
                    aria-label="Remove service"
                  >
                    ×
                  </button>
                </div>
              ))}
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
                      <div className="flex flex-col gap-1">
                        {m.services.map((s, i) => (
                          <div key={i} className="text-xs">
                            <span className="inline-block rounded bg-blue-100 px-2 py-0.5 font-medium text-blue-800 dark:bg-blue-900 dark:text-blue-200">
                              {s.name || "(all)"}
                            </span>
                            <span className="ml-2 font-mono text-gray-600 dark:text-gray-400">
                              {s.image}:{s.tag}
                            </span>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <span className="text-gray-400 dark:text-gray-500">
                        none
                      </span>
                    )}
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
