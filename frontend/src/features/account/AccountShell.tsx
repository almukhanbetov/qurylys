"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { accountApi, authApi } from "@/lib/api";
import { hardNavigate } from "@/lib/redirect";
import { useI18n } from "@/lib/i18n/client";
import { useSubmit } from "@/hooks/useSubmit";
import type { Me } from "@/types/api";
import { Alert } from "@/components/ui/Field";
import { MeProvider } from "./MeContext";

export function AccountShell({ me, children }: { me: Me; children: ReactNode }) {
  const { t } = useI18n();
  const pathname = usePathname();
  const { pending, error, run } = useSubmit();

  const items = [
    { href: "/account", label: t.account.overview },
    ...(me.active_company_id
      ? [
          { href: "/account/company", label: t.account.company },
          { href: "/account/members", label: t.account.members },
        ]
      : []),
    {
      href: "/account/invitations",
      label: t.account.invitations + (me.pending_invitations ? ` (${me.pending_invitations})` : ""),
    },
    { href: "/account/profile", label: t.account.profile },
    { href: "/account/security", label: t.account.security },
    ...(me.platform_roles.includes("admin") ? [{ href: "/account/admin", label: t.account.admin }] : []),
  ];
  const isActive = (href: string) => (href === "/account" ? pathname === href : pathname.startsWith(href));

  return (
    <MeProvider me={me}>
      <div className="mx-auto grid max-w-7xl gap-6 px-4 py-6 sm:px-6 lg:grid-cols-[260px_minmax(0,1fr)] lg:py-8">
        <aside className="min-w-0 space-y-4 lg:self-start">
          {me.companies.length > 0 ? (
            <label className="block rounded-2xl border border-ink-200 bg-white p-4">
              <span className="text-xs font-bold uppercase tracking-wide text-ink-500">{t.account.activeCompany}</span>
              <select
                value={me.active_company_id ?? ""}
                disabled={pending}
                onChange={(e) => {
                  const id = Number(e.target.value);
                  void run(async () => {
                    await accountApi.setActiveCompany(id);
                    // Full load drops everything cached for the previous company.
                    hardNavigate("/account");
                  });
                }}
                className="mt-2 h-11 w-full rounded-xl border border-ink-300 bg-white px-3 text-base text-ink-900 focus:border-brand-700 focus:outline-none focus:ring-2 focus:ring-brand-200"
              >
                {me.active_company_id ? null : <option value="">{t.account.chooseCompany}</option>}
                {me.companies.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </select>
            </label>
          ) : null}
          {error ? <Alert>{error}</Alert> : null}
          <nav aria-label={t.account.title} className="rounded-2xl border border-ink-200 bg-white p-2">
            <ul className="grid grid-cols-2 gap-1 lg:grid-cols-1">
              {items.map((it) => (
                <li key={it.href} className="min-w-0">
                  <Link
                    href={it.href}
                    aria-current={isActive(it.href) ? "page" : undefined}
                    className={`block rounded-lg px-3 py-2.5 text-sm wrap-break-word transition-colors ${
                      isActive(it.href) ? "bg-brand-100 font-semibold text-brand-900" : "text-ink-700 hover:bg-ink-50"
                    }`}
                  >
                    {it.label}
                  </Link>
                </li>
              ))}
              <li>
                <button
                  type="button"
                  disabled={pending}
                  onClick={() =>
                    void run(async () => {
                      await authApi.logout();
                      hardNavigate("/");
                    })
                  }
                  className="w-full cursor-pointer rounded-lg px-3 py-2.5 text-left text-sm text-red-700 hover:bg-red-50"
                >
                  {t.account.logout}
                </button>
              </li>
            </ul>
          </nav>
        </aside>
        <div className="min-w-0">{children}</div>
      </div>
    </MeProvider>
  );
}
