"use client";

import { useI18n } from "@/lib/i18n/client";
import { pick } from "@/lib/i18n";
import type { OfferCard, OfferReason, OfferState } from "@/types/api";
import { Badge } from "@/components/ui/Badge";

const tones = { draft: "muted", published: "brand", hidden: "attention" } as const;

export function StateBadge({ state }: { state: OfferState }) {
  const { t } = useI18n();
  return <Badge tone={tones[state]}>{t.offers.state[state]}</Badge>;
}

export function Reasons({ reasons }: { reasons: OfferReason[] }) {
  const { t } = useI18n();
  if (reasons.length === 0) return <p className="text-sm text-brand-800">{t.offers.conditionsMet}</p>;
  return (
    <div className="text-sm text-ink-700">
      <p className="font-semibold">{t.offers.conditionsMissing}</p>
      <ul className="ml-5 list-disc">
        {reasons.map((r) => (
          <li key={r}>{t.offers.reasons[r]}</li>
        ))}
      </ul>
    </div>
  );
}

export function CardLine({ card }: { card: OfferCard }) {
  const { t, locale } = useI18n();
  return (
    <div className="min-w-0">
      <p className="font-semibold wrap-anywhere">{pick(locale, card.name_kk, card.name_ru)}</p>
      <p className="text-sm text-ink-500 wrap-anywhere">
        {t.catalogRequests.catalogNo}: {card.catalog_no} · GTIN: {card.gtin || "—"} · {t.offers.baseUnit}: {t.units[card.unit]}
      </p>
    </div>
  );
}
