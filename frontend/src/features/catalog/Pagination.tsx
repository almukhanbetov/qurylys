"use client";

import { useI18n } from "@/lib/i18n/client";
import { format } from "@/lib/i18n";
import { Icon } from "@/components/ui/Icon";

export function Pagination({
  page,
  totalPages,
  onChange,
}: {
  page: number;
  totalPages: number;
  onChange: (page: number) => void;
}) {
  const { t } = useI18n();
  if (totalPages <= 1) return null;

  const btn =
    "inline-flex h-10 items-center gap-1 rounded-xl px-4 text-sm font-semibold ring-1 ring-ink-200 bg-white text-brand-800 hover:bg-brand-50 disabled:cursor-not-allowed disabled:text-ink-300 disabled:hover:bg-white";

  return (
    <nav aria-label={t.catalog.pagination} className="mt-8 flex items-center justify-center gap-3">
      <button type="button" className={btn} disabled={page <= 1} onClick={() => onChange(page - 1)}>
        <Icon name="chevronLeft" className="h-4 w-4" />
        {t.catalog.prev}
      </button>
      <span className="text-sm text-ink-600">{format(t.catalog.pageOf, { page, total: totalPages })}</span>
      <button type="button" className={btn} disabled={page >= totalPages} onClick={() => onChange(page + 1)}>
        {t.catalog.next}
        <Icon name="chevronRight" className="h-4 w-4" />
      </button>
    </nav>
  );
}
