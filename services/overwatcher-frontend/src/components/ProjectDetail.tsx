import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import type {
  ComposeServiceResponse,
  ProjectMemberResponse,
  ProjectResponse,
} from "../types/project";
import type { AgentStatus } from "../types/agent";
import type { UserResponse } from "../types/user";
import {
  addProjectMember,
  fetchProject,
  fetchProjectMembers,
  removeProjectMember,
  replaceProjectServices,
} from "../api/projects";
import { bindAgentProject, fetchAgents } from "../api/agents";
import { fetchUsers } from "../api/users";
import { useAuth } from "../auth/context";

interface ServiceRow {
  name: string;
  repo: string;
  root_directory: string;
  branch: string;
  image: string;
  tag: string;
  workflow: string;
}

const emptyRow = (): ServiceRow => ({
  name: "",
  repo: "",
  root_directory: "/",
  branch: "",
  image: "",
  tag: "latest",
  workflow: "",
});

function normalizeRepo(input: string): string {
  let r = input.trim();
  for (const prefix of [
    "https://github.com/",
    "http://github.com/",
    "git@github.com:",
  ]) {
    if (r.startsWith(prefix)) {
      r = r.slice(prefix.length);
      break;
    }
  }
  if (r.endsWith("/")) r = r.slice(0, -1);
  if (r.endsWith(".git")) r = r.slice(0, -4);
  return r;
}

function toRow(s: ComposeServiceResponse): ServiceRow {
  return {
    name: s.name,
    repo: s.repo,
    root_directory: s.root_directory,
    branch: s.branch,
    image: s.image,
    tag: s.tag,
    workflow: s.workflow ?? "",
  };
}

function AutoInput({
  className = "",
  onKeyDown,
  ...props
}: React.TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return (
    <textarea
      rows={1}
      onKeyDown={(e) => {
        if (e.key === "Enter") e.preventDefault();
        onKeyDown?.(e);
      }}
      className={`field-sizing-content resize-none break-all ${className}`}
      {...props}
    />
  );
}

function Field({
  label,
  className,
  hint,
  children,
}: {
  label: string;
  className?: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <label className={`flex flex-col gap-1 ${className ?? ""}`}>
      <span className="text-xs font-medium text-gray-500 dark:text-gray-400">
        {label}
      </span>
      {children}
      {hint && (
        <span className="text-xs text-gray-400 dark:text-gray-500">{hint}</span>
      )}
    </label>
  );
}

function ViewRow({
  label,
  value,
  muted = false,
}: {
  label: string;
  value: React.ReactNode;
  muted?: boolean;
}) {
  return (
    <div className="grid grid-cols-[80px_1fr] items-baseline gap-3 py-1">
      <span className="text-xs font-medium text-gray-500 dark:text-gray-400">
        {label}
      </span>
      <span
        className={`text-sm font-mono break-all ${
          muted
            ? "text-gray-400 dark:text-gray-500"
            : "text-gray-800 dark:text-gray-200"
        }`}
      >
        {value}
      </span>
    </div>
  );
}

function ServiceView({ row }: { row: ServiceRow }) {
  const repoLine = row.repo ? (
    <>
      {row.repo}
      {row.branch && (
        <>
          <span className="mx-1.5 text-gray-400 dark:text-gray-500">·</span>
          {row.branch}
        </>
      )}
    </>
  ) : (
    <span className="italic text-gray-400 dark:text-gray-500">not set</span>
  );
  const imageLine = row.image ? (
    <>
      {row.image}
      <span className="text-gray-400 dark:text-gray-500">:{row.tag || "latest"}</span>
    </>
  ) : (
    <span className="italic text-gray-400 dark:text-gray-500">not set</span>
  );
  return (
    <div className="px-4 py-3">
      <ViewRow label="Repo" value={repoLine} />
      <ViewRow label="Root dir" value={row.root_directory || "/"} />
      <ViewRow label="Image" value={imageLine} />
      <ViewRow
        label="Workflow"
        value={row.workflow || "—"}
        muted={!row.workflow}
      />
    </div>
  );
}

