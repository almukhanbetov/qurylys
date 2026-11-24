"use client";

import { useI18n } from "@/lib/i18n/client";
import { format } from "@/lib/i18n";
import { formatDate } from "@/lib/format";
import type { CompanyDetails, VerificationDocument, VerificationEvent, VerificationStatus } from "@/types/api";
import { Badge } from "@/components/ui/Badge";
import { Icon } from "@/components/ui/Icon";

const tones = {
  none: "muted",
  draft: "muted",
  in_review: "attention",
  needs_changes: "attention",
  approved: "brand",
  rejected: "danger",
  reverification: "attention",
} as const;

export function VerificationBadge({ status }: { status: VerificationStatus }) {
  const { t } = useI18n();
  return (
    <Badge tone={tones[status]}>
      {status === "approved" ? <Icon name="check" className="h-3 w-3" /> : null}
      {t.verification.status[status]}
    </Badge>
  );
}

function useFieldLabels(): Record<string, string> {
  const { t } = useI18n();
  const a = t.account;
  return {
    name: a.name, bin: a.bin, city: a.city, legal_address: a.legalAddress, actual_address: a.actualAddress,
    work_phone: a.workPhone, representative_name: a.representative, is_buyer: a.buyer, is_supplier: a.supplier,
  };
}

export function RequisitesList({ value }: { value: CompanyDetails }) {
  const { t } = useI18n();
  const labels = useFieldLabels();
  const fields = ["name", "bin", "city", "work_phone", "legal_address", "actual_address", "representative_name"] as const;
  const roles = [value.is_buyer ? t.account.buyer : "", value.is_supplier ? t.account.supplier : ""].filter(Boolean);
  return (
    <dl className="grid gap-3 text-sm sm:grid-cols-2">
      {fields.map((f) => (
        <div key={f} className="min-w-0">
          <dt className="text-ink-500">{labels[f]}</dt>
          <dd className="font-medium wrap-anywhere">{value[f] || t.account.notSet}</dd>
        </div>
      ))}
      <div>
        <dt className="text-ink-500">{t.account.companyType}</dt>
        <dd className="font-medium">{roles.join(", ")}</dd>
      </div>
    </dl>
  );
}

export function DocumentList({
  documents,
  url,
  onRemove,
  disabled = false,
}: {
  documents: VerificationDocument[];
  url: (id: number) => string;
  onRemove?: (id: number) => void;
  disabled?: boolean;
}) {
  const { t } = useI18n();
  if (documents.length === 0) return <p className="text-sm text-ink-500">{t.verification.noDocuments}</p>;
  return (
    <ul className="divide-y divide-ink-100">
      {documents.map((d) => (
        <li key={d.id} className="flex flex-wrap items-center justify-between gap-2 py-3">
          <div className="min-w-0 flex-1 basis-48 text-sm">
            <p className="font-medium wrap-anywhere">{d.filename}</p>
            <p className="text-ink-500">
              {t.verification.kinds[d.kind]} · {Math.max(1, Math.round(d.size / 1024))} KB
            </p>
          </div>
          <div className="flex gap-4 text-sm font-semibold">
            <a href={url(d.id)} download className="inline-flex min-h-11 items-center text-brand-700 hover:underline">
              {t.verification.download}
            </a>
            {onRemove ? (
              <button
                type="button"
                disabled={disabled}
                onClick={() => {
                  if (window.confirm(t.verification.confirmRemove)) onRemove(d.id);
                }}
                className="min-h-11 cursor-pointer text-red-700 hover:underline disabled:opacity-60"
              >
                {t.verification.remove}
              </button>
            ) : null}
          </div>
        </li>
      ))}
    </ul>
  );
}

export function EventList({ events }: { events: VerificationEvent[] }) {
  const { t, locale } = useI18n();
  const labels = useFieldLabels();
  const flag = (v: string) => (v === "yes" ? t.verification.yes : v === "no" ? t.verification.no : v || "—");
  if (events.length === 0) return <p className="text-sm text-ink-500">{t.verification.historyEmpty}</p>;
  return (
    <ol className="space-y-4 border-l-2 border-brand-100 pl-4">
      {events.map((e) => (
        <li key={e.id} className="text-sm">
          <p className="font-semibold">
            {t.verification.actions[e.action]}
            {e.submission_no > 0 ? (
              <span className="font-normal text-ink-500"> · {format(t.verification.submission, { n: e.submission_no })}</span>
            ) : null}
          </p>
          <p className="text-ink-500 wrap-anywhere">
            {e.actor || t.verification.reviewer} · {formatDate(e.created_at, locale)}
          </p>
          {e.changed.length > 0 ? (
            <ul className="mt-1 text-ink-600">
              {e.changed.map((c) => (
                <li key={c.field} className="wrap-anywhere">
                  {labels[c.field] ?? c.field}: {flag(c.old)} → {flag(c.new)}
                </li>
              ))}
            </ul>
          ) : null}
          {e.comment ? <p className="mt-1 whitespace-pre-line rounded-lg bg-ink-50 p-3 wrap-anywhere">{e.comment}</p> : null}
        </li>
      ))}
    </ol>
  );
}
