import { kk } from "./dictionaries/kk";
import { ru } from "./dictionaries/ru";
import type { Dictionary, Locale } from "./types";

export * from "./types";

const dictionaries: Record<Locale, Dictionary> = { kk, ru };

export function getDictionary(locale: Locale): Dictionary {
  return dictionaries[locale];
}

/** Replaces {name} placeholders. Unknown placeholders are left as is. */
export function format(template: string, values: Record<string, string | number>): string {
  return template.replace(/\{(\w+)\}/g, (match, key: string) =>
    key in values ? String(values[key]) : match,
  );
}

/** Picks the localized variant of a bilingual API field. */
export function pick(locale: Locale, kkValue: string, ruValue: string): string {
  return locale === "ru" ? ruValue || kkValue : kkValue || ruValue;
}
