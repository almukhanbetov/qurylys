import type { kk } from "./dictionaries/kk";

export const LOCALES = ["kk", "ru"] as const;
export type Locale = (typeof LOCALES)[number];
export const DEFAULT_LOCALE: Locale = "kk";
export const LOCALE_COOKIE = "qurylys_locale";

type DeepString<T> = { [K in keyof T]: T[K] extends string ? string : DeepString<T[K]> };

/** Shape of every dictionary, derived from the Kazakh reference. */
export type Dictionary = DeepString<typeof kk>;

export function isLocale(v: unknown): v is Locale {
  return typeof v === "string" && (LOCALES as readonly string[]).includes(v);
}
