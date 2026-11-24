import type { Money } from "@/types/api";
import type { Locale } from "./i18n/types";

const groupFormatter = new Intl.NumberFormat("ru-RU", { useGrouping: true, maximumFractionDigits: 0 });

/**
 * Formats integer tiyn as tenge without floating-point arithmetic on the
 * fractional part: 275000 -> "2 750 ₸", 275050 -> "2 750,50 ₸".
 * The same grouping (space) and decimal comma are used in Kazakh and Russian.
 */
export function formatMoney(m: Money): string {
  const negative = m.amount_tiyn < 0;
  const abs = Math.abs(m.amount_tiyn);
  const tenge = Math.trunc(abs / 100);
  const tiyn = abs % 100;
  const whole = groupFormatter.format(tenge).replace(/ | /g, " ");
  const frac = tiyn === 0 ? "" : "," + String(tiyn).padStart(2, "0");
  return `${negative ? "−" : ""}${whole}${frac} ₸`;
}

/** Formats an exact decimal string from the API ("12.750" -> "12,75"). */
export function formatQuantity(value: string): string {
  const [intPart = "0", fracPart = ""] = value.split(".");
  const trimmed = fracPart.replace(/0+$/, "");
  const grouped = intPart.replace(/\B(?=(\d{3})+(?!\d))/g, " ");
  return trimmed ? `${grouped},${trimmed}` : grouped;
}

export function htmlLang(locale: Locale): string {
  return locale === "ru" ? "ru" : "kk";
}

/** Date and time in Kazakhstan, independent of the visitor's device settings. */
export function formatDate(iso: string, locale: Locale): string {
  return new Intl.DateTimeFormat(locale === "ru" ? "ru-RU" : "kk-KZ", {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone: "Asia/Almaty",
  }).format(new Date(iso));
}
