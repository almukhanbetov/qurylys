// Exact money and quantity helpers: integers and BigInt only, no floating point.
// The same cases are tested in Go (backend/internal/quantity) and in SQL.

/** "1500", "1 500,5" or "84.37" tenge -> integer tiyn; null when not exact. */
export function parseMoney(raw: string): number | null {
  const m = /^(\d{1,13})(?:[.,](\d{1,2}))?$/.exec(raw.replace(/[\s\u00a0\u202f]/g, ""));
  if (!m) return null;
  const tiyn = BigInt(m[1] ?? "0") * 100n + BigInt((m[2] ?? "").padEnd(2, "0"));
  return tiyn > 0n && tiyn <= BigInt(Number.MAX_SAFE_INTEGER) ? Number(tiyn) : null;
}

/** Tiyn -> text for an input: 115050 -> "1150,50". */
export function moneyInput(tiyn: number | null): string {
  if (tiyn === null) return "";
  const s = String(tiyn).padStart(3, "0");
  const frac = s.slice(-2);
  return s.slice(0, -2) + (frac === "00" ? "" : `,${frac}`);
}

/** "0,6" -> "0.6"; null when there are more than 3 fraction digits or it is not a number. */
export function parseQuantity(raw: string): string | null {
  const v = raw.replace(/[\s\u00a0\u202f]/g, "").replace(",", ".");
  return /^\d{1,11}(\.\d{1,3})?$/.test(v) ? v : null;
}

function thousandths(q: string): bigint {
  const [whole = "0", frac = ""] = q.split(".");
  return BigInt(whole) * 1000n + BigInt(frac.padEnd(3, "0"));
}

/** price × quantity in tiyn, rounded half up once; null when it does not fit. */
export function lineTotal(priceTiyn: number, quantity: string): number | null {
  const total = (BigInt(priceTiyn) * thousandths(quantity) + 500n) / 1000n;
  return total <= BigInt(Number.MAX_SAFE_INTEGER) ? Number(total) : null;
}
