"use client";

import Link from "next/link";
import { useState } from "react";
import { moderationApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { format, pick } from "@/lib/i18n";
import { useApiQuery } from "@/hooks/useApiQuery";
import { useSubmit } from "@/hooks/useSubmit";
import { GTIN_SOURCE_KINDS, type GtinSourceKind, type ModerationReview, type ReviewDecision } from "@/types/api";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Alert, Card, Field } from "@/components/ui/Field";
import { QueryState } from "@/features/account/QueryState";
import { controlClass, GtinBadge, RequestBadge, RequestDetails, RequestEvents } from "./parts";

export function ModerationReviewView({ id }: { id: number }) {
  const { t, locale } = useI18n();
  const c = t.catalogRequests;
  const result = useApiQuery(`moderation-request:${id}`, (signal) => moderationApi.get(id, signal));

  return (
    <div className="space-y-4">
      <Link href="/account/catalog-moderation" className="text-sm font-semibold text-brand-700 hover:underline">
        {t.verification.backToList}
      </Link>
      <QueryState result={result}>
        {(r) => (
          <>
            <h1 className="text-2xl font-bold wrap-anywhere">{pick(locale, r.name_kk, r.name_ru)}</h1>
            <p className="flex flex-wrap items-center gap-2 text-sm text-ink-500">
              <RequestBadge status={r.status} />
              <GtinBadge confirmed={r.gtin_confirmed} />
              {format(t.verification.submission, { n: r.submission_no })}
              <span className="wrap-anywhere">{c.company}: {r.company_name}</span>
            </p>
            <Card title={c.edit}>
              <RequestDetails value={r} />
              {r.confirmed_source ? (
                <p className="mt-4 text-sm wrap-anywhere">
                  <span className="text-ink-500">{c.checked}: </span>
                  {r.confirmed_source_kind ? `${c.sources[r.confirmed_source_kind]}: ` : ""}
                  {r.confirmed_source}
                </p>
              ) : null}
              {r.same_gtin_requests > 0 ? (
                <div className="mt-4"><Alert tone="info">{format(c.sameGtinRequests, { count: r.same_gtin_requests })}</Alert></div>
              ) : null}
            </Card>
            <Card title={c.similar}>
              {r.similar.length === 0 ? (
                <p className="text-sm text-ink-500">{c.similarNone}</p>
              ) : (
                <ul className="divide-y divide-ink-100">
                  {r.similar.map((s) => (
                    <li key={s.id} className="flex flex-wrap items-center gap-2 py-2 text-sm">
                      <span className="min-w-0 wrap-anywhere">
                        {s.catalog_no} · {pick(locale, s.name_kk, s.name_ru)}
                        {s.brand ? ` · ${s.brand}` : ""}
                        {s.gtin ? ` · GTIN ${s.gtin}` : ""}
                      </span>
                      {s.same_gtin ? <Badge tone="danger">{c.sameGtin}</Badge> : null}
                      {s.is_demo ? <Badge tone="demo">{t.common.demo}</Badge> : null}
                    </li>
                  ))}
                </ul>
              )}
            </Card>
            {r.status === "in_review" ? (
              <>
                <ConfirmForm key={`g${r.submission_no}:${r.gtin_confirmed}`} review={r} onDone={result.retry} />
                <DecisionForm key={`d${r.submission_no}`} review={r} onDone={result.retry} />
              </>
            ) : (
              <Card title={t.verification.decision}>
                <p className="text-sm text-ink-600">{t.verification.decided}</p>
                {r.catalog_no ? <p className="mt-2 text-sm">{c.catalogNo}: <b>{r.catalog_no}</b></p> : null}
              </Card>
            )}
            <Card title={t.verification.history}><RequestEvents events={r.events} /></Card>
          </>
        )}
      </QueryState>
    </div>
  );
}

function ConfirmForm({ review, onDone }: { review: ModerationReview; onDone: () => void }) {
  const { t } = useI18n();
  const c = t.catalogRequests;
  const { pending, error, run } = useSubmit();
  const [kind, setKind] = useState<GtinSourceKind>(review.gtin_source_kind ?? "packaging");
  const [source, setSource] = useState("");
  return (
    <Card title={c.confirmTitle}>
      <form
        className="space-y-4"
        onSubmit={(e) => {
          e.preventDefault();
          void run(() => moderationApi.confirmGtin(review.id, { submission_no: review.submission_no, source_kind: kind, source })).then((ok) => ok && onDone());
        }}
      >
        <p className="text-sm text-ink-600">{c.confirmHint}</p>
        <p className="text-sm">GTIN: <span className="font-semibold">{review.gtin}</span></p>
        <div className="grid gap-4 sm:grid-cols-2">
          <label className="min-w-0 text-sm font-medium">
            {c.sourceKind}
            <select value={kind} onChange={(e) => setKind(e.target.value as GtinSourceKind)} className={controlClass}>
              {GTIN_SOURCE_KINDS.map((k) => (
                <option key={k} value={k}>{c.sources[k]}</option>
              ))}
            </select>
          </label>
          <Field label={c.checked} value={source} maxLength={500} onChange={(e) => setSource(e.target.value)} required />
        </div>
        {error ? <Alert>{error}</Alert> : null}
        <Button type="submit" variant="secondary" disabled={pending}>{c.confirmGtin}</Button>
      </form>
    </Card>
  );
}

function DecisionForm({ review, onDone }: { review: ModerationReview; onDone: () => void }) {
  const { t } = useI18n();
  const { pending, error, run } = useSubmit();
  const [comment, setComment] = useState("");
  const missing = comment.trim() === "";
  const decide = (decision: ReviewDecision) => {
    const question = decision === "approve" ? t.catalogRequests.confirmApprove : t.verification.confirm[decision];
    if (!window.confirm(question)) return;
    void run(() => moderationApi.decide(review.id, { submission_no: review.submission_no, decision, comment })).then((ok) => ok && onDone());
  };
  return (
    <Card title={t.verification.decision}>
      <div className="space-y-4">
        <label className="block">
          <span className="text-sm font-medium">{t.verification.decisionComment}</span>
          <textarea value={comment} maxLength={2000} rows={4} onChange={(e) => setComment(e.target.value)} className={`${controlClass} py-2`} />
          <span className="mt-1 block text-xs text-ink-500">{t.verification.decisionHint}</span>
        </label>
        {review.gtin_confirmed ? null : <Alert tone="info">{t.errors.gtin_unconfirmed}</Alert>}
        {error ? <Alert>{error}</Alert> : null}
        <div className="flex flex-wrap gap-3">
          <Button disabled={pending || !review.gtin_confirmed} onClick={() => decide("approve")}>{t.verification.approve}</Button>
          <Button variant="secondary" disabled={pending || missing} onClick={() => decide("request_changes")}>
            {t.verification.requestChanges}
          </Button>
          <button
            type="button"
            disabled={pending || missing}
            onClick={() => decide("reject")}
            className="min-h-11 cursor-pointer rounded-xl px-4 text-sm font-semibold text-red-700 ring-1 ring-red-200 hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {t.verification.reject}
          </button>
        </div>
      </div>
    </Card>
  );
}
