"use client";

import { useI18n } from "@/lib/i18n/client";
import { pick } from "@/lib/i18n";
import type { CategoryNode } from "@/types/api";

/** Category tree filter. Selecting a parent also includes all its subcategories (server-side). */
export function CategoryFilter({
  categories,
  active,
  onSelect,
}: {
  categories: CategoryNode[];
  active?: string;
  onSelect: (slug: string | undefined) => void;
}) {
  const { t, locale } = useI18n();

  const item = (n: CategoryNode) => (
    <li key={n.id}>
      <button
        type="button"
        onClick={() => onSelect(n.slug)}
        aria-current={active === n.slug ? "true" : undefined}
        style={{ paddingLeft: `${0.75 + n.depth * 0.9}rem` }}
        className={`flex w-full items-center justify-between gap-2 rounded-lg py-1.5 pr-2 text-left text-sm transition-colors ${
          active === n.slug ? "bg-brand-700 font-semibold text-white" : "text-ink-700 hover:bg-brand-50 hover:text-brand-800"
        } ${n.depth === 0 ? "font-medium" : ""}`}
      >
        <span>{pick(locale, n.name_kk, n.name_ru)}</span>
        <span className={`text-xs ${active === n.slug ? "text-brand-100" : "text-ink-400"}`}>{n.product_count}</span>
      </button>
      {n.children.length > 0 ? <ul className="mt-0.5 space-y-0.5">{n.children.map(item)}</ul> : null}
    </li>
  );

  return (
    <div className="rounded-2xl bg-white p-3 ring-1 ring-ink-100">
      <p className="px-2 pb-2 pt-1 text-xs font-bold uppercase tracking-wide text-ink-500">{t.catalog.filterCategory}</p>
      <ul className="max-h-[50vh] space-y-0.5 overflow-y-auto lg:max-h-none">
        <li>
          <button
            type="button"
            onClick={() => onSelect(undefined)}
            aria-current={!active ? "true" : undefined}
            className={`w-full rounded-lg px-3 py-1.5 text-left text-sm font-medium ${
              !active ? "bg-brand-700 text-white" : "text-ink-700 hover:bg-brand-50"
            }`}
          >
            {t.catalog.allCategories}
          </button>
        </li>
        {categories.map(item)}
      </ul>
    </div>
  );
}
