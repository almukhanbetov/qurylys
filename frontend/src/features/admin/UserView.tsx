"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { adminApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { format } from "@/lib/i18n";
import { formatDate } from "@/lib/format";
import { useApiQuery } from "@/hooks/useApiQuery";
import { useSubmit } from "@/hooks/useSubmit";
import { PLATFORM_PERMISSIONS, type AdminUserDetail, type PlatformPermission } from "@/types/api";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Alert, Card } from "@/components/ui/Field";
import { useMe } from "@/features/account/MeContext";
import { QueryState } from "@/features/account/QueryState";

export function UserView({ id }: { id: number }) {
  const { t, locale } = useI18n();
  const a = t.admin;
  const result = useApiQuery(`admin-user:${id}`, (signal) => adminApi.user(id, signal));

  return (
    <div className="space-y-4">
      <Link href="/account/admin/users" className="text-sm font-semibold text-brand-700 hover:underline">
        {a.back}
      </Link>
      <QueryState result={result}>
        {(u) => (
          <>
            <h1 className="text-2xl font-bold wrap-anywhere">{u.full_name}</h1>
            <Card title={a.user.contacts}>
              <dl className="grid gap-3 text-sm sm:grid-cols-2">
                {(
                  [
                    [t.account.email, u.email, u.email_verified_at],
                    [t.account.phone, u.phone, u.phone_verified_at],
                  ] as const
                ).map(([label, value, at]) => (
                  <div key={label} className="min-w-0">
                    <dt className="text-ink-500">{label}</dt>
                    <dd className="font-medium wrap-anywhere">
                      {value ?? a.user.noContact}
                      {value && at ? <span className="ml-2 text-xs text-ink-500">{format(a.user.confirmedAt, { date: formatDate(at, locale) })}</span> : null}
                    </dd>
                  </div>
                ))}
                <div>
                  <dt className="text-ink-500">{a.user.status}</dt>
                  <dd>
                    <Badge tone={u.status === "active" ? "brand" : "danger"}>{a.users.statusOptions[u.status]}</Badge>
                  </dd>
                </div>
                <div>
                  <dt className="text-ink-500">{a.user.registered}</dt>
                  <dd className="font-medium">{formatDate(u.created_at, locale)}</dd>
                </div>
                <div>
                  <dt className="text-ink-500">{a.user.lastSeen}</dt>
                  <dd className="font-medium">{u.last_seen_at ? formatDate(u.last_seen_at, locale) : a.user.never}</dd>
                </div>
              </dl>
              <p className="mt-4 text-xs text-ink-500">{a.user.contactsNote}</p>
            </Card>
            <Grants user={u} onChanged={result.retry} />
            <Card title={a.user.memberships}>
              {u.memberships.length === 0 ? (
                <p className="text-sm text-ink-500">{a.user.noMemberships}</p>
              ) : (
                <ul className="divide-y divide-ink-100">
                  {u.memberships.map((m) => (
                    <li key={m.company_id} className="flex flex-wrap items-center justify-between gap-2 py-3 text-sm">
                      <span className="min-w-0 wrap-anywhere">
                        <Link href={`/account/admin/companies/${m.company_id}`} className="font-semibold text-brand-700 hover:underline">
                          {m.name}
                        </Link>
                        <span className="ml-2 text-ink-500">{format(a.user.joined, { date: formatDate(m.joined_at, locale) })}</span>
                      </span>
                      <span className="flex flex-wrap gap-2">
                        <Badge>{m.role === "owner" ? t.account.owner : t.account.employee}</Badge>
                        <Badge tone={m.verification === "approved" ? "brand" : "muted"}>{t.verification.status[m.verification]}</Badge>
                        {m.status === "suspended" ? <Badge tone="danger">{a.companies.suspended}</Badge> : null}
                        {m.is_demo ? <Badge tone="demo">{a.user.demo}</Badge> : null}
                      </span>
                    </li>
                  ))}
                </ul>
              )}
            </Card>
            <Card title={a.user.history}>
              {u.history.length === 0 ? (
                <p className="text-sm text-ink-500">{a.user.noHistory}</p>
              ) : (
                <ol className="space-y-2 text-sm">
                  {u.history.map((h, i) => (
                    <li key={i} className="wrap-anywhere">
                      <span className="text-ink-500">{formatDate(h.created_at, locale)}</span>{" "}
                      {a.user.actions[h.action as keyof typeof a.user.actions] ?? h.action}:{" "}
                      <span className="font-medium">{label(a, h.value)}</span> · {h.actor || a.user.operator}
                    </li>
                  ))}
                </ol>
              )}
            </Card>
          </>
        )}
      </QueryState>
    </div>
  );
}

type AdminDict = ReturnType<typeof useI18n>["t"]["admin"];

function label(a: AdminDict, value: string): string {
  if (value === "admin" || value === "mrx") return a.roles[value];
  return a.permissions[value as PlatformPermission] ?? value;
}

function Grants({ user: u, onChanged }: { user: AdminUserDetail; onChanged: () => void }) {
  const { t, locale } = useI18n();
  const a = t.admin;
  const me = useMe();
  const router = useRouter();
  const { pending, error, run } = useSubmit();
  const granted = (kind: "role" | "permission", value: string) => u.grants.find((g) => g.kind === kind && g.value === value);
  const admin = granted("role", "admin");

  function change(action: () => Promise<unknown>) {
    void run(action).then((ok) => {
      if (!ok) return;
      onChanged();
      // Own rights changed: reload the menu.
      if (u.id === me.id) router.refresh();
    });
  }

  const rows: { key: string; title: string; text?: string; held: ReturnType<typeof granted>; toggle: (on: boolean) => Promise<unknown> }[] = [
    { key: "mrx", title: a.roles.mrx, held: granted("role", "mrx"), toggle: (on) => adminApi.setMrx(u.id, on) },
    ...PLATFORM_PERMISSIONS.map((p) => ({
      key: p,
      title: a.permissions[p],
      text: a.permissionText[p],
      held: granted("permission", p),
      toggle: (on: boolean) => adminApi.setPermission(u.id, p, on),
    })),
  ];

  return (
    <Card title={a.user.grants}>
      {error ? <div className="mb-3"><Alert>{error}</Alert></div> : null}
      <ul className="divide-y divide-ink-100">
        {admin ? (
          <li className="py-3 text-sm">
            <Badge>{a.roles.admin}</Badge>{" "}
            <span className="text-ink-500">{format(a.user.grantedAt, { date: formatDate(admin.granted_at, locale) })}</span>
          </li>
        ) : null}
        {rows.map((r) => (
          <li key={r.key} className="flex flex-wrap items-center justify-between gap-3 py-3">
            <div className="min-w-0 flex-1 basis-60 text-sm">
              <p className="font-semibold">{r.title}</p>
              {r.text ? <p className="text-ink-500">{r.text}</p> : null}
              {r.held ? <p className="text-ink-500">{format(a.user.grantedAt, { date: formatDate(r.held.granted_at, locale) })}</p> : null}
            </div>
            <Button
              variant="secondary"
              disabled={pending}
              onClick={() => {
                if (!r.held && u.id === me.id && !window.confirm(format(a.confirmSelf, { name: r.title }))) return;
                change(() => r.toggle(!r.held));
              }}
            >
              {r.held ? a.revoke : a.grant}
            </Button>
          </li>
        ))}
      </ul>
      <p className="mt-3 text-xs text-ink-500">{a.user.adminNote}</p>
    </Card>
  );
}
