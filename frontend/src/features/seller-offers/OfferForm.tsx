"use client";

import { useState } from "react";
import { ApiError } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { format } from "@/lib/i18n";
import { formatMoney } from "@/lib/format";
import { lineTotal, moneyInput, parseMoney, parseQuantity } from "@/lib/quantity";
import { useSubmit } from "@/hooks/useSubmit";
import { PRICE_LEVELS, type OfferLevel, type PriceLevel, type SaleMode, type SellerOfferInput, type Unit, type VatStatus } from "@/types/api";
import { Button } from "@/components/ui/Button";
import { Alert, Field } from "@/components/ui/Field";
import { controlClass } from "@/features/catalog-requests/parts";

type LevelText = { on: boolean; price: string; mode: SaleMode; min: string; step: string };
type FormState = {
  sku: string; vat: VatStatus; pack: string; stock: string; city: string;
  levels: Record<PriceLevel, LevelText>;
};

const show = (q: string) => q.replace(".", ",");

function levelText(l: OfferLevel | null): LevelText {
  return l
    ? { on: true, price: moneyInput(l.price_tiyn), mode: l.sale_mode, min: show(l.min_quantity), step: show(l.step_quantity) }
    : { on: false, price: "", mode: "pack", min: "", step: "" };
}

function toState(i: SellerOfferInput): FormState {
  return {
    sku: i.seller_sku, vat: i.vat_status, pack: show(i.pack_size), stock: show(i.stock_quantity), city: i.city,
    levels: { retail: levelText(i.retail), wholesale: levelText(i.wholesale), large_wholesale: levelText(i.large_wholesale) },
  };
}

const bad = (code: string) => new ApiError(400, code, code);

/** Text -> exact values. Business rules are checked by the API. */
function toInput(f: FormState): SellerOfferInput {
  const level = (name: PriceLevel): OfferLevel | null => {
    const l = f.levels[name];
    if (!l.on) return null;
    const price = l.price.trim() === "" ? null : parseMoney(l.price);
    if (l.price.trim() !== "" && price === null) throw bad(`invalid_${name}_price`);
    const min = parseQuantity(l.min);
    const step = parseQuantity(l.step);
    if (min === null || step === null) throw bad(`invalid_${name}_quantity`);
    return { price_tiyn: price, sale_mode: l.mode, min_quantity: min, step_quantity: step };
  };
  const pack = f.pack.trim() === "" ? "" : parseQuantity(f.pack);
  const stock = parseQuantity(f.stock);
  if (pack === null) throw bad("invalid_pack_size");
  if (stock === null) throw bad("invalid_stock_quantity");
  const retail = level("retail");
  if (!retail) throw bad("invalid_number");
  return {
    seller_sku: f.sku, vat_status: f.vat, pack_size: pack, stock_quantity: stock, city: f.city,
    retail, wholesale: level("wholesale"), large_wholesale: level("large_wholesale"),
  };
}

export function OfferForm({
  initial,
  unit,
  disabled = false,
  onSave,
  onDirty,
}: {
  initial: SellerOfferInput;
  unit: Unit;
  disabled?: boolean;
  onSave: (f: SellerOfferInput) => Promise<void>;
  onDirty?: (dirty: boolean) => void;
}) {
  const { t } = useI18n();
  const o = t.offers;
  const u = t.units[unit];
  const { pending, error, run } = useSubmit();
  const [f, setF] = useState(() => toState(initial));
  const set = (patch: Partial<FormState>) => {
    onDirty?.(true);
    setF((prev) => ({ ...prev, ...patch }));
  };
  const setLevel = (name: PriceLevel, patch: Partial<LevelText>) =>
    set({ levels: { ...f.levels, [name]: { ...f.levels[name], ...patch } } });

  const calc = (price: string, quantity: string) => {
    const p = parseMoney(price);
    const q = parseQuantity(quantity);
    const v = p !== null && q !== null ? lineTotal(p, q) : null;
    return v === null ? "—" : formatMoney({ amount_tiyn: v, currency: "KZT" });
  };

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        void run(async () => onSave(toInput(f)));
      }}
    >
      <fieldset disabled={disabled || pending} className="space-y-5">
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={o.sellerSku} name="seller_sku" value={f.sku} maxLength={64} onChange={(e) => set({ sku: e.target.value })} />
          <label className="min-w-0 text-sm font-medium">
            {o.vat}
            <select name="vat_status" value={f.vat} onChange={(e) => set({ vat: e.target.value as VatStatus })} className={controlClass}>
              <option value="">{t.catalogRequests.choose}</option>
              <option value="with_vat">{o.vatOptions.with_vat}</option>
              <option value="without_vat">{o.vatOptions.without_vat}</option>
            </select>
          </label>
          <Field label={format(o.packSize, { unit: u })} name="pack_size" inputMode="decimal" value={f.pack} onChange={(e) => set({ pack: e.target.value })} />
          <Field label={format(o.stock, { unit: u })} name="stock_quantity" inputMode="decimal" value={f.stock} onChange={(e) => set({ stock: e.target.value })} />
          <Field label={o.city} name="city" value={f.city} maxLength={100} onChange={(e) => set({ city: e.target.value })} />
        </div>
        <p className="text-sm text-ink-600">{o.priceNote}</p>
        {PRICE_LEVELS.map((name) => {
          const l = f.levels[name];
          return (
            <fieldset key={name} className="rounded-2xl border border-ink-200 p-4">
              <legend className="px-1 font-semibold">{o.levels[name]}</legend>
              {name === "retail" ? null : (
                <label className="flex min-h-11 items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    name={`${name}_on`}
                    className="h-5 w-5 accent-brand-700"
                    checked={l.on}
                    onChange={(e) => setLevel(name, { on: e.target.checked })}
                  />
                  {o.enable}
                </label>
              )}
              {l.on ? (
                <>
                  <div className="grid gap-4 sm:grid-cols-2">
                    <Field label={format(o.price, { unit: u })} name={`${name}_price`} inputMode="decimal" value={l.price} onChange={(e) => setLevel(name, { price: e.target.value })} />
                    <label className="min-w-0 text-sm font-medium">
                      {o.mode}
                      <select name={`${name}_mode`} value={l.mode} onChange={(e) => setLevel(name, { mode: e.target.value as SaleMode })} className={controlClass}>
                        <option value="piece">{o.modes.piece}</option>
                        <option value="pack">{o.modes.pack}</option>
                      </select>
                    </label>
                    <Field label={format(o.min, { unit: u })} name={`${name}_min`} inputMode="decimal" value={l.min} onChange={(e) => setLevel(name, { min: e.target.value })} />
                    <Field label={format(o.step, { unit: u })} name={`${name}_step`} inputMode="decimal" value={l.step} onChange={(e) => setLevel(name, { step: e.target.value })} />
                  </div>
                  <p className="mt-3 flex flex-wrap gap-x-6 gap-y-1 text-sm text-ink-600" data-level={name}>
                    <span>{o.packPrice}: <b>{calc(l.price, f.pack)}</b></span>
                    <span>{o.minTotal}: <b>{calc(l.price, l.min)}</b></span>
                  </p>
                </>
              ) : null}
            </fieldset>
          );
        })}
        {error ? <Alert>{error}</Alert> : null}
        {disabled ? null : <Button type="submit">{pending ? t.common.loading : o.save}</Button>}
      </fieldset>
    </form>
  );
}
