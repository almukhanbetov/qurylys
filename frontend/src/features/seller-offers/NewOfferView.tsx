"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { sellerOfferApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { useApiQuery } from "@/hooks/useApiQuery";
import type { OfferCard, SellerOfferInput } from "@/types/api";
import { Badge } from "@/components/ui/Badge";
import { Button, ButtonLink } from "@/components/ui/Button";
import { Alert, Card, Field } from "@/components/ui/Field";
import { EmptyState } from "@/components/ui/StateMessage";
import { NoCompany } from "@/features/account/CompanyView";
import { useActiveCompany } from "@/features/account/MeContext";
import { QueryState } from "@/features/account/QueryState";
import { OfferForm } from "./OfferForm";
import { CardLine } from "./parts";

const emptyOffer: SellerOfferInput = {
  seller_sku: "", vat_status: "", pack_size: "", stock_quantity: "0", city: "",
  retail: { price_tiyn: null, sale_mode: "piece", min_quantity: "1", step_quantity: "1" },
  wholesale: null, large_wholesale: null,
};

export function NewOfferView() {
  const { t } = useI18n();
  const router = useRouter();
  const active = useActiveCompany();
  const [card, setCard] = useState<OfferCard | null>(null);
  if (!active) return <NoCompany />;
  if (active.role !== "owner" || !active.is_supplier) {
    return <Alert>{active.is_supplier ? t.errors.forbidden : t.errors.not_supplier}</Alert>;
  }
  return (
    <div className="space-y-4">
      <Link href="/account/offers" className="text-sm font-semibold text-brand-700 hover:underline">{t.offers.title}</Link>
      <h1 className="text-2xl font-bold">{t.offers.create}</h1>
      {card ? (
        <>
          <Card>
            <CardLine card={card} />
            <Button variant="secondary" className="mt-4" onClick={() => setCard(null)}>{t.auth.back}</Button>
          </Card>
          <Card>
            <OfferForm
              initial={emptyOffer}
              unit={card.unit}
              onSave={async (f) => {
                const created = await sellerOfferApi.create(active.id, card.id, f);
                router.replace(`/account/offers/${created.id}`);
              }}
            />
          </Card>
        </>
      ) : (
        <CardPicker companyId={active.id} onPick={setCard} />
      )}
    </div>
  );
}

function CardPicker({ companyId, onPick }: { companyId: number; onPick: (c: OfferCard) => void }) {
  const { t } = useI18n();
  const [input, setInput] = useState("");
  const [q, setQ] = useState("");
  const result = useApiQuery(`offer-cards:${companyId}:${q}`, (signal) => sellerOfferApi.cards(companyId, q, signal));
  return (
    <Card title={t.offers.chooseCard}>
      <form
        className="flex flex-wrap items-end gap-3"
        onSubmit={(e) => {
          e.preventDefault();
          setQ(input.trim());
        }}
      >
        <div className="min-w-0 flex-1 basis-60">
          <Field label={t.offers.cardSearch} name="card_q" value={input} maxLength={100} onChange={(e) => setInput(e.target.value)} />
        </div>
        <Button type="submit" className="h-11">{t.header.searchButton}</Button>
      </form>
      <div className="mt-4">
        <QueryState result={result}>
          {(cards) =>
            cards.length === 0 ? (
              <EmptyState
                title={t.offers.noCards}
                action={<ButtonLink href="/account/catalog-requests" variant="secondary">{t.catalogRequests.title}</ButtonLink>}
              />
            ) : (
              <ul className="divide-y divide-ink-100">
                {cards.map((c) => (
                  <li key={c.id} className="flex flex-wrap items-center justify-between gap-3 py-3">
                    <CardLine card={c} />
                    {c.has_offer ? <Badge tone="muted">{t.offers.hasOffer}</Badge> : (
                      <Button variant="secondary" onClick={() => onPick(c)}>{t.offers.select}</Button>
                    )}
                  </li>
                ))}
              </ul>
            )
          }
        </QueryState>
      </div>
    </Card>
  );
}
