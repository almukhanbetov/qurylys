import type { Metadata } from "next";
import { Suspense } from "react";
import { getI18n } from "@/lib/i18n/server";
import { Breadcrumbs } from "@/features/public/Breadcrumbs";
import { SuppliersView } from "@/features/public/SuppliersView";

export const dynamic = "force-dynamic";

export async function generateMetadata(): Promise<Metadata> {
  const { t } = await getI18n();
  return { title: t.suppliersPage.title };
}

export default async function SuppliersPage() {
  const { t } = await getI18n();
  return (
    <div className="mx-auto max-w-7xl space-y-6 px-4 py-8 sm:px-6">
      <Breadcrumbs items={[{ href: "/", label: t.product.breadcrumbHome }, { label: t.suppliersPage.title }]} />
      <div>
        <h1 className="text-2xl font-bold sm:text-3xl">{t.suppliersPage.title}</h1>
        <p className="mt-1 text-sm text-ink-500">{t.suppliersPage.intro}</p>
      </div>
      <Suspense>
        <SuppliersView />
      </Suspense>
    </div>
  );
}
