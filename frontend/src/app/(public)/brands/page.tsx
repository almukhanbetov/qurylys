import type { Metadata } from "next";
import { Suspense } from "react";
import { getI18n } from "@/lib/i18n/server";
import { Breadcrumbs } from "@/features/public/Breadcrumbs";
import { BrandsView } from "@/features/public/BrandsView";

export const dynamic = "force-dynamic";

export async function generateMetadata(): Promise<Metadata> {
  const { t } = await getI18n();
  return { title: t.brandsPage.title };
}

export default async function BrandsPage() {
  const { t } = await getI18n();
  return (
    <div className="mx-auto max-w-7xl space-y-6 px-4 py-8 sm:px-6">
      <Breadcrumbs items={[{ href: "/", label: t.product.breadcrumbHome }, { label: t.brandsPage.title }]} />
      <div>
        <h1 className="text-2xl font-bold sm:text-3xl">{t.brandsPage.title}</h1>
        <p className="mt-1 text-sm text-ink-500">{t.brandsPage.intro}</p>
      </div>
      {/* useSearchParams needs a Suspense boundary. */}
      <Suspense>
        <BrandsView />
      </Suspense>
    </div>
  );
}
