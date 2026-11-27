"use client";

import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { format } from "@/lib/i18n";
import { useApiQuery } from "@/hooks/useApiQuery";
import { Badge } from "@/components/ui/Badge";
import { Skeleton } from "@/components/ui/Skeleton";
import { EmptyState, ErrorState } from "@/components/ui/StateMessage";
import { Pagination } from "@/features/catalog/Pagination";
import { SearchRow } from "./SearchRow";

export function SuppliersView() {
  const { t } = useI18n();
  const s = t.suppliersPage;
  const router = useRouter();
  const pathname = usePathname();
  const sp = useSearchParams();
  const q = sp.get("q")?.trim() ?? "";
  const city = sp.get("city") ?? "";
  const pageRaw = Number(sp.get("page"));
  const page = Number.isInteger(pageRaw) && pageRaw > 1 ? pageRaw : 1;
  const result = useApiQuery(`public-suppliers:${q}:${city}:${page}`, (signal) => api.suppliers(q, city, page, signal));

  function go(next: { q?: string; city?: string; page?: number }) {
    const p = new URLSearchParams();
    const nq = next.q ?? q;
    const nc = next.city ?? city;
    if (nq) p.set("q", nq);
    if (nc) p.set("city", nc);
    if (next.page && next.page > 1) p.set("page", String(next.page));
    const str = p.toString();
    router.push(pathname + (str ? `?${str}` : ""), { scroll: next.page !== undefined });
  }

  const cities = result.data?.cities ?? [];
  return (
    <div className="space-y-6">
      <SearchRow label={s.searchLabel} button={s.search} initial={q} onSearch={(v) => go({ q: v })}>
        <label className="block text-sm font-medium text-ink-900 sm:w-56">
          {s.city}
          <select
            value={city}
            onChange={(e) => go({ city: e.target.value })}
            className="mt-1 h-11 w-full rounded-xl border border-ink-300 bg-white px-3 text-base text-ink-900 focus:border-brand-700 focus:outline-none focus:ring-2 focus:ring-brand-200"
          >
            <option value="">{s.allCities}</option>
            {city && !cities.includes(city) ? <option value={city}>{city}</option> : null}
            {cities.map((c) => (
              <option key={c} value={c}>{c}</option>
            ))}
          </select>
        </label>
      </SearchRow>
      <p className="rounded-xl bg-brand-50 px-4 py-2.5 text-xs text-brand-900 ring-1 ring-brand-100">{s.verifiedNote}</p>
      {result.status === "error" ? (
        <ErrorState onRetry={result.retry} />
      ) : !result.data ? (
        <div role="status" className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          <span className="sr-only">{t.common.loading}</span>
          {Array.from({ length: 6 }, (_, i) => <Skeleton key={i} className="h-24 w-full" />)}
        </div>
      ) : result.data.items.length === 0 ? (
        <EmptyState title={s.empty} text={q || city ? s.emptyText : undefined} />
      ) : (
        <div className={result.status === "loading" ? "opacity-60" : ""} aria-busy={result.status === "loading"}>
          <p className="mb-3 text-sm text-ink-500" aria-live="polite">{format(t.catalog.found, { count: result.data.total })}</p>
          <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {result.data.items.map((x) => (
              <li key={x.id}>
                <Link
                  href={`/suppliers/${x.id}`}
                  className="block h-full rounded-2xl border border-ink-200 bg-white p-4 transition hover:border-brand-700 hover:shadow-md hover:shadow-brand-900/5"
                >
                  <span className="block font-bold text-ink-900 wrap-anywhere">{x.name}</span>
                  <span className="mt-1 block text-sm text-ink-500">{x.city ?? s.cityNotSet}</span>
                  {x.verified ? <span className="mt-2 inline-block"><Badge>{s.verified}</Badge></span> : null}
                </Link>
              </li>
            ))}
          </ul>
          <Pagination page={result.data.page} totalPages={result.data.total_pages} onChange={(p) => go({ page: p })} />
        </div>
      )}
    </div>
  );
}
