"use client";

import { useTransition } from "react";
import { useI18n } from "@/lib/i18n/client";
import { LOCALES, type Locale } from "@/lib/i18n";
import { setLocale } from "@/lib/i18n/actions";

const labels: Record<Locale, string> = { kk: "KZ", ru: "RU" };

/**
 * Stores the choice via a Server Action; the route re-renders on the server,
 * so both server- and client-rendered texts switch language.
 */
export function LanguageSwitcher() {
  const { locale, t } = useI18n();
  const [pending, startTransition] = useTransition();

  function choose(next: Locale) {
    if (next === locale) return;
    startTransition(() => setLocale(next));
  }

  return (
    <div
      role="group"
      aria-label={t.header.language}
      className={`flex rounded-xl bg-white/10 p-1 text-xs font-bold ${pending ? "opacity-60" : ""}`}
    >
      {LOCALES.map((l) => (
        <button
          key={l}
          type="button"
          onClick={() => choose(l)}
          disabled={pending}
          aria-pressed={l === locale}
          lang={l}
          className={`min-h-9 cursor-pointer rounded-lg px-2.5 transition-colors focus-visible:outline-2 focus-visible:outline-accent-400 ${
            l === locale ? "bg-white text-brand-900" : "text-brand-100 hover:text-white"
          }`}
        >
          {labels[l]}
        </button>
      ))}
    </div>
  );
}
