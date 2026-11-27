"use client";

import { adminApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { format } from "@/lib/i18n";
import { useApiQuery } from "@/hooks/useApiQuery";
import type { SystemRole } from "@/types/api";
import { Badge } from "@/components/ui/Badge";
import { Card } from "@/components/ui/Field";
import { QueryState } from "@/features/account/QueryState";

const statusTone = { implemented: "brand", partial: "attention", planned: "muted" } as const;
const CONTOURS = ["marketplace", "erp", "platform"] as const;

/** Directory of the 19 roles: what each may do, where, who assigns it, and what works now. */
export function RolesView() {
  const { t } = useI18n();
  const r = t.admin.roleDirectory;
  const result = useApiQuery("admin-roles", (signal) => adminApi.roles(signal));

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">{t.admin.sections.roles}</h1>
      <p className="text-sm text-ink-600">{r.intro}</p>
      <QueryState result={result}>
        {(d) => (
          <>
            <Card title={r.summary}>
              <div className="-mx-4 overflow-x-auto px-4 sm:mx-0 sm:px-0">
                <table className="w-full text-left text-sm">
                  <thead className="text-ink-500">
                    <tr>
                      <th className="py-2 pr-3 font-medium">{r.code}</th>
                      <th className="py-2 pr-3 font-medium">{r.name}</th>
                      <th className="hidden py-2 pr-3 font-medium sm:table-cell">{r.contour}</th>
                      <th className="py-2 font-medium">{r.state}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-ink-100">
                    {d.roles.map((x) => (
                      <tr key={x.code}>
                        <td className="py-2 pr-3 font-mono font-semibold">
                          <a href={`#role-${x.code}`} className="text-brand-700 hover:underline">{x.code}</a>
                        </td>
                        <td className="py-2 pr-3">{r.items[x.code].name}</td>
                        <td className="hidden py-2 pr-3 sm:table-cell">{r.contours[x.contour]}</td>
                        <td className="py-2"><Badge tone={statusTone[x.status]}>{r.status[x.status]}</Badge></td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </Card>
            {CONTOURS.map((c) => (
              <section key={c} className="space-y-3">
                <h2 className="text-lg font-bold">{r.contours[c]}</h2>
                {d.roles.filter((x) => x.contour === c).map((x) => (
                  <RoleCard key={x.code} role={x} />
                ))}
              </section>
            ))}
            <Card title={r.mechanismsTitle}>
              <p className="mb-3 text-sm text-ink-600">{r.mechanismsText}</p>
              <ul className="divide-y divide-ink-100">
                {d.mechanisms.map((m) => (
                  <li key={m.key} className="flex flex-wrap items-start justify-between gap-2 py-3 text-sm">
                    <div className="min-w-0 flex-1 basis-60">
                      <p className="font-semibold">{r.mechanisms[m.key].name}</p>
                      <p className="text-ink-600">{r.mechanisms[m.key].text}</p>
                    </div>
                    <span className="text-ink-500">{format(r.holders, { count: m.holders })}</span>
                  </li>
                ))}
              </ul>
            </Card>
          </>
        )}
      </QueryState>
    </div>
  );
}

function RoleCard({ role: x }: { role: SystemRole }) {
  const { t } = useI18n();
  const r = t.admin.roleDirectory;
  const it = r.items[x.code];
  const rows: [string, string][] = [
    [r.purpose, it.purpose],
    [r.actions, it.actions],
    [r.area, `${it.area} ${r.scope[x.scope]}; ${r.branch[x.branch_scope]}.`],
    [r.assigns, it.assigns],
    [r.now, it.now],
  ];
  const held =
    x.via === "platform_role"
      ? format(r.holders, { count: x.holders })
      : x.via === "company_direction"
        ? format(r.companies, { companies: x.companies, count: x.holders })
        : null;
  return (
    <article id={`role-${x.code}`} className="scroll-mt-24 rounded-2xl border border-ink-200 bg-white p-4 sm:p-6">
      <header className="flex flex-wrap items-center gap-2">
        <span className="rounded-lg bg-brand-50 px-2 py-0.5 font-mono text-sm font-bold text-brand-800">{x.code}</span>
        <h3 className="text-base font-bold">{it.name}</h3>
        <Badge tone={statusTone[x.status]}>{r.status[x.status]}</Badge>
      </header>
      <dl className="mt-3 grid gap-3 text-sm sm:grid-cols-2">
        {rows.map(([label, value]) => (
          <div key={label} className="min-w-0">
            <dt className="text-ink-500">{label}</dt>
            <dd className="wrap-anywhere">{value}</dd>
          </div>
        ))}
        <div className="min-w-0">
          <dt className="text-ink-500">{r.howHeld}</dt>
          <dd className="wrap-anywhere">
            {r.via[x.via]}
            {held ? <span className="mt-1 block text-ink-500">{held}</span> : null}
          </dd>
        </div>
      </dl>
    </article>
  );
}
