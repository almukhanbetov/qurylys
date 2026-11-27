"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useI18n } from "@/lib/i18n/client";
import { useMe } from "@/features/account/MeContext";
import { adminSections } from "./access";

/** Tabs inside "Administration": only the sections this user may open. */
export function AdminNav() {
  const { t } = useI18n();
  const me = useMe();
  const pathname = usePathname();
  const router = useRouter();
  const sections = adminSections(me);
  if (sections.length === 0) return null;
  const current = sections.find(({ section }) => pathname.startsWith(`/account/admin/${section}`));
  const label = (s: (typeof sections)[number]) => t.admin.sections[s.section] + (s.allowed ? "" : ` (${t.admin.gate.badge})`);
  return (
    <nav aria-label={t.admin.title}>
      {/* Phone: one select instead of a row that hides most sections. */}
      <label className="block text-sm font-medium text-ink-900 sm:hidden">
        {t.admin.title}
        <select
          value={current?.section ?? ""}
          onChange={(e) => router.push(e.target.value ? `/account/admin/${e.target.value}` : "/account/admin")}
          className="mt-1 h-11 w-full rounded-xl border border-ink-300 bg-white px-3 text-base text-ink-900 focus:border-brand-700 focus:outline-none focus:ring-2 focus:ring-brand-200"
        >
          <option value="">{t.admin.title}</option>
          {sections.map((s) => (
            <option key={s.section} value={s.section}>{label(s)}</option>
          ))}
        </select>
      </label>
      {/* Wider screens: every tab is visible, wrapping to a second row when needed. */}
      <ul className="hidden flex-wrap gap-2 border-b border-ink-200 pb-2 sm:flex">
        {sections.map(({ section: s, allowed }) => {
          const href = `/account/admin/${s}`;
          const active = current?.section === s;
          return (
            <li key={s}>
              <Link
                href={href}
                aria-current={active ? "page" : undefined}
                className={`inline-flex min-h-11 items-center whitespace-nowrap rounded-xl px-3 text-sm font-semibold transition-colors ${
                  active ? "bg-brand-100 text-brand-900" : "text-ink-700 hover:bg-ink-50"
                }`}
              >
                {t.admin.sections[s]}
                {allowed ? null : <span className="ml-2 rounded-full bg-ink-100 px-2 py-0.5 text-[10px] font-bold uppercase text-ink-500">{t.admin.gate.badge}</span>}
              </Link>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
