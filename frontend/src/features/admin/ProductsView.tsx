"use client";

import Link from "next/link";
import { useState } from "react";
import { api, cardApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { format, pick } from "@/lib/i18n";
import { useApiQuery } from "@/hooks/useApiQuery";
import { CARD_STATES, type AdminCardItem, type CardFilter, type CardState } from "@/types/api";
import { Badge } from "@/components/ui/Badge";
import { ButtonLink } from "@/components/ui/Button";
import { EmptyState } from "@/components/ui/StateMessage";
import { Pagination } from "@/features/catalog/Pagination";
import { QueryState } from "@/features/account/QueryState";
import { flattenCategories } from "@/features/catalog-requests/parts";
import { BrandPicker } from "./BrandPicker";
import { SearchBar, Select } from "./parts";

export function CardBadges({ c }: { c: Pick<AdminCardItem, "is_demo" | "blocked" | "needs_review" | "gtin"> }) {
  const { t } = useI18n();
  const s = t.admin.products.states;
  return (
    <span className="flex flex-wrap gap-2">
      {c.is_demo ? <Badge tone="demo">{s.demo}</Badge> : null}
      {c.blocked ? <Badge tone="danger">{s.blocked}</Badge> : null}
      {c.is_demo ? null : c.needs_review ? <Badge tone="attention">{s.needs_review}</Badge> : <Badge>{s.reviewed}</Badge>}
      {c.gtin ? null : <Badge tone="muted">{t.admin.products.noGtin}</Badge>}
    </span>
  );
}

export function ProductsView() {
  const { t, locale } = useI18n();
  const a = t.admin;
  const [f, setF] = useState<CardFilter>({ q: "", category_id: null, brand: "", state: "", page: 1 });
  const [brandName, setBrandName] = useState<string | null>(null);
  const set = (patch: Partial<CardFilter>) => setF((prev) => ({ ...prev, page: 1, ...patch }));
  const result = useApiQuery(`admin-cards:${JSON.stringify(f)}`, (signal) => cardApi.list(f, signal));
  const tree = useApiQuery("category-tree", (signal) => api.categoryTree(signal));
  const categories = tree.data ? flattenCategories(tree.data, locale) : [];

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-bold">{a.sections.products}</h1>
        <ButtonLink href="/account/admin/products/new">{a.products.create}</ButtonLink>
      </div>
      <p className="text-sm text-ink-600">{a.products.text}</p>
      <SearchBar label={a.products.searchLabel} initial={f.q} onSearch={(q) => set({ q })}>
        <Select label={a.products.category} value={f.category_id ? String(f.category_id) : ""} onChange={(v) => set({ category_id: v ? Number(v) : null })}>
          <option value="">{a.all}</option>
          {categories.map((c) => (
            <option key={c.id} value={c.id}>{c.label}</option>
          ))}
        </Select>
        <Select label={a.products.state} value={f.state} onChange={(v) => set({ state: v as CardState | "" })}>
          <option value="">{a.all}</option>
          {CARD_STATES.map((s) => (
            <option key={s} value={s}>{a.products.states[s]}</option>
          ))}
        </Select>
        <div className="sm:col-span-2">
          <Select
            label={a.products.brand}
            value={f.brand === "none" ? "none" : f.brand ? "id" : ""}
            onChange={(v) => {
              setBrandName(null);
              set({ brand: v === "none" ? "none" : "" });
            }}
          >
            <option value="">{a.all}</option>
            <option value="none">{a.products.brandNone}</option>
            {f.brand && f.brand !== "none" ? <option value="id">{brandName ?? `#${f.brand}`}</option> : null}
          </Select>
          <div className="mt-2">
            <BrandPicker
              label={a.products.brand}
              value={f.brand && f.brand !== "none" ? Number(f.brand) : null}
              current={brandName}
              onChange={(id, name) => {
                setBrandName(name);
                set({ brand: id ? String(id) : "" });
              }}
            />
          </div>
        </div>
      </SearchBar>
      <QueryState result={result}>
        {(data) =>
          data.items.length === 0 ? (
            <EmptyState title={a.products.empty} />
          ) : (
            <>
              <p className="text-sm text-ink-500">{format(a.total, { count: data.total })}</p>
              <ul className="space-y-3">
                {data.items.map((c) => (
                  <li key={c.id} className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-ink-200 bg-white p-4">
                    <div className="min-w-0 flex-1 basis-60 space-y-1">
                      <p className="font-semibold wrap-anywhere">{pick(locale, c.name_kk, c.name_ru)}</p>
                      <p className="text-sm text-ink-500 wrap-anywhere">
                        {[c.catalog_no, c.gtin ? `GTIN ${c.gtin}` : "", pick(locale, c.category_kk, c.category_ru), c.brand ?? "", format(a.products.offers, { count: c.offer_count })]
                          .filter(Boolean)
                          .join(" · ")}
                      </p>
                      <CardBadges c={c} />
                    </div>
                    <Link href={`/account/admin/products/${c.id}`} className="inline-flex min-h-11 items-center font-semibold text-brand-700 hover:underline">
                      {a.open}
                    </Link>
                  </li>
                ))}
              </ul>
              <Pagination page={data.page} totalPages={data.total_pages} onChange={(page) => setF((prev) => ({ ...prev, page }))} />
            </>
          )
        }
      </QueryState>
    </div>
  );
}
