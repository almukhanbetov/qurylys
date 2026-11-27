"use client";

import Link from "next/link";
import { useState } from "react";
import { adminApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { format } from "@/lib/i18n";
import { formatDate } from "@/lib/format";
import { useApiQuery } from "@/hooks/useApiQuery";
import type { AdminCompanyItem } from "@/types/api";
import { Badge } from "@/components/ui/Badge";
import { Card } from "@/components/ui/Field";
import { EmptyState } from "@/components/ui/StateMessage";
import { Pagination } from "@/features/catalog/Pagination";
import { QueryState } from "@/features/account/QueryState";
import { SearchBar } from "./parts";

function CompanyBadges({ c }: { c: AdminCompanyItem }) {
  const { t } = useI18n();
  const a = t.admin;
  return (
    <span className="flex flex-wrap gap-2">
      {c.is_buyer ? <Badge tone="muted">{a.companies.buyer}</Badge> : null}
      {c.is_supplier ? <Badge tone="muted">{a.companies.supplier}</Badge> : null}
      <Badge tone={c.verification === "approved" ? "brand" : "muted"}>{t.verification.status[c.verification]}</Badge>
      {c.status === "suspended" ? <Badge tone="danger">{a.companies.suspended}</Badge> : null}
      {c.is_demo ? <Badge tone="demo">{a.user.demo}</Badge> : null}
    </span>
  );
}

export function CompaniesView() {
  const { t } = useI18n();
  const a = t.admin;
  const [q, setQ] = useState("");
  const [page, setPage] = useState(1);
  const result = useApiQuery(`admin-companies:${q}:${page}`, (signal) => adminApi.companies(q, page, signal));

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">{a.sections.companies}</h1>
      <p className="text-sm text-ink-600">{a.companies.text}</p>
      <SearchBar
        label={a.companies.searchLabel}
        initial={q}
        onSearch={(v) => {
          setQ(v);
          setPage(1);
        }}
      />
      <QueryState result={result}>
        {(data) =>
          data.items.length === 0 ? (
            <EmptyState title={a.companies.empty} />
          ) : (
            <>
              <p className="text-sm text-ink-500">{format(a.total, { count: data.total })}</p>
              <ul className="space-y-3">
                {data.items.map((c) => (
                  <li key={c.id} className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-ink-200 bg-white p-4">
                    <div className="min-w-0 flex-1 basis-60 space-y-1">
                      <p className="font-semibold wrap-anywhere">{c.name}</p>
                      <p className="text-sm text-ink-500">
                        {[c.bin ? `${t.account.bin}: ${c.bin}` : "", c.city ?? "", format(a.companies.members, { count: c.member_count })].filter(Boolean).join(" · ")}
                      </p>
                      <CompanyBadges c={c} />
                    </div>
                    <Link href={`/account/admin/companies/${c.id}`} className="inline-flex min-h-11 items-center font-semibold text-brand-700 hover:underline">
                      {a.open}
                    </Link>
                  </li>
                ))}
              </ul>
              <Pagination page={data.page} totalPages={data.total_pages} onChange={setPage} />
            </>
          )
        }
      </QueryState>
    </div>
  );
}

export function CompanyMembersView({ id }: { id: number }) {
  const { t, locale } = useI18n();
  const a = t.admin;
  const result = useApiQuery(`admin-company:${id}`, (signal) => adminApi.companyMembers(id, signal));
  return (
    <div className="space-y-4">
      <Link href="/account/admin/companies" className="text-sm font-semibold text-brand-700 hover:underline">
        {a.back}
      </Link>
      <QueryState result={result}>
        {({ company, members }) => (
          <>
            <h1 className="text-2xl font-bold wrap-anywhere">{company.name}</h1>
            <CompanyBadges c={company} />
            <p className="text-sm text-ink-600">{a.companies.text}</p>
            <Card title={format(a.companies.members, { count: members.length })}>
              {members.length === 0 ? (
                <p className="text-sm text-ink-500">{a.companies.noMembers}</p>
              ) : (
                <ul className="divide-y divide-ink-100">
                  {members.map((m) => (
                    <li key={m.user_id} className="flex flex-wrap items-center justify-between gap-2 py-3 text-sm">
                      <span className="min-w-0 wrap-anywhere">
                        <Link href={`/account/admin/users/${m.user_id}`} className="font-semibold text-brand-700 hover:underline">
                          {m.full_name}
                        </Link>
                        <span className="ml-2 text-ink-500">{format(a.user.joined, { date: formatDate(m.joined_at, locale) })}</span>
                      </span>
                      <Badge>{m.role === "owner" ? t.account.owner : t.account.employee}</Badge>
                    </li>
                  ))}
                </ul>
              )}
            </Card>
          </>
        )}
      </QueryState>
    </div>
  );
}
