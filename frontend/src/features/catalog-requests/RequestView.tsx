"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { catalogRequestApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { useApiQuery } from "@/hooks/useApiQuery";
import type { CatalogRequestFields, CompanyCatalogRequest } from "@/types/api";
import { ButtonLink } from "@/components/ui/Button";
import { Alert, Card } from "@/components/ui/Field";
import { NoCompany } from "@/features/account/CompanyView";
import { useActiveCompany } from "@/features/account/MeContext";
import { QueryState } from "@/features/account/QueryState";
import { GtinBadge, RequestBadge, RequestDetails, RequestEvents } from "./parts";
import { emptyFields, RequestForm } from "./RequestForm";

function BackLink() {
  const { t } = useI18n();
  return (
    <Link href="/account/catalog-requests" className="text-sm font-semibold text-brand-700 hover:underline">
      {t.catalogRequests.title}
    </Link>
  );
}

export function NewRequestView() {
  const { t } = useI18n();
  const router = useRouter();
  const active = useActiveCompany();
  // Kept after the first save, so a failed submission does not create a second draft.
  const [id, setId] = useState<number | null>(null);
  if (!active) return <NoCompany />;
  if (active.role !== "owner" || !active.is_supplier) {
    return <Alert>{active.is_supplier ? t.errors.forbidden : t.errors.not_supplier}</Alert>;
  }
  const save = async (f: CatalogRequestFields): Promise<number> => {
    if (id) {
      await catalogRequestApi.update(active.id, id, f);
      return id;
    }
    const created = await catalogRequestApi.create(active.id, f);
    setId(created.id);
    return created.id;
  };
  return (
    <div className="space-y-4">
      <BackLink />
      <h1 className="text-2xl font-bold">{t.catalogRequests.create}</h1>
      <Card>
        <RequestForm
          initial={emptyFields}
          onSave={async (f) => router.replace(`/account/catalog-requests/${await save(f)}`)}
          onSubmit={async (f) => {
            const saved = await save(f);
            await catalogRequestApi.submit(active.id, saved);
            router.replace(`/account/catalog-requests/${saved}`);
          }}
        />
      </Card>
    </div>
  );
}

export function RequestView({ id }: { id: number }) {
  const active = useActiveCompany();
  if (!active) return <NoCompany />;
  return (
    <div className="space-y-4">
      <BackLink />
      <Loaded key={`${active.id}:${id}`} companyId={active.id} id={id} canEdit={active.role === "owner" && active.is_supplier} />
    </div>
  );
}

function Loaded({ companyId, id, canEdit }: { companyId: number; id: number; canEdit: boolean }) {
  const { t } = useI18n();
  const c = t.catalogRequests;
  const result = useApiQuery(`catalog-request:${companyId}:${id}`, (signal) => catalogRequestApi.get(companyId, id, signal));
  return (
    <QueryState result={result}>
      {(r: CompanyCatalogRequest) => (
        <>
          <h1 className="text-2xl font-bold wrap-anywhere">{r.name_ru || r.name_kk || c.edit}</h1>
          <Card>
            <p className="flex flex-wrap gap-2">
              <RequestBadge status={r.status} />
              {r.gtin ? <GtinBadge confirmed={r.gtin_confirmed} /> : null}
            </p>
            <p className="mt-3 text-sm text-ink-600">{c.hint[r.status]}</p>
            {r.comment ? (
              <div className="mt-4 rounded-xl bg-amber-50 p-4 text-sm text-amber-900 ring-1 ring-amber-200">
                <p className="font-semibold">{c.comment}</p>
                <p className="mt-1 whitespace-pre-line wrap-anywhere">{r.comment}</p>
              </div>
            ) : null}
            {r.product_id ? (
              <div className="mt-4 space-y-3">
                <p className="text-sm">{c.catalogNo}: <span className="font-semibold">{r.catalog_no}</span></p>
                <ButtonLink href={`/products/${r.product_id}`} variant="secondary">{c.openCard}</ButtonLink>
              </div>
            ) : null}
          </Card>
          <Card>
            {canEdit && (r.status === "draft" || r.status === "needs_changes") ? (
              <RequestForm
                initial={{
                  category_id: r.category_id, name_kk: r.name_kk, name_ru: r.name_ru, brand: r.brand, unit: r.unit,
                  description_kk: r.description_kk, description_ru: r.description_ru, gtin: r.gtin,
                  gtin_source_kind: r.gtin_source_kind, gtin_source: r.gtin_source,
                }}
                onSave={async (f) => {
                  await catalogRequestApi.update(companyId, id, f);
                  result.retry();
                }}
                onSubmit={async (f) => {
                  await catalogRequestApi.update(companyId, id, f);
                  await catalogRequestApi.submit(companyId, id);
                  result.retry();
                }}
              />
            ) : (
              <RequestDetails value={r} />
            )}
          </Card>
          <Card title={t.verification.history}><RequestEvents events={r.events} /></Card>
        </>
      )}
    </QueryState>
  );
}
