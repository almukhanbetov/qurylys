"use client";

import { useRouter } from "next/navigation";
import { useRef, useState } from "react";
import { ApiError, verificationApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { useApiQuery, type QueryResult } from "@/hooks/useApiQuery";
import { useSubmit } from "@/hooks/useSubmit";
import { DOCUMENT_KINDS, type CompanyVerification, type DocumentKind } from "@/types/api";
import { Button, ButtonLink } from "@/components/ui/Button";
import { Alert, Card } from "@/components/ui/Field";
import { NoCompany } from "./CompanyView";
import { useActiveCompany } from "./MeContext";
import { QueryState } from "./QueryState";
import { DocumentList, EventList, RequisitesList, VerificationBadge } from "./VerificationParts";

export function VerificationView() {
  const { t } = useI18n();
  const active = useActiveCompany();
  if (!active) return <NoCompany />;
  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">{t.verification.title}</h1>
      <Loaded key={active.id} id={active.id} owner={active.role === "owner" && active.status === "active"} />
    </div>
  );
}

function Loaded({ id, owner }: { id: number; owner: boolean }) {
  const result = useApiQuery(`verification:${id}`, (signal) => verificationApi.get(id, signal));
  return <QueryState result={result}>{(v) => <Content id={id} owner={owner} v={v} result={result} />}</QueryState>;
}

function Content({
  id,
  owner,
  v,
  result,
}: {
  id: number;
  owner: boolean;
  v: CompanyVerification;
  result: QueryResult<CompanyVerification>;
}) {
  const { t } = useI18n();
  const router = useRouter();
  const { pending, error, run } = useSubmit();
  const act = (fn: () => Promise<void>) =>
    void run(fn).then((ok) => {
      if (!ok) return;
      result.retry();
      router.refresh(); // the shell and the overview show the status
    });
  const canCreate = owner && (v.status === "none" || v.status === "rejected" || v.status === "reverification");
  const editable = owner && v.can_edit;

  return (
    <>
      <Card>
        <VerificationBadge status={v.status} />
        <p className="mt-3 text-sm text-ink-600">{t.verification.hint[v.status]}</p>
        {v.comment ? (
          <div className="mt-4 rounded-xl bg-amber-50 p-4 text-sm text-amber-900 ring-1 ring-amber-200">
            <p className="font-semibold">{t.verification.comment}</p>
            <p className="mt-1 whitespace-pre-line wrap-anywhere">{v.comment}</p>
          </div>
        ) : null}
        {owner ? null : <p className="mt-3 text-sm text-ink-500">{t.verification.ownerOnly}</p>}
        {error ? <div className="mt-4"><Alert>{error}</Alert></div> : null}
        {canCreate ? (
          <Button className="mt-4" disabled={pending} onClick={() => act(() => verificationApi.create(id))}>
            {t.verification.create}
          </Button>
        ) : null}
        {editable ? (
          <Button
            className="mt-4"
            disabled={pending}
            onClick={() => {
              if (window.confirm(t.verification.confirmSubmit)) act(() => verificationApi.submit(id));
            }}
          >
            {v.status === "needs_changes" ? t.verification.resubmit : t.verification.submit}
          </Button>
        ) : null}
      </Card>

      <Card title={t.verification.requisites}>
        <RequisitesList value={v.requisites} />
        {editable ? (
          <ButtonLink href="/account/company" variant="secondary" className="mt-4">
            {t.verification.editRequisites}
          </ButtonLink>
        ) : null}
      </Card>

      {owner && v.status !== "none" ? (
        <Card title={t.verification.documents}>
          {editable ? <UploadForm id={id} full={v.documents.length >= v.max_documents} maxSize={v.max_file_size} onDone={result.retry} /> : null}
          <DocumentList
            documents={v.documents}
            url={(doc) => verificationApi.documentUrl(id, doc)}
            disabled={pending}
            onRemove={editable ? (doc) => act(() => verificationApi.removeDocument(id, doc)) : undefined}
          />
        </Card>
      ) : null}

      <Card title={t.verification.history}>
        <EventList events={v.events} />
      </Card>
    </>
  );
}

function UploadForm({ id, full, maxSize, onDone }: { id: number; full: boolean; maxSize: number; onDone: () => void }) {
  const { t } = useI18n();
  const { pending, error, run } = useSubmit();
  const [kind, setKind] = useState<DocumentKind>("registration");
  const input = useRef<HTMLInputElement>(null);
  const control =
    "mt-1 min-h-11 w-full rounded-xl border border-ink-300 bg-white px-3 text-base text-ink-900 focus:border-brand-700 focus:outline-none focus:ring-2 focus:ring-brand-200";

  return (
    <form
      className="mb-4 space-y-3"
      onSubmit={(e) => {
        e.preventDefault();
        const file = input.current?.files?.[0];
        if (!file) return;
        void run(async () => {
          // The API checks again; this only saves a useless upload.
          if (file.size > maxSize) throw new ApiError(413, "file_too_large", "File is too large");
          await verificationApi.upload(id, kind, file);
        }).then((ok) => {
          if (!ok) return;
          if (input.current) input.current.value = "";
          onDone();
        });
      }}
    >
      <p className="text-sm text-ink-600">{t.verification.documentsHint}</p>
      <fieldset disabled={pending || full} className="grid gap-3 sm:grid-cols-2">
        <label className="min-w-0 text-sm font-medium">
          {t.verification.kind}
          <select value={kind} onChange={(e) => setKind(e.target.value as DocumentKind)} className={control}>
            {DOCUMENT_KINDS.map((k) => (
              <option key={k} value={k}>
                {t.verification.kinds[k]}
              </option>
            ))}
          </select>
        </label>
        <label className="min-w-0 text-sm font-medium">
          {t.verification.file}
          <input ref={input} type="file" accept="application/pdf,image/jpeg,image/png" required className={`${control} py-2 text-sm`} />
        </label>
        <div>
          <Button type="submit" variant="secondary">{pending ? t.common.loading : t.verification.add}</Button>
        </div>
      </fieldset>
      {full ? <Alert tone="info">{t.errors.too_many_documents}</Alert> : null}
      {error ? <Alert>{error}</Alert> : null}
    </form>
  );
}
