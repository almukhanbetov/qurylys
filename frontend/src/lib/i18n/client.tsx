"use client";

import { createContext, useContext, type ReactNode } from "react";
import { getDictionary, type Dictionary, type Locale } from "./index";

const I18nContext = createContext<{ locale: Locale; t: Dictionary } | null>(null);

/**
 * Receives the locale from the server layout. When the language switcher
 * changes the cookie and refreshes the route, the layout re-renders with the
 * new locale and this provider passes the new dictionary down.
 */
export function I18nProvider({ locale, children }: { locale: Locale; children: ReactNode }) {
  return <I18nContext.Provider value={{ locale, t: getDictionary(locale) }}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error("useI18n must be used inside I18nProvider");
  return ctx;
}
