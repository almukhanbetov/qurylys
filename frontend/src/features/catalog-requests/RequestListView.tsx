"use client";

import Link from "next/link";
import { catalogRequestApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { pick } from "@/lib/i18n";
import { useApiQuery } from "@/hooks/useApiQuery";
import { ButtonLink } from "@/components/ui/Button";
import { EmptyState } from "@/components/ui/StateMessage";
import { NoCompany } from "@/features/account/CompanyView";
import { useActiveCompany } from "@/features/account/MeContext";
import { QueryState } from "@/features/account/QueryState";
import { GtinBadge, RequestBadge } from "./parts";

export function RequestListView() {
  const { t } = useI18n();
  const active = useActiveCompany();
  if (!active) return <NoCompany />;
  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">{t.catalogRequests.title}</h1>
      <p className="text-sm text-ink-600">{t.catalogRequests.intro}</p>
      <div className="flex flex-wrap gap-3">
        <ButtonLink href="/catalog" variant="secondary">{t.catalogRequests.findInCatalog}</ButtonLink>
        {active.role === "owner" && active.is_supplier ? (
          <ButtonLink href="/account/catalog-requests/new">{t.catalogRequests.create}</ButtonLink>
        ) : null}
      </div>
      <List key={active.id} id={active.id} />
    </div>
  );
}

function List({ id }: { id: number }) {
  const { t, locale } = useI18n();
  const result = useApiQuery(`catalog-requests:${id}`, (signal) => catalogRequestApi.list(id, signal));
  return (
    <QueryState result={result}>
      {(list) =>
        list.length === 0 ? (
          <EmptyState title={t.catalogRequests.empty} />
        ) : (
          <ul className="space-y-3">
            {list.map((r) => (
              <li key={r.id}>
                <Link
                  href={`/account/catalog-requests/${r.id}`}
                  className="block rounded-2xl border border-ink-200 bg-white p-4 transition-colors hover:border-brand-700"
                >
                  <p className="font-semibold wrap-anywhere">{pick(locale, r.name_kk, r.name_ru) || t.catalogRequests.unnamed}</p>
                  <p className="text-sm text-ink-500 wrap-anywhere">
                    {[r.catalog_no ? `${t.catalogRequests.catalogNo}: ${r.catalog_no}` : "", r.gtin ? `GTIN: ${r.gtin}` : ""].filter(Boolean).join(" · ")}
                  </p>
                  <p className="mt-2 flex flex-wrap gap-2">
                    <RequestBadge status={r.status} />
                    {r.gtin ? <GtinBadge confirmed={r.gtin_confirmed} /> : null}
                  </p>
                </Link>
              </li>
            ))}
          </ul>
        )
      }
    </QueryState>
  );
}
