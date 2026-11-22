"use client";

import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useI18n } from "@/lib/i18n/client";
import { format, pick } from "@/lib/i18n";
import { api, ApiError, productQueryString, type ProductQuery } from "@/lib/api";
import { useApiQuery } from "@/hooks/useApiQuery";
import { PRODUCT_SORTS, type CategoryNode, type ProductSort } from "@/types/api";
import { ProductCard, ProductGrid } from "@/features/products/ProductCard";
import { ProductGridSkeleton } from "@/components/ui/Skeleton";
import { EmptyState, ErrorState } from "@/components/ui/StateMessage";
import { ButtonLink } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { CategoryFilter } from "./CategoryFilter";
import { Pagination } from "./Pagination";

const PAGE_SIZE = 12;

function readQuery(sp: URLSearchParams): ProductQuery {
  const sort = sp.get("sort");
  const page = Number(sp.get("page"));
  return {
    q: sp.get("q")?.trim() || undefined,
    category: sp.get("category") || undefined,
    sort: PRODUCT_SORTS.includes(sort as ProductSort) ? (sort as ProductSort) : undefined,
    page: Number.isInteger(page) && page > 1 ? page : 1,
    pageSize: PAGE_SIZE,
  };
}

function findCategory(nodes: CategoryNode[], slug: string): CategoryNode | undefined {
  for (const n of nodes) {
    if (n.slug === slug) return n;
    const found = findCategory(n.children, slug);
    if (found) return found;
  }
  return undefined;
}

/**
 * The URL is the single source of truth for search, category, sort and page,
 * so results are shareable and the back button works.
 */
export function CatalogView({ categories }: { categories: CategoryNode[] }) {
  const { t, locale } = useI18n();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const query = readQuery(new URLSearchParams(searchParams.toString()));
  const key = productQueryString(query);

  const result = useApiQuery(key, (signal) => api.products(query, signal));

  function update(patch: Partial<ProductQuery>) {
    const next = { ...query, ...patch };
    // Changing filters returns to the first page.
    if (!("page" in patch)) next.page = 1;
    const qs = productQueryString({ ...next, pageSize: undefined });
    router.push(pathname + qs, { scroll: "page" in patch });
  }

  const activeCategory = query.category ? findCategory(categories, query.category) : undefined;
  const title = activeCategory ? pick(locale, activeCategory.name_kk, activeCategory.name_ru) : t.catalog.title;
  const data = result.data;
  const unknownCategory = result.status === "error" && result.error instanceof ApiError && result.error.code === "category_not_found";

  return (
    <div className="grid gap-8 lg:grid-cols-[260px_1fr]">
      <aside className="lg:sticky lg:top-36 lg:self-start">
        <CategoryFilter
          categories={categories}
          active={query.category}
          onSelect={(slug) => update({ category: slug })}
        />
      </aside>

      <section aria-labelledby="catalog-title" className="min-w-0">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h1 id="catalog-title" className="text-2xl font-bold sm:text-3xl">{title}</h1>
            <p className="mt-1 text-sm text-ink-500" aria-live="polite">
              {query.q ? `${format(t.catalog.searchResults, { q: query.q })} · ` : ""}
              {data ? format(t.catalog.found, { count: data.total }) : t.common.loading}
            </p>
          </div>
          <label className="flex items-center gap-2 text-sm">
            <span className="text-ink-500">{t.catalog.sortLabel}</span>
            <select
              value={query.sort ?? "newest"}
              onChange={(e) => update({ sort: e.target.value as ProductSort })}
              className="h-10 rounded-xl border border-ink-200 bg-white px-3 text-sm text-ink-900 focus:border-brand-500 focus:outline-none focus:ring-2 focus:ring-brand-200"
            >
              {PRODUCT_SORTS.map((s) => (
                <option key={s} value={s}>
                  {t.catalog.sort[s]}
                </option>
              ))}
            </select>
          </label>
        </div>

        {(query.q || query.category) && (
          <div className="mt-4 flex flex-wrap items-center gap-2">
            {query.q ? <Badge tone="muted">«{query.q}»</Badge> : null}
            {activeCategory ? <Badge>{pick(locale, activeCategory.name_kk, activeCategory.name_ru)}</Badge> : null}
            <Link href="/catalog" className="text-sm font-semibold text-brand-700 hover:underline">
              {t.catalog.resetFilters}
            </Link>
          </div>
        )}

        <p className="mt-4 rounded-xl bg-amber-50 px-4 py-2.5 text-xs text-amber-900 ring-1 ring-amber-100">
          {t.common.demoNotice}
        </p>

        <div className="mt-6">
          {unknownCategory ? (
            <EmptyState
              title={t.common.notFoundTitle}
              action={<ButtonLink href="/catalog" variant="secondary">{t.catalog.resetFilters}</ButtonLink>}
            />
          ) : result.status === "error" ? (
            <ErrorState onRetry={result.retry} />
          ) : !data ? (
            <ProductGridSkeleton count={8} label={t.common.loading} />
          ) : data.items.length === 0 ? (
            <EmptyState
              title={t.catalog.emptyTitle}
              text={t.catalog.emptyText}
              action={<ButtonLink href="/catalog" variant="secondary">{t.catalog.resetFilters}</ButtonLink>}
            />
          ) : (
            <div className={result.status === "loading" ? "opacity-60 transition-opacity" : ""} aria-busy={result.status === "loading"}>
              <ProductGrid>
                {data.items.map((p) => (
                  <ProductCard key={p.id} product={p} locale={locale} t={t} />
                ))}
              </ProductGrid>
              <Pagination page={data.page} totalPages={data.total_pages} onChange={(page) => update({ page })} />
            </div>
          )}
        </div>
      </section>
    </div>
  );
}
