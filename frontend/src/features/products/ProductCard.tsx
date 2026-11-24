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
      className="group flex gap-4 rounded-2xl border border-ink-200 bg-white p-3 transition hover:border-brand-700 hover:shadow-md hover:shadow-brand-900/5 focus-visible:outline-2 focus-visible:outline-brand-700 sm:flex-col sm:gap-3 sm:p-4"
    >
      <div className="relative grid h-28 w-28 shrink-0 place-items-center rounded-xl bg-ink-50 text-brand-700 sm:aspect-[4/3] sm:h-auto sm:w-full">
        <Icon name={categoryIcon(product.category.slug)} className="h-12 w-12 opacity-70 sm:h-14 sm:w-14" />
        {product.is_demo ? (
          <span className="absolute left-2 top-2">
            <Badge tone="demo">{t.common.demo}</Badge>
          </span>
        ) : null}
      </div>
      <div className="flex min-w-0 flex-1 flex-col">
        <p className="text-xs font-medium text-ink-500">
          {pick(locale, product.category.name_kk, product.category.name_ru)}
        </p>
        <h3 className="mt-1 line-clamp-2 text-sm font-semibold text-ink-900 group-hover:text-brand-800">{name}</h3>
        <div className="mt-auto pt-3">
          {product.min_price ? (
            <>
              <p className="text-lg font-bold text-ink-900">
                <span className="whitespace-nowrap">{format(t.product.priceFrom, { price: formatMoney(product.min_price) })}</span>{" "}
                <span className="whitespace-nowrap text-sm font-medium text-ink-500">{format(t.product.perUnit, { unit })}</span>
              </p>
              <p className="mt-1.5">
                <Badge>{format(t.product.offersCount, { count: product.offer_count })}</Badge>
              </p>
            </>
          ) : (
            <p className="text-sm font-medium text-ink-400">{t.product.noOffers}</p>
          )}
        </div>
      </div>
    </Link>
  );
}

export function ProductGrid({ children }: { children: React.ReactNode }) {
  return <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">{children}</div>;
}
