import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { cache, Suspense } from "react";
import { getI18n } from "@/lib/i18n/server";
import { format } from "@/lib/i18n";
import { api, ApiError } from "@/lib/api";
import { Breadcrumbs } from "@/features/public/Breadcrumbs";
import { BrandProducts } from "@/features/public/BrandsView";

export const dynamic = "force-dynamic";

type Props = { params: Promise<{ id: string }> };

/** 404 for unknown ids and for brands without publicly shown cards. */
const loadBrand = cache(async (rawId: string) => {
  const id = Number(rawId);
  if (!Number.isSafeInteger(id) || id < 1) notFound();
  try {
    return await api.brand(id);
  } catch (err) {
    if (err instanceof ApiError && (err.status === 404 || err.status === 400)) notFound();
    throw err;
  }
});

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const brand = await loadBrand((await params).id);
  return { title: brand.name };
}

export default async function BrandPage({ params }: Props) {
  const { t } = await getI18n();
  const brand = await loadBrand((await params).id);
  return (
    <div className="mx-auto max-w-7xl space-y-6 px-4 py-8 sm:px-6">
      <Breadcrumbs items={[{ href: "/", label: t.product.breadcrumbHome }, { href: "/brands", label: t.brandsPage.title }, { label: brand.name }]} />
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold wrap-anywhere sm:text-3xl">{brand.name}</h1>
          <p className="mt-1 text-sm text-ink-500">{format(t.brandsPage.products, { count: brand.product_count })}</p>
        </div>
        <Link href={`/catalog?brand=${brand.id}`} className="text-sm font-semibold text-brand-700 hover:underline">
          {t.brandsPage.allProducts}
        </Link>
      </div>
      <h2 className="text-lg font-bold">{t.brandsPage.productsTitle}</h2>
      <Suspense>
        <BrandProducts brandId={brand.id} />
      </Suspense>
    </div>
  );
}
