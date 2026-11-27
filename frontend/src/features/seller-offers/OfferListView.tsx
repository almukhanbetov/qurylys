"use client";

import Link from "next/link";
import { sellerOfferApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { formatMoney, formatQuantity } from "@/lib/format";
import { useApiQuery } from "@/hooks/useApiQuery";
import { PRICE_LEVELS } from "@/types/api";
import { ButtonLink } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Field";
import { EmptyState } from "@/components/ui/StateMessage";
import { NoCompany } from "@/features/account/CompanyView";
import { useActiveCompany } from "@/features/account/MeContext";
import { QueryState } from "@/features/account/QueryState";
import { CardLine, Reasons, StateBadge } from "./parts";

export function OfferListView() {
  const { t } = useI18n();
  const active = useActiveCompany();
  if (!active) return <NoCompany />;
  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">{t.offers.title}</h1>
      <List key={active.id} id={active.id} />
    </div>
  );
}

function List({ id }: { id: number }) {
  const { t } = useI18n();
  const result = useApiQuery(`seller-offers:${id}`, (signal) => sellerOfferApi.list(id, signal));
  return (
    <QueryState result={result}>
      {(data) => (
        <>
          <Alert tone="info">{t.offers.notPublicYet}</Alert>
          {data.can_create ? <ButtonLink href="/account/offers/new">{t.offers.create}</ButtonLink> : null}
          {data.can_manage ? null : <p className="text-sm text-ink-600">{t.offers.readOnly}</p>}
          {data.items.length === 0 ? (
            <EmptyState title={t.offers.empty} />
          ) : (
            <ul className="space-y-3">
              {data.items.map((o) => (
                <li key={o.id}>
                  <Link
                    href={`/account/offers/${o.id}`}
                    className="block space-y-3 rounded-2xl border border-ink-200 bg-white p-4 transition-colors hover:border-brand-700"
                  >
                    <CardLine card={o.card} />
                    <p className="flex flex-wrap items-center gap-2 text-sm text-ink-600">
                      <StateBadge state={o.owner_state} />
                      <span className="wrap-anywhere">{t.offers.sellerSku}: {o.seller_sku || "—"}</span>
                    </p>
                    <p className="flex flex-wrap gap-x-4 gap-y-1 text-sm">
                      {PRICE_LEVELS.map((l) => {
                        const price = o[l]?.price_tiyn;
                        return price ? (
                          <span key={l}>
                            {t.offers.levels[l]}: <b>{formatMoney({ amount_tiyn: price, currency: "KZT" })}</b>
                          </span>
                        ) : null;
                      })}
                      <span>{t.product.stock}: {formatQuantity(o.stock_quantity)} {t.units[o.card.unit]}</span>
                    </p>
                    <Reasons reasons={o.reasons} />
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </>
      )}
    </QueryState>
  );
}
