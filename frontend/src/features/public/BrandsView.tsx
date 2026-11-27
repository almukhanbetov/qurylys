"use client";

import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { format } from "@/lib/i18n";
import { useApiQuery, type QueryResult } from "@/hooks/useApiQuery";
import type { ProductPage } from "@/types/api";
import type { Dictionary, Locale } from "@/lib/i18n";
import { ProductCard, ProductGrid } from "@/features/products/ProductCard";
import { ProductGridSkeleton, Skeleton } from "@/components/ui/Skeleton";
import { EmptyState, ErrorState } from "@/components/ui/StateMessage";
import { Pagination } from "@/features/catalog/Pagination";
import { SearchRow } from "./SearchRow";

export function BrandsView() {
  const { t } = useI18n();
  const b = t.brandsPage;
  const router = useRouter();
  const pathname = usePathname();
  const sp = useSearchParams();
  const q = sp.get("q")?.trim() ?? "";
  const pageRaw = Number(sp.get("page"));
  const page = Number.isInteger(pageRaw) && pageRaw > 1 ? pageRaw : 1;
  const result = useApiQuery(`public-brands:${q}:${page}`, (signal) => api.brands(q, page, signal));

  function go(next: { q?: string; page?: number }) {
    const p = new URLSearchParams();
    const nq = next.q ?? q;
    if (nq) p.set("q", nq);
    if (next.page && next.page > 1) p.set("page", String(next.page));
    const s = p.toString();
    router.push(pathname + (s ? `?${s}` : ""), { scroll: next.page !== undefined });
  }

  return (
    <div className="space-y-6">
      <SearchRow label={b.searchLabel} button={b.search} initial={q} onSearch={(v) => go({ q: v })} />
      {result.status === "error" ? (
        <ErrorState onRetry={result.retry} />
      ) : !result.data ? (
        <div role="status" className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <span className="sr-only">{t.common.loading}</span>
          {Array.from({ length: 8 }, (_, i) => <Skeleton key={i} className="h-20 w-full" />)}
        </div>
      ) : result.data.items.length === 0 ? (
        <EmptyState title={b.empty} text={q ? b.emptyText : undefined} />
      ) : (
        <div className={result.status === "loading" ? "opacity-60" : ""} aria-busy={result.status === "loading"}>
          <p className="mb-3 text-sm text-ink-500" aria-live="polite">{format(t.catalog.found, { count: result.data.total })}</p>
          <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            {result.data.items.map((br) => (
              <li key={br.id}>
                <Link
                  href={`/brands/${br.id}`}
                  className="block h-full rounded-2xl border border-ink-200 bg-white p-4 transition hover:border-brand-700 hover:shadow-md hover:shadow-brand-900/5"
                >
                  <span className="block font-bold text-ink-900 wrap-anywhere">{br.name}</span>
                  <span className="mt-1 block text-sm text-ink-500">{format(b.products, { count: br.product_count })}</span>
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

/** Products of one brand, from the public catalogue (same rules as /catalog). */
export function BrandProducts({ brandId }: { brandId: number }) {
  const { t, locale } = useI18n();
  const sp = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const pageRaw = Number(sp.get("page"));
  const page = Number.isInteger(pageRaw) && pageRaw > 1 ? pageRaw : 1;
  const query = { brand: brandId, page, pageSize: 12 };
  const result = useApiQuery(`brand-products:${brandId}:${page}`, (signal) => api.products(query, signal));
  return (
    <BrandProductsGrid
      result={result}
      locale={locale}
      t={t}
      onPage={(p) => router.push(pathname + (p > 1 ? `?page=${p}` : ""), { scroll: true })}
    />
  );
}

function BrandProductsGrid({
  result,
  locale,
  t,
  onPage,
}: {
  result: QueryResult<ProductPage>;
  locale: Locale;
  t: Dictionary;
  onPage: (p: number) => void;
}) {
  if (result.status === "error") return <ErrorState onRetry={result.retry} />;
  if (!result.data) return <ProductGridSkeleton count={4} label={t.common.loading} />;
  if (result.data.items.length === 0) return <EmptyState title={t.brandsPage.productsEmpty} />;
  return (
    <div className={result.status === "loading" ? "opacity-60" : ""} aria-busy={result.status === "loading"}>
      <ProductGrid>
        {result.data.items.map((p) => (
          <ProductCard key={p.id} product={p} locale={locale} t={t} />
        ))}
      </ProductGrid>
      <Pagination page={result.data.page} totalPages={result.data.total_pages} onChange={onPage} />
    </div>
  );
}
