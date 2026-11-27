"use client";

import { useI18n } from "@/lib/i18n/client";
import { format, pick } from "@/lib/i18n";
import { formatDate } from "@/lib/format";
import type { CatalogEvent, CatalogRequest, CatalogRequestStatus, CategoryNode } from "@/types/api";
import { Badge } from "@/components/ui/Badge";

const tones = { draft: "muted", in_review: "attention", needs_changes: "attention", approved: "brand", rejected: "danger" } as const;

export function RequestBadge({ status }: { status: CatalogRequestStatus }) {
  const { t } = useI18n();
  return <Badge tone={tones[status]}>{t.catalogRequests.status[status]}</Badge>;
}

export function GtinBadge({ confirmed }: { confirmed: boolean }) {
  const { t } = useI18n();
  return <Badge tone={confirmed ? "brand" : "muted"}>{confirmed ? t.catalogRequests.gtinConfirmed : t.catalogRequests.gtinDeclared}</Badge>;
}

/** Flat list for a select; indentation shows the tree. */
export function flattenCategories(nodes: CategoryNode[], locale: "kk" | "ru"): { id: number; label: string }[] {
  return nodes.flatMap((n) => [
    { id: n.id, label: "\u00a0\u00a0".repeat(n.depth) + pick(locale, n.name_kk, n.name_ru) },
    ...flattenCategories(n.children, locale),
  ]);
}

export function RequestDetails({ value: r }: { value: CatalogRequest }) {
  const { t, locale } = useI18n();
  const c = t.catalogRequests;
  const rows: [string, string][] = [
    [c.category, pick(locale, r.category_kk, r.category_ru)],
    [c.unit, r.unit ? t.units[r.unit] : ""],
    [c.nameKk, r.name_kk],
    [c.nameRu, r.name_ru],
    [c.brand, r.brand],
    [c.gtin, r.gtin ?? ""],
    [c.declared, [r.gtin_source_kind ? c.sources[r.gtin_source_kind] : "", r.gtin_source].filter(Boolean).join(": ")],
    [c.descriptionKk, r.description_kk],
    [c.descriptionRu, r.description_ru],
  ];
  return (
    <dl className="grid gap-3 text-sm sm:grid-cols-2">
      {rows.map(([label, value]) => (
        <div key={label} className="min-w-0">
          <dt className="text-ink-500">{label}</dt>
          <dd className="font-medium whitespace-pre-line wrap-anywhere">{value || t.account.notSet}</dd>
        </div>
      ))}
    </dl>
  );
}

export function RequestEvents({ events }: { events: CatalogEvent[] }) {
  const { t, locale } = useI18n();
  return (
    <ol className="space-y-4 border-l-2 border-brand-100 pl-4">
      {events.map((e) => (
        <li key={e.id} className="text-sm">
          <p className="font-semibold">
            {t.catalogRequests.actions[e.action]}
            {e.submission_no > 0 ? (
              <span className="font-normal text-ink-500"> · {format(t.verification.submission, { n: e.submission_no })}</span>
            ) : null}
          </p>
          <p className="text-ink-500 wrap-anywhere">
            {e.actor || t.catalogRequests.moderator} · {formatDate(e.created_at, locale)}
          </p>
          {e.comment ? <p className="mt-1 whitespace-pre-line rounded-lg bg-ink-50 p-3 wrap-anywhere">{e.comment}</p> : null}
        </li>
      ))}
    </ol>
  );
}

export const controlClass =
  "mt-1 min-h-11 w-full rounded-xl border border-ink-300 bg-white px-3 text-base text-ink-900 focus:border-brand-700 focus:outline-none focus:ring-2 focus:ring-brand-200 disabled:bg-ink-50";
