"use client";

import { useI18n } from "@/lib/i18n/client";
import { format, pick } from "@/lib/i18n";
import { api } from "@/lib/api";
import { formatMoney, formatQuantity } from "@/lib/format";
import { useApiQuery } from "@/hooks/useApiQuery";
import type { Offer, Unit } from "@/types/api";
import { Badge } from "@/components/ui/Badge";
import { Icon } from "@/components/ui/Icon";
import { Skeleton } from "@/components/ui/Skeleton";
import { EmptyState, ErrorState } from "@/components/ui/StateMessage";
import { UnavailableButton } from "@/components/ui/Button";

export function OffersList({ productId, unit }: { productId: number; unit: Unit }) {
  const { t } = useI18n();
  const result = useApiQuery(`offers:${productId}`, (signal) => api.offers(productId, signal));

  return (
    <section aria-labelledby="offers-title" className="rounded-2xl bg-white p-5 ring-1 ring-ink-100 sm:p-6">
      <h2 id="offers-title" className="text-xl font-bold">{t.product.offersTitle}</h2>
      <p className="mt-1 text-sm text-ink-500">{t.product.offersText}</p>

      <div className="mt-5">
        {result.status === "error" ? (
          <ErrorState onRetry={result.retry} />
        ) : !result.data ? (
          <div role="status" className="space-y-3">
            <span className="sr-only">{t.common.loading}</span>
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} className="h-24 w-full" />
            ))}
          </div>
        ) : result.data.length === 0 ? (
          <EmptyState title={t.product.offersEmpty} />
        ) : (
          <ul className="space-y-3">
            {result.data.map((o) => (
              <OfferRow key={o.id} offer={o} unit={unit} />
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}

function OfferRow({ offer: o, unit }: { offer: Offer; unit: Unit }) {
  const { t, locale } = useI18n();
  const unitLabel = t.units[unit];

  let deliveryText = t.product.pickupOnly;
  if (o.delivery_available) {
    if (o.delivery_days_min != null && o.delivery_days_max != null) {
      deliveryText =
        o.delivery_days_min === o.delivery_days_max
          ? format(t.product.deliveryDaysSame, { min: o.delivery_days_min })
          : format(t.product.deliveryDays, { min: o.delivery_days_min, max: o.delivery_days_max });
    } else {
      deliveryText = t.product.deliveryYes;
    }
  }

  return (
    <li
      className={`grid gap-4 rounded-xl p-4 ring-1 sm:grid-cols-[1.4fr_1fr_1fr_auto] sm:items-center ${
        o.is_available ? "bg-brand-50/50 ring-brand-100" : "bg-ink-50 ring-ink-100"
      }`}
    >
      <div className="min-w-0">
        <p className="text-xs text-ink-500">{t.product.supplier}</p>
        <p className="flex flex-wrap items-center gap-2 font-semibold text-ink-900">
          {o.supplier.name}
          {o.is_demo || o.supplier.is_demo ? <Badge tone="demo">{t.common.demo}</Badge> : null}
        </p>
        {o.city ? (
          <p className="mt-0.5 flex items-center gap-1 text-xs text-ink-500">
            <Icon name="pin" className="h-3.5 w-3.5" />
            {o.city}
          </p>
        ) : null}
      </div>

      <div>
        <p className="text-xs text-ink-500">{t.product.price}</p>
        <p className={`text-lg font-bold ${o.is_available ? "text-brand-900" : "text-ink-500"}`}>
          {formatMoney(o.price)}
          <span className="ml-1 text-sm font-medium text-ink-500">{format(t.product.perUnit, { unit: unitLabel })}</span>
        </p>
        <p className="mt-0.5">
          {o.is_available ? (
            <Badge>
              <Icon name="check" className="h-3 w-3" />
              {t.product.inStock}
            </Badge>
          ) : (
            <Badge tone="danger">{t.product.outOfStock}</Badge>
          )}
        </p>
      </div>

      <dl className="grid grid-cols-3 gap-2 text-sm sm:grid-cols-1 sm:gap-1">
        <div className="sm:flex sm:gap-1">
          <dt className="text-xs text-ink-500 sm:text-sm">{t.product.stock}:</dt>
          <dd className="font-medium">{formatQuantity(o.stock_quantity)} {unitLabel}</dd>
        </div>
        <div className="sm:flex sm:gap-1">
          <dt className="text-xs text-ink-500 sm:text-sm">{t.product.moq}:</dt>
          <dd className="font-medium">{formatQuantity(o.min_order_quantity)} {unitLabel}</dd>
        </div>
        <div className="sm:flex sm:gap-1">
          <dt className="text-xs text-ink-500 sm:text-sm">{t.product.delivery}:</dt>
          <dd className="font-medium">
            {deliveryText}
            {pick(locale, o.delivery_terms_kk, o.delivery_terms_ru) ? (
              <span className="block text-xs font-normal text-ink-500">
                {pick(locale, o.delivery_terms_kk, o.delivery_terms_ru)}
              </span>
            ) : null}
          </dd>
        </div>
      </dl>

      <UnavailableButton soonLabel={t.common.soon} title={t.product.addToCartSoon} className="w-full sm:w-auto">
        {t.product.addToCart}
      </UnavailableButton>
    </li>
  );
}
