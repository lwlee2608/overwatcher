import { useCallback, useEffect, useState } from "react";
import type { APIKeyResponse, CreateAPIKeyResponse } from "../types/apiKey";
import { createAPIKey, deleteAPIKey, fetchAPIKeys } from "../api/apiKeys";
import { timeAgo } from "../utils/time";
import { CopyButton } from "./CopyButton";

export function APIKeysDashboard() {
  const [keys, setKeys] = useState<APIKeyResponse[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [name, setName] = useState("");
  const [saving, setSaving] = useState(false);
  const [created, setCreated] = useState<CreateAPIKeyResponse | null>(null);

  const loadData = useCallback(async () => {
    try {
      const res = await fetchAPIKeys();
      setKeys(res.keys ?? []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to fetch");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadData();
  }, [loadData]);

  function closeForm() {
    setShowForm(false);
    setName("");
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      setCreated(await createAPIKey(name.trim()));
      closeForm();
      await loadData();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Create failed");
    } finally {
      setSaving(false);
    }
  }

  async function handleRevoke(k: APIKeyResponse) {
    if (!window.confirm(`Revoke API key "${k.name}"? Anything using it will stop working.`)) return;
    try {
      await deleteAPIKey(k.id);
      if (created?.id === k.id) setCreated(null);
      await loadData();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Revoke failed");
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
            {keys.length}
          </span>{" "}
          API key{keys.length !== 1 && "s"}
        </div>
        {!showForm && (
          <button
            onClick={() => setShowForm(true)}
            className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
          >
            New API key
          </button>
        )}
      </div>

      <p className="mb-6 text-sm text-gray-600 dark:text-gray-400">
        API keys let scripts and AI agents call the Overwatcher API as you, e.g.
        to create projects. Send one as{" "}
        <code className="font-mono text-xs">Authorization: Bearer owk_…</code>.
        Keys cannot manage other keys or change your password.
      </p>

      {created && (
        <div className="mb-6 rounded-lg border border-green-200 bg-green-50 p-4 dark:border-green-800 dark:bg-green-900/20">
          <div className="mb-2 flex items-center justify-between">
            <span className="text-sm font-medium text-green-800 dark:text-green-300">
              Key "{created.name}" created. Copy it now — it won't be shown again.
            </span>
            <button
              onClick={() => setCreated(null)}
              className="text-sm text-green-700 hover:text-green-900 dark:text-green-400 dark:hover:text-green-200"
            >
              Dismiss
            </button>
          </div>
          <div className="relative">
            <pre className="overflow-x-auto rounded bg-gray-900 p-3 pr-12 text-xs text-gray-100 dark:bg-gray-950">
              <code>{created.key}</code>
            </pre>
            <div className="absolute right-2 top-2">
              <CopyButton text={created.key} label="Copy API key" />
            </div>
          </div>
        </div>
      )}

      {showForm && (
        <form
          onSubmit={handleSubmit}
          className="mb-6 rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800"
        >
          <h3 className="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-4">
            New API key
          </h3>
          <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
            Name
          </label>
          <input
            type="text"
            required
            autoFocus
            placeholder="e.g. claude-code"
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
          />
          <div className="mt-4 flex gap-2">
            <button
              type="submit"
              disabled={saving || !name.trim()}
              className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
            >
              {saving ? "Creating..." : "Create"}
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

      {keys.length === 0 && !error && (
        <div className="text-center py-12 text-gray-400 dark:text-gray-500">
          No API keys yet
        </div>
      )}

      {keys.length > 0 && (
        <div className="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr className="text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                <th className="px-4 py-3">Name</th>
                <th className="px-4 py-3">Created</th>
                <th className="px-4 py-3">Last used</th>
                <th className="px-4 py-3"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 bg-white dark:divide-gray-700 dark:bg-gray-900">
              {keys.map((k) => (
                <tr key={k.id}>
                  <td className="px-4 py-3 font-mono text-gray-900 dark:text-gray-100">
                    {k.name}
                  </td>
                  <td className="px-4 py-3 text-gray-700 dark:text-gray-300">
                    {timeAgo(k.created_at)}
                  </td>
                  <td className="px-4 py-3 text-gray-700 dark:text-gray-300">
                    {k.last_used_at ? (
                      timeAgo(k.last_used_at)
                    ) : (
                      <span className="text-gray-400 dark:text-gray-500">Never</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <button
                      onClick={() => handleRevoke(k)}
                      className="text-sm text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300"
                    >
                      Revoke
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
