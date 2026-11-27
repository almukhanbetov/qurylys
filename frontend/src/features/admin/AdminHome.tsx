"use client";

import Link from "next/link";
import { useI18n } from "@/lib/i18n/client";
import { useMe } from "@/features/account/MeContext";
import { Alert } from "@/components/ui/Field";
import { adminSections } from "./access";

export function AdminHome() {
  const { t } = useI18n();
  const sections = adminSections(useMe());
  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">{t.admin.title}</h1>
      <p className="text-sm text-ink-600">{t.admin.intro}</p>
      {sections.length === 0 ? (
        <Alert tone="info">{t.admin.noSections}</Alert>
      ) : (
        <ul className="grid gap-3 sm:grid-cols-2">
          {sections.map(({ section: s, allowed }) => (
            <li key={s}>
              <Link
                href={`/account/admin/${s}`}
                className="block h-full rounded-2xl border border-ink-200 bg-white p-4 transition-colors hover:border-brand-700"
              >
                <span className="block font-bold text-brand-800">{t.admin.sections[s]}</span>
                <span className="mt-1 block text-sm text-ink-600">{t.admin.sectionText[s]}</span>
                {allowed ? null : <span className="mt-2 block text-xs font-semibold text-amber-800">{t.admin.gate.tile}</span>}
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