export function ProjectDetail() {
  const { id } = useParams<{ id: string }>();
  const { user } = useAuth();
  const [project, setProject] = useState<ProjectResponse | null>(null);
  const [rows, setRows] = useState<ServiceRow[]>([]);
  const [editing, setEditing] = useState<boolean[]>([]);
  const [agents, setAgents] = useState<AgentStatus[]>([]);
  const [agentSelection, setAgentSelection] = useState<string>("");
  const [bindingAgent, setBindingAgent] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);

  const load = useCallback(async () => {
    if (!id) return;
    try {
      const [p, a] = await Promise.all([fetchProject(id), fetchAgents()]);
      setProject(p);
      const loadedRows = (p.services ?? []).map(toRow);
      setRows(loadedRows);
      setEditing(loadedRows.map(() => false));
      const agentList = a.agents ?? [];
      setAgents(agentList);
      const bound = agentList.find((ag) => ag.project_id === p.id);
      setAgentSelection(bound?.id ?? "");
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
    setEditing((es) => [...es, true]);
    setDirty(true);
  }

  function removeRow(i: number) {
    setRows((rs) => rs.filter((_, idx) => idx !== i));
    setEditing((es) => es.filter((_, idx) => idx !== i));
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
    setEditing((es) => {
      const copy = es.slice();
      [copy[i], copy[j]] = [copy[j], copy[i]];
      return copy;
    });
    setDirty(true);
  }

  function toggleEdit(i: number) {
    setEditing((es) => es.map((v, idx) => (idx === i ? !v : v)));
  }

  async function handleBindAgent() {
    if (!project) return;
    setBindingAgent(true);
    setError(null);
    try {
      const boundNow = agents.find((ag) => ag.project_id === project.id);
      if (agentSelection === "") {
        if (boundNow) {
          await bindAgentProject(boundNow.id, { project_id: "" });
        }
      } else {
        await bindAgentProject(agentSelection, { project_id: project.id });
      }
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Bind failed");
    } finally {
      setBindingAgent(false);
    }
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
        workflow: r.workflow.trim(),
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

  const isOwner = project.role === "owner";

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

      <div className="mb-6 rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800">
        <div className="mb-2 flex items-center justify-between">
          <h2 className="text-sm font-semibold text-gray-900 dark:text-gray-100">
            Agent
          </h2>
          {(() => {
            const bound = agents.find((a) => a.project_id === project.id);
            if (!bound) {
              return (
                <span className="text-xs text-gray-500 dark:text-gray-400">
                  Not bound
                </span>
              );
            }
            const badgeClass = {
              connected:
                "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200",
              stale:
                "bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200",
              disconnected:
                "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200",
              lost:
                "bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400",
            }[bound.status];
            return (
              <span
                className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${badgeClass}`}
              >
                {bound.status}
              </span>
            );
          })()}
        </div>
        <p className="mb-3 text-xs text-gray-500 dark:text-gray-400">
          One agent runs this project. Changing this moves future deploys to the
          selected agent.
        </p>
        <div className="flex items-center gap-2">
          <select
            value={agentSelection}
            onChange={(e) => setAgentSelection(e.target.value)}
            className="flex-1 rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
          >
            <option value="">— Unbound —</option>
            {agents.map((a) => {
              const boundElsewhere =
                a.project_id && a.project_id !== project.id;
              return (
                <option key={a.id} value={a.id}>
                  {a.name}
                  {boundElsewhere ? " (reassign)" : ""}
                  {a.status === "lost"
                    ? " [lost]"
                    : a.status === "disconnected"
                      ? " [offline]"
                      : a.status === "stale"
                        ? " [stale]"
                        : ""}
                </option>
              );
            })}
          </select>
          <button
            type="button"
            onClick={handleBindAgent}
            disabled={
              !isOwner ||
              bindingAgent ||
              agentSelection ===
                (agents.find((a) => a.project_id === project.id)?.id ?? "")
            }
            className="rounded-lg bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {bindingAgent ? "Saving..." : "Apply"}
          </button>
        </div>
      </div>

      <MembersPanel
        projectId={project.id}
        currentUserId={user?.id ?? ""}
        isOwner={isOwner}
      />

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
        <div className="space-y-3">
          {rows.map((r, i) => {
            const isEditing = editing[i];
            return (
              <div
                key={i}
                className="rounded-lg border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800"
              >
                <div className="flex items-center justify-between gap-2 border-b border-gray-100 px-4 py-2.5 dark:border-gray-700/60">
                  <div className="flex items-baseline gap-2 min-w-0">
                    <span className="text-[10px] font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">
                      #{i + 1}
                    </span>
                    <span className="truncate text-sm font-medium text-gray-900 dark:text-gray-100">
                      {r.name || (
                        <span className="italic text-gray-400 dark:text-gray-500">
                          unnamed
                        </span>
                      )}
                    </span>
                  </div>
                  <div className="flex items-center gap-1">
                    <button
                      type="button"
                      onClick={() => moveRow(i, -1)}
                      disabled={i === 0}
                      className="rounded-md border border-gray-300 px-2 py-1 text-xs text-gray-600 hover:bg-gray-50 disabled:opacity-30 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-700"
                      aria-label="Move up"
                    >
                      ▲
                    </button>
                    <button
                      type="button"
                      onClick={() => moveRow(i, 1)}
                      disabled={i === rows.length - 1}
                      className="rounded-md border border-gray-300 px-2 py-1 text-xs text-gray-600 hover:bg-gray-50 disabled:opacity-30 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-700"
                      aria-label="Move down"
                    >
                      ▼
                    </button>
                    <button
                      type="button"
                      onClick={() => toggleEdit(i)}
                      className="ml-1 rounded-md border border-gray-300 px-2 py-1 text-xs font-medium text-gray-700 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
                    >
                      {isEditing ? "Done" : "Edit"}
                    </button>
                    <button
                      type="button"
                      onClick={() => removeRow(i)}
                      className="rounded-md border border-gray-300 px-2 py-1 text-xs text-gray-600 hover:border-red-300 hover:bg-red-50 hover:text-red-600 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                      aria-label="Remove service"
                    >
                      Remove
                    </button>
                  </div>
                </div>

                {isEditing ? (
                  <div className="grid grid-cols-12 gap-3 p-4">
                    <Field label="Name" className="col-span-12">
                      <AutoInput
                        placeholder="web"
                        value={r.name}
                        onChange={(e) => updateRow(i, { name: e.target.value })}
                        className="w-full rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
                      />
                    </Field>

                    <Field label="Repo" className="col-span-12">
                      <AutoInput
                        placeholder="owner/repo"
                        value={r.repo}
                        onChange={(e) => updateRow(i, { repo: e.target.value })}
                        onBlur={(e) => {
                          const normalized = normalizeRepo(e.target.value);
                          if (normalized !== e.target.value) {
                            updateRow(i, { repo: normalized });
                          }
                        }}
                        className="w-full rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm font-mono text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
                      />
                    </Field>
                    <Field label="Branch" className="col-span-12">
                      <AutoInput
                        placeholder="main"
                        value={r.branch}
                        onChange={(e) =>
                          updateRow(i, { branch: e.target.value })
                        }
                        className="w-full rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm font-mono text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
                      />
                    </Field>

                    <Field label="Root dir" className="col-span-12">
                      <AutoInput
                        placeholder="/"
                        value={r.root_directory}
                        onChange={(e) =>
                          updateRow(i, { root_directory: e.target.value })
                        }
                        className="w-full rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm font-mono text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
                      />
                    </Field>

                    <Field label="Image" className="col-span-12">
                      <AutoInput
                        placeholder="ghcr.io/owner/image"
                        value={r.image}
                        onChange={(e) =>
                          updateRow(i, { image: e.target.value })
                        }
                        className="w-full rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm font-mono text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
                      />
                    </Field>
                    <Field label="Tag" className="col-span-12">
                      <AutoInput
                        placeholder="latest"
                        value={r.tag}
                        onChange={(e) => updateRow(i, { tag: e.target.value })}
                        className="w-full rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm font-mono text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
                      />
                    </Field>

                    <Field
                      label="Workflow"
                      className="col-span-12"
                      hint="Optional. If set, deploys wait for this workflow to succeed."
                    >
                      <AutoInput
                        placeholder="build.yml"
                        value={r.workflow}
                        onChange={(e) =>
                          updateRow(i, { workflow: e.target.value })
                        }
                        className="w-full rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm font-mono text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
                      />
                    </Field>
                  </div>
                ) : (
                  <ServiceView row={r} />
                )}
              </div>
            );
          })}
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

function MembersPanel({
  projectId,
  currentUserId,
  isOwner,
}: {
  projectId: string;
  currentUserId: string;
  isOwner: boolean;
}) {
  const [members, setMembers] = useState<ProjectMemberResponse[]>([]);
  const [email, setEmail] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [users, setUsers] = useState<UserResponse[]>([]);
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [activeIdx, setActiveIdx] = useState(-1);

  const load = useCallback(async () => {
    try {
      const r = await fetchProjectMembers(projectId);
      setMembers(r.members ?? []);
      setError(null);
      setLoaded(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to fetch members");
      setLoaded(true);
    }
  }, [projectId]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (!isOwner) return;
    fetchUsers()
      .then((r) => setUsers(r.users ?? []))
      .catch(() => {});
  }, [isOwner]);

  const suggestions = useMemo(() => {
    const q = email.trim().toLowerCase();
    const memberIds = new Set(members.map((m) => m.user_id));
    return users
      .filter(
        (u) =>
          u.id !== currentUserId &&
          !memberIds.has(u.id) &&
          (u.email.toLowerCase().includes(q) ||
            u.name.toLowerCase().includes(q))
      )
      .slice(0, 8);
  }, [users, members, email, currentUserId]);

  function selectUser(u: UserResponse) {
    setEmail(u.email);
    setShowSuggestions(false);
    setActiveIdx(-1);
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (!showSuggestions || suggestions.length === 0) return;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIdx((i) => (i + 1) % suggestions.length);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIdx((i) => (i <= 0 ? suggestions.length - 1 : i - 1));
    } else if (e.key === "Enter" && activeIdx >= 0) {
      e.preventDefault();
      selectUser(suggestions[activeIdx]);
    } else if (e.key === "Escape") {
      setShowSuggestions(false);
      setActiveIdx(-1);
    }
  }

  async function handleAdd(e: React.FormEvent) {
    e.preventDefault();
    if (!email.trim()) return;
    setBusy(true);
    setError(null);
    try {
      await addProjectMember(projectId, { email: email.trim() });
      setEmail("");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Add failed");
    } finally {
      setBusy(false);
    }
  }

  async function handleRemove(userId: string) {
    const isSelf = userId === currentUserId;
    const confirmMsg = isSelf
      ? "Leave this project?"
      : "Remove this member?";
    if (!window.confirm(confirmMsg)) return;
    setBusy(true);
    setError(null);
    try {
      await removeProjectMember(projectId, userId);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Remove failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mb-6 rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800">
      <div className="mb-2 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-gray-900 dark:text-gray-100">
          Members
        </h2>
        <span className="text-xs text-gray-500 dark:text-gray-400">
          {members.length} member{members.length !== 1 && "s"}
        </span>
      </div>
      <p className="mb-3 text-xs text-gray-500 dark:text-gray-400">
        Members can view this project, edit services, and trigger deploys.
        Project settings and agent binding are owner-only.
      </p>

      {error && (
        <div className="mb-3 rounded-lg border border-red-200 bg-red-50 p-2 text-xs text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400">
          {error}
        </div>
      )}

      {isOwner && (
        <form onSubmit={handleAdd} className="mb-3 flex items-center gap-2">
          <div className="relative flex-1">
            <input
              type="email"
              placeholder="user@example.com"
              value={email}
              onChange={(e) => {
                setEmail(e.target.value);
                setShowSuggestions(true);
                setActiveIdx(-1);
              }}
              onFocus={() => setShowSuggestions(true)}
              onBlur={() => {
                setShowSuggestions(false);
                setActiveIdx(-1);
              }}
              onKeyDown={handleKeyDown}
              className="w-full rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
            />
            {showSuggestions && suggestions.length > 0 && (
              <ul className="absolute z-10 mt-1 max-h-48 w-full overflow-auto rounded-md border border-gray-300 bg-white shadow-lg dark:border-gray-600 dark:bg-gray-700">
                {suggestions.map((u, i) => (
                  <li key={u.id}>
                    <button
                      type="button"
                      onMouseDown={(e) => {
                        e.preventDefault();
                        selectUser(u);
                      }}
                      onMouseEnter={() => setActiveIdx(i)}
                      className={`block w-full px-2 py-1.5 text-left text-sm ${
                        i === activeIdx ? "bg-gray-100 dark:bg-gray-600" : ""
                      }`}
                    >
                      <span className="text-gray-900 dark:text-gray-100">
                        {u.email}
                      </span>
                      {u.name && (
                        <span className="ml-2 text-xs text-gray-500 dark:text-gray-400">
                          {u.name}
                        </span>
                      )}
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
          <button
            type="submit"
            disabled={busy || !email.trim()}
            className="rounded-lg bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
          >
            Add
          </button>
        </form>
      )}

      {loaded && members.length === 0 ? (
        <div className="text-xs text-gray-400 dark:text-gray-500">
          No additional members yet.
        </div>
      ) : (
        <ul className="divide-y divide-gray-200 dark:divide-gray-700">
          {members.map((m) => {
            const isSelf = m.user_id === currentUserId;
            const canRemove = isOwner || isSelf;
            return (
              <li
                key={m.user_id}
                className="flex items-center justify-between py-2 text-sm"
              >
                <div className="min-w-0">
                  <div className="text-gray-900 dark:text-gray-100">
                    {m.user_email}
                    {isSelf && (
                      <span className="ml-1 text-xs text-gray-400 dark:text-gray-500">
                        (you)
                      </span>
                    )}
                  </div>
                  {m.user_name && (
                    <div className="text-xs text-gray-500 dark:text-gray-400">
                      {m.user_name}
                    </div>
                  )}
                </div>
                {canRemove && (
                  <button
                    type="button"
                    onClick={() => handleRemove(m.user_id)}
                    disabled={busy}
                    className="text-xs text-red-600 hover:text-red-800 disabled:opacity-50 dark:text-red-400 dark:hover:text-red-300"
                  >
                    {isSelf ? "Leave" : "Remove"}
                  </button>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
