import type { Metadata } from "next";
import Link from "next/link";
import { getI18n } from "@/lib/i18n/server";
import { format, pick } from "@/lib/i18n";
import { api } from "@/lib/api";
import type { CategoryNode } from "@/types/api";
import { Icon, categoryIcon } from "@/components/ui/Icon";
import { EmptyState } from "@/components/ui/StateMessage";

export const dynamic = "force-dynamic";

export async function generateMetadata(): Promise<Metadata> {
  const { t } = await getI18n();
  return { title: t.categories.title };
}

export default async function CategoriesPage() {
  const { t, locale } = await getI18n();
  const tree = await api.categoryTree();

  const sub = (nodes: CategoryNode[]) => (
    <ul className="mt-2 space-y-1 border-l border-brand-100 pl-3">
      {nodes.map((n) => (
        <li key={n.id}>
          <Link
            href={`/catalog?category=${n.slug}`}
            className="flex justify-between gap-2 rounded-lg px-2 py-1 text-sm text-ink-700 hover:bg-brand-50 hover:text-brand-800"
          >
            <span>{pick(locale, n.name_kk, n.name_ru)}</span>
            <span className="text-xs text-ink-400">{n.product_count}</span>
          </Link>
          {n.children.length > 0 ? sub(n.children) : null}
        </li>
      ))}
    </ul>
  );

  return (
    <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6">
      <h1 className="text-2xl font-bold sm:text-3xl">{t.categories.title}</h1>
      <p className="mt-1 text-sm text-ink-500">{t.categories.text}</p>

      {tree.length === 0 ? (
        <div className="mt-8"><EmptyState title={t.categories.empty} /></div>
      ) : (
        <ul className="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {tree.map((root) => (
            <li key={root.id} className="rounded-2xl bg-white p-5 ring-1 ring-ink-100">
              <Link href={`/catalog?category=${root.slug}`} className="flex items-center gap-3 group">
                <span className="grid h-11 w-11 place-items-center rounded-xl bg-brand-50 text-brand-700">
                  <Icon name={categoryIcon(root.slug)} className="h-6 w-6" />
                </span>
                <span>
                  <span className="block font-semibold text-ink-900 group-hover:text-brand-800">
                    {pick(locale, root.name_kk, root.name_ru)}
                  </span>
                  <span className="text-xs text-ink-500">{format(t.home.productsCount, { count: root.product_count })}</span>
                </span>
              </Link>
              {root.children.length > 0 ? sub(root.children) : null}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
