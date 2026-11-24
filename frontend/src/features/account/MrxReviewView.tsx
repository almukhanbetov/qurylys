"use client";

import Link from "next/link";
import { useState } from "react";
import { mrxApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { format } from "@/lib/i18n";
import { formatDate } from "@/lib/format";
import { useApiQuery } from "@/hooks/useApiQuery";
import { useSubmit } from "@/hooks/useSubmit";
import type { Review, ReviewDecision } from "@/types/api";
import { Button } from "@/components/ui/Button";
import { Alert, Card } from "@/components/ui/Field";
import { QueryState } from "./QueryState";
import { DocumentList, EventList, RequisitesList, VerificationBadge } from "./VerificationParts";

export function MrxReviewView({ id }: { id: number }) {
  const { t, locale } = useI18n();
  const result = useApiQuery(`mrx-request:${id}`, (signal) => mrxApi.get(id, signal));

  return (
    <div className="space-y-4">
      <Link href="/account/mrx" className="text-sm font-semibold text-brand-700 hover:underline">
        {t.verification.backToList}
      </Link>
      <QueryState result={result}>
        {(r) => (
          <>
            <h1 className="text-2xl font-bold wrap-anywhere">{r.requisites.name}</h1>
            <p className="flex flex-wrap items-center gap-2 text-sm text-ink-500">
              <VerificationBadge status={r.status} />
              {format(t.verification.submission, { n: r.submission_no })}
              {r.submitted_at ? <span>{format(t.verification.submittedAt, { date: formatDate(r.submitted_at, locale) })}</span> : null}
              {r.decided_at ? <span>{format(t.verification.decidedAt, { date: formatDate(r.decided_at, locale) })}</span> : null}
            </p>
            <Card title={t.verification.requisites}>
              <RequisitesList value={r.requisites} />
            </Card>
            <Card title={t.verification.documents}>
              <p className="mb-2 text-xs text-ink-500">{t.verification.uploadedFileWarning}</p>
              <DocumentList documents={r.documents} url={(doc) => mrxApi.documentUrl(id, doc)} />
            </Card>
            <Card title={t.verification.decision}>
              {r.status === "in_review" ? (
                <DecisionForm key={r.submission_no} review={r} onDone={result.retry} />
              ) : (
                <p className="text-sm text-ink-600">{t.verification.decided}</p>
              )}
            </Card>
            <Card title={t.verification.history}>
              <EventList events={r.events} />
            </Card>
          </>
        )}
      </QueryState>
    </div>
  );
}

function DecisionForm({ review, onDone }: { review: Review; onDone: () => void }) {
  const { t } = useI18n();
  const { pending, error, run } = useSubmit();
  const [comment, setComment] = useState("");
  const decide = (decision: ReviewDecision) => {
    if (!window.confirm(t.verification.confirm[decision])) return;
    void run(() => mrxApi.decide(review.id, review.submission_no, decision, comment)).then((ok) => ok && onDone());
  };
  const missing = comment.trim() === "";

  return (
    <div className="space-y-4">
      <label className="block">
        <span className="text-sm font-medium">{t.verification.decisionComment}</span>
        <textarea
          value={comment}
          maxLength={2000}
          rows={4}
          onChange={(e) => setComment(e.target.value)}
          className="mt-1 w-full rounded-xl border border-ink-300 bg-white p-3 text-base focus:border-brand-700 focus:outline-none focus:ring-2 focus:ring-brand-200"
        />
        <span className="mt-1 block text-xs text-ink-500">{t.verification.decisionHint}</span>
      </label>
      {error ? <Alert>{error}</Alert> : null}
      <div className="flex flex-wrap gap-3">
        <Button disabled={pending} onClick={() => decide("approve")}>{t.verification.approve}</Button>
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
  );
}
