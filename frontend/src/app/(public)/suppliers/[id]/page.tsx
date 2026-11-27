import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { cache } from "react";
import { getI18n } from "@/lib/i18n/server";
import { api, ApiError } from "@/lib/api";
import { Badge } from "@/components/ui/Badge";
import { EmptyState } from "@/components/ui/StateMessage";
import { Breadcrumbs } from "@/features/public/Breadcrumbs";

export const dynamic = "force-dynamic";

type Props = { params: Promise<{ id: string }> };

/**
 * The same 404 for a missing, unverified, buyer-only, suspended or demo
 * company: a direct link discloses nothing.
 */
const loadSupplier = cache(async (rawId: string) => {
  const id = Number(rawId);
  if (!Number.isSafeInteger(id) || id < 1) notFound();
  try {
    return await api.supplier(id);
  } catch (err) {
    if (err instanceof ApiError && (err.status === 404 || err.status === 400)) notFound();
    throw err;
  }
});

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const supplier = await loadSupplier((await params).id);
  return { title: supplier.name };
}

export default async function SupplierPage({ params }: Props) {
  const { t } = await getI18n();
  const s = t.suppliersPage;
  const supplier = await loadSupplier((await params).id);
  return (
    <div className="mx-auto max-w-7xl space-y-6 px-4 py-8 sm:px-6">
      <Breadcrumbs items={[{ href: "/", label: t.product.breadcrumbHome }, { href: "/suppliers", label: s.title }, { label: supplier.name }]} />
      <section className="rounded-3xl border border-ink-200 bg-white p-6">
        <h1 className="text-2xl font-bold wrap-anywhere sm:text-3xl">{supplier.name}</h1>
        <p className="mt-2 text-ink-600">{supplier.city ?? s.cityNotSet}</p>
        {supplier.verified ? (
          <div className="mt-4 space-y-2">
            <Badge>{s.verified}</Badge>
            <p className="max-w-2xl text-sm text-ink-600">{s.verifiedNote}</p>
          </div>
        ) : null}
        <p className="mt-4 text-xs text-ink-500">{s.publicFields}</p>
      </section>
      <section className="space-y-3">
        <h2 className="text-lg font-bold">{s.assortment}</h2>
        <EmptyState title={s.noAssortmentTitle} text={s.noAssortment} />
      </section>
    </div>
  );
}
