"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useI18n } from "@/lib/i18n/client";
import { useMe } from "@/features/account/MeContext";
import { adminSections } from "./access";

/** Tabs inside "Administration": only the sections this user may open. */
export function AdminNav() {
  const { t } = useI18n();
  const me = useMe();
  const pathname = usePathname();
  const sections = adminSections(me);
  if (sections.length === 0) return null;
  return (
    <nav aria-label={t.admin.title} className="-mx-4 overflow-x-auto px-4 sm:mx-0 sm:px-0">
      <ul className="flex min-w-max gap-2 border-b border-ink-200 pb-2">
        {sections.map(({ section: s, allowed }) => {
          const href = `/account/admin/${s}`;
          const active = pathname.startsWith(href);
          return (
            <li key={s}>
              <Link
                href={href}
                aria-current={active ? "page" : undefined}
                ref={active ? (el) => el?.scrollIntoView({ block: "nearest", inline: "nearest" }) : undefined}
                className={`inline-flex min-h-11 items-center rounded-xl px-3 text-sm font-semibold transition-colors ${
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
