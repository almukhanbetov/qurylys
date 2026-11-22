import type { Metadata } from "next";
import { Suspense } from "react";
import { getI18n } from "@/lib/i18n/server";
import { api } from "@/lib/api";
import { CatalogView } from "@/features/catalog/CatalogView";
import { ProductGridSkeleton } from "@/components/ui/Skeleton";

export const dynamic = "force-dynamic";

export async function generateMetadata(): Promise<Metadata> {
  const { t } = await getI18n();
  return { title: t.catalog.title };
}

export default async function CatalogPage() {
  const { t } = await getI18n();
  // The category tree is rendered on the server; a failure goes to error.tsx (with retry).
  const categories = await api.categoryTree();

  return (
    <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6">
      {/* useSearchParams in CatalogView needs a Suspense boundary. */}
      <Suspense fallback={<ProductGridSkeleton count={8} label={t.common.loading} />}>
        <CatalogView categories={categories} />
      </Suspense>
    </div>
  );
}
