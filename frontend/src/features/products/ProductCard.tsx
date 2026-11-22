import Link from "next/link";
import type { ProductListItem } from "@/types/api";
import { format, pick, type Dictionary, type Locale } from "@/lib/i18n";
import { formatMoney } from "@/lib/format";
import { Badge } from "@/components/ui/Badge";
import { Icon, categoryIcon } from "@/components/ui/Icon";

/** Pure presentational card, usable from server and client components. */
export function ProductCard({ product, locale, t }: { product: ProductListItem; locale: Locale; t: Dictionary }) {
  const name = pick(locale, product.name_kk, product.name_ru);
  const unit = t.units[product.unit];

  return (
    <Link
      href={`/products/${product.id}`}
      className="group flex flex-col rounded-2xl bg-white p-4 ring-1 ring-ink-100 transition hover:-translate-y-0.5 hover:shadow-lg hover:shadow-brand-900/5 hover:ring-brand-200"
    >
      <div className="relative grid aspect-[4/3] place-items-center rounded-xl bg-brand-50 text-brand-500">
        <Icon name={categoryIcon(product.category.slug)} className="h-14 w-14 opacity-70" />
        {product.is_demo ? (
          <span className="absolute left-2 top-2">
            <Badge tone="demo">{t.common.demo}</Badge>
          </span>
        ) : null}
      </div>
      <p className="mt-3 text-xs font-medium text-ink-500">
        {pick(locale, product.category.name_kk, product.category.name_ru)}
      </p>
      <h3 className="mt-1 line-clamp-2 text-sm font-semibold text-ink-900 group-hover:text-brand-800">{name}</h3>
      <div className="mt-auto pt-4">
        {product.min_price ? (
          <>
            <p className="text-lg font-bold text-brand-900">
              {format(t.product.priceFrom, { price: formatMoney(product.min_price) })}
              <span className="ml-1 text-sm font-medium text-ink-500">{format(t.product.perUnit, { unit })}</span>
            </p>
            <p className="mt-1 text-xs text-ink-500">{format(t.product.offersCount, { count: product.offer_count })}</p>
          </>
        ) : (
          <p className="text-sm font-medium text-ink-400">{t.product.noOffers}</p>
        )}
      </div>
    </Link>
  );
}

export function ProductGrid({ children }: { children: React.ReactNode }) {
  return <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">{children}</div>;
}
