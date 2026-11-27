"use client";

import Link from "next/link";
import { useState } from "react";
import { moderationApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { format, pick } from "@/lib/i18n";
import { formatDate } from "@/lib/format";
import { useApiQuery } from "@/hooks/useApiQuery";
import { REVIEW_STATUSES, type ReviewStatus } from "@/types/api";
import { Badge } from "@/components/ui/Badge";
import { EmptyState } from "@/components/ui/StateMessage";
import { Pagination } from "@/features/catalog/Pagination";
import { QueryState } from "@/features/account/QueryState";
import { GtinBadge, RequestBadge } from "./parts";

export function ModerationListView() {
  const { t, locale } = useI18n();
  const c = t.catalogRequests;
  const [status, setStatus] = useState<ReviewStatus | "">("in_review");
  const [page, setPage] = useState(1);
  const result = useApiQuery(`moderation:${status}:${page}`, (signal) => moderationApi.list(status, page, signal));

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">{c.moderationTitle}</h1>
      <div role="group" aria-label={t.verification.filter} className="flex flex-wrap gap-2">
        {(["", ...REVIEW_STATUSES] as const).map((s) => (
          <button
            key={s || "all"}
            type="button"
            aria-pressed={status === s}
            onClick={() => {
              setStatus(s);
              setPage(1);
            }}
            className={`min-h-11 cursor-pointer rounded-xl border px-4 text-sm font-semibold transition-colors ${
              status === s ? "border-brand-700 bg-brand-100 text-brand-900" : "border-ink-200 bg-white text-ink-700 hover:border-brand-700"
            }`}
          >
            {s ? c.status[s] : t.verification.all}
          </button>
        ))}
      </div>
      <QueryState result={result}>
        {(data) =>
          data.items.length === 0 ? (
            <EmptyState title={c.empty} />
          ) : (
            <>
              <ul className="space-y-3">
                {data.items.map((r) => (
                  <li key={r.id} className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-ink-200 bg-white p-4">
                    <div className="min-w-0 flex-1 basis-52">
                      <p className="font-semibold wrap-anywhere">{pick(locale, r.name_kk, r.name_ru) || c.unnamed}</p>
                      <p className="text-sm text-ink-500 wrap-anywhere">
                        {c.company}: {r.company_name} · GTIN: {r.gtin}
                      </p>
                      <p className="mt-1 text-sm text-ink-500">
                        {format(t.verification.submission, { n: r.submission_no })}
                        {r.submitted_at ? ` · ${format(t.verification.submittedAt, { date: formatDate(r.submitted_at, locale) })}` : ""}
                      </p>
                      <p className="mt-2 flex flex-wrap gap-2">
                        <RequestBadge status={r.status} />
                        <GtinBadge confirmed={r.gtin_confirmed} />
                        {r.conflict ? <Badge tone="muted">{t.verification.conflict}</Badge> : null}
                      </p>
                    </div>
                    {r.conflict ? null : (
                      <Link href={`/account/catalog-moderation/${r.id}`} className="inline-flex min-h-11 items-center font-semibold text-brand-700 hover:underline">
                        {t.verification.open}
                      </Link>
                    )}
                  </li>
                ))}
              </ul>
              <Pagination page={data.page} totalPages={data.total_pages} onChange={setPage} />
            </>
          )
        }
      </QueryState>
    </div>
  );
}
