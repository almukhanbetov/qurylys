import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { cache } from "react";
import { getI18n } from "@/lib/i18n/server";
import { format, pick } from "@/lib/i18n";
import { api, ApiError } from "@/lib/api";
import { formatMoney } from "@/lib/format";
import { Badge } from "@/components/ui/Badge";
import { Icon, categoryIcon } from "@/components/ui/Icon";
import { OffersList } from "@/features/products/OffersList";

export const dynamic = "force-dynamic";

type Props = { params: Promise<{ id: string }> };

/** Loads the product once per request; 404 for unknown or malformed ids. */
const loadProduct = cache(async (rawId: string) => {
  const id = Number(rawId);
  if (!Number.isSafeInteger(id) || id < 1) notFound();
  try {
    return await api.product(id);
  } catch (err) {
    if (err instanceof ApiError && (err.status === 404 || err.status === 400)) notFound();
    throw err; // other errors -> error.tsx with retry
  }
});

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { locale } = await getI18n();
  const product = await loadProduct((await params).id);
  return { title: pick(locale, product.name_kk, product.name_ru) };
}

export default async function ProductPage({ params }: Props) {
  const { t, locale } = await getI18n();
  const product = await loadProduct((await params).id);
  const name = pick(locale, product.name_kk, product.name_ru);
  const description = pick(locale, product.description_kk, product.description_ru);
  const unit = t.units[product.unit];

  return (
    <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6">
      <nav aria-label="breadcrumb" className="flex flex-wrap items-center gap-1 text-sm text-ink-500">
        <Link href="/" className="hover:text-brand-800">{t.product.breadcrumbHome}</Link>
        <Icon name="chevronRight" className="h-3.5 w-3.5" />
        <Link href="/catalog" className="hover:text-brand-800">{t.nav.catalog}</Link>
        {product.category_path.map((c) => (
          <span key={c.id} className="flex items-center gap-1">
            <Icon name="chevronRight" className="h-3.5 w-3.5" />
            <Link href={`/catalog?category=${c.slug}`} className="hover:text-brand-800">
              {pick(locale, c.name_kk, c.name_ru)}
            </Link>
          </span>
        ))}
      </nav>

      <div className="mt-6 grid gap-8 lg:grid-cols-[1fr_1.2fr]">
        <div className="relative grid aspect-[4/3] place-items-center rounded-3xl bg-brand-50 text-brand-500 ring-1 ring-brand-100">
          <Icon name={categoryIcon(product.category.slug)} className="h-28 w-28 opacity-70" />
          {product.is_demo ? (
            <span className="absolute left-4 top-4"><Badge tone="demo">{t.common.demo}</Badge></span>
          ) : null}
        </div>

        <div>
          <h1 className="text-2xl font-bold leading-tight sm:text-3xl">{name}</h1>
          <div className="mt-3 flex flex-wrap gap-2">
            <Badge tone="muted">
              {t.product.category}: {pick(locale, product.category.name_kk, product.category.name_ru)}
            </Badge>
            <Badge tone="muted">{t.product.unit}: {unit}</Badge>
            {product.brand ? <Badge tone="muted">{product.brand}</Badge> : null}
          </div>

          <div className="mt-6 rounded-2xl bg-white p-5 ring-1 ring-ink-100">
            {product.min_price ? (
              <>
                <p className="text-3xl font-extrabold text-brand-900">
                  {format(t.product.priceFrom, { price: formatMoney(product.min_price) })}
                  <span className="ml-1 text-base font-medium text-ink-500">{format(t.product.perUnit, { unit })}</span>
                </p>
                <p className="mt-1 text-sm text-ink-500">{format(t.product.offersCount, { count: product.offer_count })}</p>
              </>
            ) : (
              <p className="text-lg font-semibold text-ink-500">{t.product.noOffers}</p>
            )}
          </div>

          {description ? (
            <div className="mt-6">
              <h2 className="text-lg font-bold">{t.product.description}</h2>
              <p className="mt-2 text-ink-700">{description}</p>
            </div>
          ) : null}

          <div className="mt-6">
            <h2 className="text-lg font-bold">{t.product.attributes}</h2>
            {product.attributes.length === 0 ? (
              <p className="mt-2 text-sm text-ink-500">{t.product.noAttributes}</p>
            ) : (
              <dl className="mt-3 divide-y divide-ink-100 overflow-hidden rounded-2xl bg-white ring-1 ring-ink-100">
                {product.attributes.map((a) => (
                  <div key={a.key} className="grid grid-cols-2 gap-4 px-4 py-2.5 text-sm">
                    <dt className="text-ink-500">{pick(locale, a.name_kk, a.name_ru)}</dt>
                    <dd className="font-medium text-ink-900">{a.value}</dd>
                  </div>
                ))}
              </dl>
            )}
          </div>
        </div>
      </div>

      <div className="mt-10">
        <OffersList productId={product.id} unit={product.unit} />
      </div>
    </div>
  );
}
