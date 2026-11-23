"use client";

import { useState, type CSSProperties } from "react";
import { useI18n } from "@/lib/i18n/client";
import { pick } from "@/lib/i18n";
import type { CategoryNode } from "@/types/api";

/** Slugs on the path from the root to the active category, inclusive. */
function activePath(nodes: CategoryNode[], active?: string): string[] {
  for (const n of nodes) {
    if (n.slug === active) return [n.slug];
    const rest = activePath(n.children, active);
    if (rest.length > 0) return [n.slug, ...rest];
  }
  return [];
}

/**
 * Category tree filter. Selecting a parent also includes all its subcategories (server-side).
 * Desktop with a mouse: subcategories fly out on hover or keyboard focus (pure CSS, no delay;
 * the flyout is a child of its <li>, so it stays open while the cursor is inside it).
 * Touch and small screens: a tap selects the category and opens its subcategories inline.
 */
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

  const [opened, setOpened] = useState(() => new Set(activePath(categories, active)));

  function select(n: CategoryNode) {
    const next = new Set(opened);
    // A second tap on the selected category collapses it.
    if (active === n.slug && next.has(n.slug)) next.delete(n.slug);
    else next.add(n.slug);
    setOpened(next);
    onSelect(n.slug);
  }

  const item = (n: CategoryNode) => (
    <li
      key={n.id}
      className="lg:any-pointer-fine:relative lg:any-pointer-fine:[&:has(:focus-visible)>ul]:block lg:any-pointer-fine:[&:hover>ul]:block"
    >
      <button
        type="button"
        onClick={() => select(n)}
        aria-current={active === n.slug ? "true" : undefined}
        style={{ "--indent": `${0.75 + n.depth * 0.9}rem` } as CSSProperties}
        className={`flex w-full cursor-pointer items-center justify-between gap-2 rounded-lg py-1.5 pl-(--indent) pr-2 text-left text-sm transition-colors lg:any-pointer-fine:pl-3 ${
          active === n.slug ? "bg-brand-700 font-semibold text-white" : "text-ink-700 hover:bg-brand-50 hover:text-brand-800"
        } ${n.depth === 0 ? "font-medium" : ""}`}
      >
        <span>{pick(locale, n.name_kk, n.name_ru)}</span>
        <span className={`text-xs ${active === n.slug ? "text-brand-100" : "text-ink-400"}`}>{n.product_count}</span>
      </button>
      {n.children.length > 0 ? (
        <ul
          className={`mt-0.5 space-y-0.5 lg:any-pointer-fine:absolute lg:any-pointer-fine:left-full lg:any-pointer-fine:top-0 lg:any-pointer-fine:z-10 lg:any-pointer-fine:mt-0 lg:any-pointer-fine:hidden lg:any-pointer-fine:w-64 lg:any-pointer-fine:rounded-2xl lg:any-pointer-fine:bg-white lg:any-pointer-fine:p-3 lg:any-pointer-fine:shadow-lg lg:any-pointer-fine:ring-1 lg:any-pointer-fine:ring-ink-100 ${
            opened.has(n.slug) ? "" : "hidden"
          }`}
        >
          {n.children.map(item)}
        </ul>
      ) : null}
    </li>
  );

  return (
    <div className="rounded-2xl bg-white p-3 ring-1 ring-ink-100">
      <p className="px-2 pb-2 pt-1 text-xs font-bold uppercase tracking-wide text-ink-500">{t.catalog.filterCategory}</p>
      <ul className="max-h-[50vh] space-y-0.5 overflow-y-auto lg:max-h-none lg:any-pointer-fine:overflow-visible">
        <li>
          <button
            type="button"
            onClick={() => onSelect(undefined)}
            aria-current={!active ? "true" : undefined}
            className={`w-full cursor-pointer rounded-lg px-3 py-1.5 text-left text-sm font-medium ${
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
