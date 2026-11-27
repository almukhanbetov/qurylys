"use client";

import Link from "next/link";
import { useState } from "react";
import { sellerOfferApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { formatMoney, formatQuantity } from "@/lib/format";
import { useApiQuery } from "@/hooks/useApiQuery";
import { useSubmit } from "@/hooks/useSubmit";
import { PRICE_LEVELS, type SellerOfferInput, type SellerOfferView } from "@/types/api";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Alert, Card } from "@/components/ui/Field";
import { NoCompany } from "@/features/account/CompanyView";
import { useActiveCompany } from "@/features/account/MeContext";
import { QueryState } from "@/features/account/QueryState";
import { OfferForm } from "./OfferForm";
import { CardLine, Reasons, StateBadge } from "./parts";

export function OfferView({ id }: { id: number }) {
  const { t } = useI18n();
  const active = useActiveCompany();
  if (!active) return <NoCompany />;
  return (
    <div className="space-y-4">
      <Link href="/account/offers" className="text-sm font-semibold text-brand-700 hover:underline">{t.offers.title}</Link>
      <h1 className="text-2xl font-bold">{t.offers.edit}</h1>
      <Loaded key={`${active.id}:${id}`} companyId={active.id} id={id} />
    </div>
  );
}

function toInput(o: SellerOfferView): SellerOfferInput {
  const { seller_sku, vat_status, pack_size, stock_quantity, city, retail, wholesale, large_wholesale } = o;
  return { seller_sku, vat_status, pack_size, stock_quantity, city, retail, wholesale, large_wholesale };
}

function Loaded({ companyId, id }: { companyId: number; id: number }) {
  const result = useApiQuery(`seller-offer:${companyId}:${id}`, (signal) => sellerOfferApi.get(companyId, id, signal));
  return <QueryState result={result}>{(o) => <Content key={o.version} companyId={companyId} o={o} reload={result.retry} />}</QueryState>;
}

function Content({ companyId, o, reload }: { companyId: number; o: SellerOfferView; reload: () => void }) {
  const { t } = useI18n();
  const { pending, error, run } = useSubmit();
  const [dirty, setDirty] = useState(false);
  const money = (v: number | null) => (v === null ? "—" : formatMoney({ amount_tiyn: v, currency: "KZT" }));
  const unit = t.units[o.card.unit];

  return (
    <>
      <Card>
        <CardLine card={o.card} />
        <p className="mt-3 flex flex-wrap gap-2"><StateBadge state={o.owner_state} /></p>
        <div className="mt-3"><Reasons reasons={o.reasons} /></div>
        <p className="mt-2 text-sm text-ink-600">{t.offers.notPublicYet}</p>
        {error ? <div className="mt-3"><Alert>{error}</Alert></div> : null}
        {o.can_manage ? (
          <div className="mt-4 flex flex-wrap items-center gap-3">
            {o.owner_state === "published" ? (
              <Button
                variant="secondary"
                disabled={pending || dirty}
                onClick={() => void run(() => sellerOfferApi.hide(companyId, o.id, o.version)).then((ok) => ok && reload())}
              >
                {t.offers.hide}
              </Button>
            ) : (
              <Button
                disabled={pending || dirty || o.reasons.length > 0}
                onClick={() => {
                  if (!window.confirm(t.offers.confirmPublish)) return;
                  void run(() => sellerOfferApi.publish(companyId, o.id, o.version)).then((ok) => ok && reload());
                }}
              >
                {t.offers.publish}
              </Button>
            )}
            {dirty ? <span className="text-sm text-ink-600">{t.offers.saveFirst}</span> : null}
          </div>
        ) : (
          <p className="mt-3 text-sm text-ink-500">{t.offers.readOnly}</p>
        )}
      </Card>

      <div className="grid gap-4 md:grid-cols-3">
        {PRICE_LEVELS.map((name) => {
          const l = o[name];
          return (
            <Card key={name} title={t.offers.levels[name]}>
              {l ? (
                <dl className="space-y-1 text-sm" data-view={name}>
                  <Row k={t.product.price} v={`${money(l.price_tiyn)} / ${unit}`} />
                  <Row k={t.offers.mode} v={t.offers.modes[l.sale_mode]} />
                  <Row k={t.product.moq} v={`${formatQuantity(l.min_quantity)} ${unit}`} />
                  <Row k={t.offers.packPrice} v={money(l.pack_price_tiyn)} />
                  <Row k={t.offers.minTotal} v={money(l.min_total_tiyn)} />
                  <dd className="pt-1">
                    <Badge tone={l.in_stock ? "brand" : "danger"}>{l.in_stock ? t.offers.inStock : t.offers.outOfStock}</Badge>
                  </dd>
                </dl>
              ) : (
                <p className="text-sm text-ink-500">{t.offers.none}</p>
              )}
            </Card>
          );
        })}
      </div>

      <Card>
        <OfferForm
          initial={toInput(o)}
          unit={o.card.unit}
          disabled={!o.can_manage}
          onDirty={setDirty}
          onSave={async (f) => {
            await sellerOfferApi.update(companyId, o.id, o.version, f);
            reload();
          }}
        />
      </Card>
    </>
  );
}

function Row({ k, v }: { k: string; v: string }) {
  return (
    <div className="flex flex-wrap justify-between gap-x-3">
      <dt className="text-ink-500">{k}</dt>
      <dd className="font-medium wrap-anywhere">{v}</dd>
    </div>
  );
}
