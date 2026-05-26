interface PaginationProps {
  page: number;
  pageSize: number;
  total: number;
  onPageChange: (page: number) => void;
  onPageSizeChange: (size: number) => void;
  pageSizeOptions?: number[];
}

const DEFAULT_PAGE_SIZES = [10, 25, 50, 100];

export function Pagination({
  page,
  pageSize,
  total,
  onPageChange,
  onPageSizeChange,
  pageSizeOptions = DEFAULT_PAGE_SIZES,
}: PaginationProps) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const start = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const end = Math.min(total, page * pageSize);
  const pages = buildPageList(page, totalPages);

  return (
    <div className="flex flex-wrap items-center justify-between gap-3 px-1 py-3 text-sm text-gray-600 dark:text-gray-400">
      <div>
        Showing{" "}
        <span className="font-medium text-gray-900 dark:text-gray-100">
          {start}
        </span>
        –
        <span className="font-medium text-gray-900 dark:text-gray-100">
          {end}
        </span>{" "}
        of{" "}
        <span className="font-medium text-gray-900 dark:text-gray-100">
          {total}
        </span>
      </div>

      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={() => onPageChange(page - 1)}
          disabled={page <= 1}
          className="rounded border border-gray-300 dark:border-gray-600 px-2 py-1 text-xs disabled:opacity-40 hover:bg-gray-100 dark:hover:bg-gray-800"
        >
          ‹
        </button>
        {pages.map((p, i) =>
          p === "ellipsis" ? (
            <span
              key={`e-${i}`}
              className="px-1 text-gray-400 dark:text-gray-500 select-none"
            >
              …
            </span>
          ) : (
            <button
              key={p}
              type="button"
              onClick={() => onPageChange(p)}
              className={`min-w-[2rem] rounded border px-2 py-1 text-xs ${
                p === page
                  ? "border-blue-500 bg-blue-50 text-blue-700 dark:border-blue-400 dark:bg-blue-900/30 dark:text-blue-300"
                  : "border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-800"
              }`}
            >
              {p}
            </button>
          ),
        )}
        <button
          type="button"
          onClick={() => onPageChange(page + 1)}
          disabled={page >= totalPages}
          className="rounded border border-gray-300 dark:border-gray-600 px-2 py-1 text-xs disabled:opacity-40 hover:bg-gray-100 dark:hover:bg-gray-800"
        >
          ›
        </button>

        <label className="ml-3 flex items-center gap-1">
          <span className="text-xs">Page size</span>
          <select
            value={pageSize}
            onChange={(e) => onPageSizeChange(Number(e.target.value))}
            className="rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 px-2 py-1 text-xs"
          >
            {pageSizeOptions.map((n) => (
              <option key={n} value={n}>
                {n}
              </option>
            ))}
          </select>
        </label>
      </div>
    </div>
  );
}

// Builds a [1, ..., current-1, current, current+1, ..., last] sequence with
// at most one ellipsis on each side. First/last pages are always shown.
function buildPageList(
  current: number,
  total: number,
): (number | "ellipsis")[] {
  if (total <= 7) {
    return Array.from({ length: total }, (_, i) => i + 1);
  }
  const out: (number | "ellipsis")[] = [1];
  const left = Math.max(2, current - 1);
  const right = Math.min(total - 1, current + 1);
  if (left > 2) out.push("ellipsis");
  for (let p = left; p <= right; p++) out.push(p);
  if (right < total - 1) out.push("ellipsis");
  out.push(total);
  return out;
}
