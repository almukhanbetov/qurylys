"use client";

import Link from "next/link";
import { useState } from "react";
import { adminApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { format } from "@/lib/i18n";
import { formatDate } from "@/lib/format";
import { useApiQuery } from "@/hooks/useApiQuery";
import { PLATFORM_PERMISSIONS, type UserAccessFilter, type UserFilter } from "@/types/api";
import { Badge } from "@/components/ui/Badge";
import { EmptyState } from "@/components/ui/StateMessage";
import { Pagination } from "@/features/catalog/Pagination";
import { QueryState } from "@/features/account/QueryState";
import { SearchBar, Select } from "./parts";

/** Users list; `staff` fixes the filter to holders of any platform right. */
export function UsersView({ staff = false }: { staff?: boolean }) {
  const { t, locale } = useI18n();
  const a = t.admin;
  const [f, setF] = useState<UserFilter>({ q: "", access: staff ? "staff" : "", membership: "", status: "", page: 1 });
  const set = (patch: Partial<UserFilter>) => setF((prev) => ({ ...prev, page: 1, ...patch }));
  const result = useApiQuery(`admin-users:${JSON.stringify(f)}`, (signal) => adminApi.users(f, signal));

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">{staff ? a.sections.staff : a.sections.users}</h1>
      {staff ? <p className="text-sm text-ink-600">{a.staff.text}</p> : null}
      <SearchBar label={a.users.searchLabel} initial={f.q} onSearch={(q) => set({ q })}>
        {staff ? null : (
          <Select label={a.users.access} value={f.access} onChange={(v) => set({ access: v as UserAccessFilter })}>
            <option value="">{a.all}</option>
            <option value="staff">{a.users.accessOptions.staff}</option>
            <option value="none">{a.users.accessOptions.none}</option>
            <option value="admin">{a.roles.admin}</option>
            <option value="mrx">{a.roles.mrx}</option>
            {PLATFORM_PERMISSIONS.map((p) => (
              <option key={p} value={p}>{a.permissions[p]}</option>
            ))}
          </Select>
        )}
        <Select label={a.users.membership} value={f.membership} onChange={(v) => set({ membership: v as UserFilter["membership"] })}>
          <option value="">{a.all}</option>
          <option value="with">{a.users.membershipOptions.with}</option>
          <option value="without">{a.users.membershipOptions.without}</option>
        </Select>
        <Select label={a.users.status} value={f.status} onChange={(v) => set({ status: v as UserFilter["status"] })}>
          <option value="">{a.all}</option>
          <option value="active">{a.users.statusOptions.active}</option>
          <option value="blocked">{a.users.statusOptions.blocked}</option>
        </Select>
      </SearchBar>
      <QueryState result={result}>
        {(data) =>
          data.items.length === 0 ? (
            <EmptyState title={staff ? a.staff.empty : a.users.empty} />
          ) : (
            <>
              <p className="text-sm text-ink-500">{format(a.total, { count: data.total })}</p>
              <ul className="space-y-3">
                {data.items.map((u) => (
                  <li key={u.id} className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-ink-200 bg-white p-4">
                    <div className="min-w-0 flex-1 basis-60">
                      <p className="font-semibold wrap-anywhere">{u.full_name}</p>
                      <p className="text-sm text-ink-500 wrap-anywhere">{[u.email, u.phone].filter(Boolean).join(" · ")}</p>
                      <p className="mt-1 text-sm text-ink-500">
                        {format(a.users.companies, { count: u.company_count })} · {format(a.users.registered, { date: formatDate(u.created_at, locale) })}
                      </p>
                      <p className="mt-2 flex flex-wrap gap-2">
                        {u.status === "blocked" ? <Badge tone="danger">{a.users.statusOptions.blocked}</Badge> : null}
                        {u.platform_roles.map((r) => (
                          <Badge key={r}>{a.roles[r]}</Badge>
                        ))}
                        {u.platform_permissions.map((p) => (
                          <Badge key={p} tone="attention">{a.permissions[p]}</Badge>
                        ))}
                      </p>
                    </div>
                    <Link href={`/account/admin/users/${u.id}`} className="inline-flex min-h-11 items-center font-semibold text-brand-700 hover:underline">
                      {a.open}
                    </Link>
                  </li>
                ))}
              </ul>
              <Pagination page={data.page} totalPages={data.total_pages} onChange={(page) => setF((prev) => ({ ...prev, page }))} />
            </>
          )
        }
      </QueryState>
    </div>
  );
}
