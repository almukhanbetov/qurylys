"use client";

import Link from "next/link";
import { useState } from "react";
import { cardApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { format, pick } from "@/lib/i18n";
import { formatDate } from "@/lib/format";
import { useApiQuery } from "@/hooks/useApiQuery";
import { useSubmit } from "@/hooks/useSubmit";
import type { AdminCard, CardEvent, CardFields, GtinConfirmation } from "@/types/api";
import { Button } from "@/components/ui/Button";
import { Alert, Card, Field } from "@/components/ui/Field";
import { QueryState } from "@/features/account/QueryState";
import { CardFieldsForm, GtinFields, TextArea } from "./CardForm";
import { CardBadges } from "./ProductsView";

export function CardView({ id }: { id: number }) {
  const { t, locale } = useI18n();
  const a = t.admin;
  const result = useApiQuery(`admin-card:${id}`, (signal) => cardApi.get(id, signal));

  return (
    <div className="space-y-4">
      <Link href="/account/admin/products" className="text-sm font-semibold text-brand-700 hover:underline">
        {a.back}
      </Link>
      <QueryState result={result}>
        {(c) => {
          const readOnly = c.is_demo || c.conflict;
          return (
            <>
              <h1 className="text-2xl font-bold wrap-anywhere">{pick(locale, c.name_kk, c.name_ru)}</h1>
              <p className="flex flex-wrap items-center gap-2 text-sm text-ink-500">
                <span>{a.card.catalogNo}: <span className="font-semibold text-ink-900">{c.catalog_no}</span></span>
                <CardBadges c={c} />
              </p>
              {c.is_demo ? <Alert tone="info">{a.card.demo}</Alert> : null}
              {c.conflict ? <Alert tone="info">{a.card.conflict}</Alert> : null}
              {c.blocked && c.blocked_at ? (
                <Alert>{format(a.card.blockedSince, { date: formatDate(c.blocked_at, locale), reason: c.block_reason ?? "" })}</Alert>
              ) : null}
              <Card title={a.card.offers}>
                <p className="text-sm">{format(a.card.offerStats, c.offer_stats)}</p>
                <p className="mt-1 text-xs text-ink-500">{a.card.offerNote}</p>
                {c.request_id ? <p className="mt-1 text-xs text-ink-500">{format(a.card.fromRequest, { id: c.request_id })}</p> : null}
              </Card>
              {/* Keyed by version: after any change the form starts from the saved card. */}
              <EditCard key={`edit:${c.version}`} card={c} readOnly={readOnly} onSaved={result.retry} />
              <GtinCard key={`gtin:${c.version}`} card={c} readOnly={readOnly} onSaved={result.retry} />
              <BlockCard key={`block:${c.version}`} card={c} readOnly={readOnly} onSaved={result.retry} />
              {c.attributes.length > 0 ? (
                <Card title={a.card.attributes}>
                  <dl className="grid gap-2 text-sm sm:grid-cols-2">
                    {c.attributes.map((x) => (
                      <div key={x.key}>
                        <dt className="text-ink-500">{pick(locale, x.name_kk, x.name_ru)}</dt>
                        <dd className="font-medium">{x.value}</dd>
                      </div>
                    ))}
                  </dl>
                  <p className="mt-3 text-xs text-ink-500">{a.card.attributesNote}</p>
                </Card>
              ) : null}
              <Card title={a.card.events}>
                <Events events={c.events} />
              </Card>
            </>
          );
        }}
      </QueryState>
    </div>
  );
}

function fieldsOf(c: AdminCard): CardFields {
  return {
    category_id: c.category_id,
    name_kk: c.name_kk,
    name_ru: c.name_ru,
    description_kk: c.description_kk,
    description_ru: c.description_ru,
    brand_id: c.brand_id,
    unit: c.unit,
  };
}

type Props = { card: AdminCard; readOnly: boolean; onSaved: () => void };

function EditCard({ card: c, readOnly, onSaved }: Props) {
  const { t } = useI18n();
  const a = t.admin;
  const [f, setF] = useState(fieldsOf(c));
  const [brandName, setBrandName] = useState(c.brand);
  const [reason, setReason] = useState("");
  const { pending, error, run } = useSubmit();
  const linked = c.offer_count > 0;

  return (
    <Card title={a.card.fields}>
      {linked && !readOnly ? <div className="mb-4"><Alert tone="info">{a.card.linked}</Alert></div> : null}
      <form
        className="space-y-4"
        onSubmit={(e) => {
          e.preventDefault();
          void run(() => cardApi.update(c.id, c.version, f, reason)).then((ok) => ok && onSaved());
        }}
      >
        <CardFieldsForm
          value={f}
          brandName={brandName}
          locked={linked}
          disabled={readOnly || pending}
          onChange={(next, name) => {
            setF(next);
            if (name !== undefined) setBrandName(name);
          }}
        />
        {readOnly ? null : (
          <>
            <TextArea label={a.card.reason} value={reason} rows={2} required onChange={setReason}>
              <span className="mt-1 block text-xs text-ink-500">{a.card.reasonHint}</span>
            </TextArea>
            {error ? <Alert>{error}</Alert> : null}
            <Button type="submit" disabled={pending}>{pending ? t.common.loading : a.save}</Button>
          </>
        )}
      </form>
    </Card>
  );
}

function GtinCard({ card: c, readOnly, onSaved }: Props) {
  const { t, locale } = useI18n();
  const a = t.admin;
  const r = t.catalogRequests;
  const [g, setG] = useState<GtinConfirmation>({ gtin: c.gtin ?? "", source_kind: c.gtin_source_kind ?? "manufacturer", source: c.gtin_source ?? "" });
  const [reason, setReason] = useState("");
  const { pending, error, run } = useSubmit();
  const locked = c.offer_count > 0 && c.gtin !== null;

  return (
    <Card title={a.card.gtinTitle}>
      {c.gtin ? (
        <dl className="grid gap-2 text-sm sm:grid-cols-2">
          <div>
            <dt className="text-ink-500">{a.card.gtinCurrent}</dt>
            <dd className="font-semibold">{c.gtin}</dd>
          </div>
          <div className="min-w-0">
            <dt className="text-ink-500">{a.card.gtinSource}</dt>
            <dd className="wrap-anywhere">
              {c.gtin_source_kind ? `${r.sources[c.gtin_source_kind]}: ` : ""}
              {c.gtin_source}
              {c.gtin_confirmed_at ? ` · ${formatDate(c.gtin_confirmed_at, locale)}` : ""}
            </dd>
          </div>
        </dl>
      ) : (
        <Alert tone="info">{a.card.gtinNone}</Alert>
      )}
      {readOnly ? null : locked ? (
        <p className="mt-3 text-sm text-ink-500">{a.card.gtinLocked}</p>
      ) : (
        <form
          className="mt-4 space-y-3"
          onSubmit={(e) => {
            e.preventDefault();
            void run(() => cardApi.confirmGtin(c.id, c.version, g, reason)).then((ok) => ok && onSaved());
          }}
        >
          <GtinFields value={g} onChange={setG} gtinLabel={c.gtin ? a.card.gtinNew : undefined} />
          <Field label={a.card.reason} value={reason} maxLength={2000} onChange={(e) => setReason(e.target.value)} />
          {error ? <Alert>{error}</Alert> : null}
          <Button type="submit" disabled={pending}>{a.card.gtinConfirm}</Button>
        </form>
      )}
    </Card>
  );
}

function BlockCard({ card: c, readOnly, onSaved }: Props) {
  const { t } = useI18n();
  const a = t.admin;
  const [reason, setReason] = useState("");
  const { pending, error, run } = useSubmit();
  if (readOnly) return null;
  return (
    <Card title={a.card.blockTitle}>
      <p className="mb-3 text-sm text-ink-600">{a.card.blockText}</p>
      <form
        className="space-y-3"
        onSubmit={(e) => {
          e.preventDefault();
          void run(() => (c.blocked ? cardApi.unblock(c.id, c.version, reason) : cardApi.block(c.id, c.version, reason))).then(
            (ok) => ok && onSaved(),
          );
        }}
      >
        <TextArea label={a.card.reason} value={reason} rows={2} required={!c.blocked} onChange={setReason} />
        {error ? <Alert>{error}</Alert> : null}
        <Button type="submit" variant={c.blocked ? "secondary" : "primary"} disabled={pending} className={c.blocked ? "" : "bg-red-700 hover:bg-red-800"}>
          {c.blocked ? a.card.unblock : a.card.block}
        </Button>
      </form>
    </Card>
  );
}

function show(v: unknown): string {
  if (v === null || v === undefined || v === "" || v === 0) return "—";
  return typeof v === "string" ? v : JSON.stringify(v);
}

function Events({ events }: { events: CardEvent[] }) {
  const { t, locale } = useI18n();
  const a = t.admin.card;
  if (events.length === 0) return <p className="text-sm text-ink-500">{a.noEvents}</p>;
  return (
    <ol className="space-y-4 border-l-2 border-brand-100 pl-4">
      {events.map((e) => {
        const d = e.details ?? {};
        const kind = d.kind as keyof typeof a.kinds | undefined;
        const changed = Object.keys(d.new ?? {}).filter((k) => k !== "brand_id" || !("brand" in (d.new ?? {})));
        return (
          <li key={e.id} className="text-sm">
            <p className="font-semibold">
              {a.actions[e.action as keyof typeof a.actions] ?? t.catalogRequests.actions[e.action as keyof typeof t.catalogRequests.actions] ?? e.action}
              {kind && a.kinds[kind] ? <span className="font-normal text-ink-500"> · {a.kinds[kind]}</span> : null}
            </p>
            <p className="text-ink-500">
              {formatDate(e.created_at, locale)} · {e.actor}
            </p>
            {e.comment ? <p className="mt-1 whitespace-pre-line wrap-anywhere">{e.comment}</p> : null}
            {e.action === "card_edited" || e.action === "card_gtin_confirmed" ? (
              <ul className="mt-1 space-y-0.5 text-xs text-ink-600">
                {changed.map((k) => (
                  <li key={k} className="wrap-anywhere">
                    {k}: {show(d.old?.[k])} → {show(d.new?.[k])}
                  </li>
                ))}
              </ul>
            ) : null}
          </li>
        );
      })}
    </ol>
  );
}
