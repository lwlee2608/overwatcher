import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { deleteProject } from "../api/projects";

interface Props {
  projectId: string;
  projectName: string;
  onClose: () => void;
}

export function DeleteProjectModal({ projectId, projectName, onClose }: Props) {
  const navigate = useNavigate();
  const [confirmText, setConfirmText] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  const confirmed = confirmText === projectName;

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape" && !deleting) onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [deleting, onClose]);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!confirmed) return;
    setDeleting(true);
    setError(null);
    try {
      await deleteProject(projectId);
      navigate("/projects", { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Delete failed");
      setDeleting(false);
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4"
      onClick={() => !deleting && onClose()}
    >
      <div
        className="w-full max-w-sm rounded-lg border border-gray-200 bg-white p-6 shadow-lg dark:border-gray-700 dark:bg-gray-800"
        onClick={(e) => e.stopPropagation()}
      >
        <h3 className="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-2">
          Delete project
        </h3>
        <p className="mb-4 text-sm text-gray-600 dark:text-gray-400">
          This permanently deletes{" "}
          <span className="font-semibold text-gray-900 dark:text-gray-100">
            {projectName}
          </span>
          , its services, and member access. This action cannot be undone.
        </p>

        {error && (
          <div className="mb-4 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit}>
          <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
            Type{" "}
            <span className="font-mono text-gray-700 dark:text-gray-300">
              {projectName}
            </span>{" "}
            to confirm
          </label>
          <input
            type="text"
            autoFocus
            autoComplete="off"
            value={confirmText}
            onChange={(e) => setConfirmText(e.target.value)}
            className="mb-5 w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
          />

          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={onClose}
              disabled={deleting}
              className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={!confirmed || deleting}
              className="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {deleting ? "Deleting..." : "Delete project"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
