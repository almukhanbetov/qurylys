"use client";

import type { ReactNode } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { useApiQuery } from "@/hooks/useApiQuery";
import { UNITS, type CardFields, type GtinConfirmation, GTIN_SOURCE_KINDS, type GtinSourceKind, type Unit } from "@/types/api";
import { Field } from "@/components/ui/Field";
import { controlClass, flattenCategories } from "@/features/catalog-requests/parts";
import { BrandPicker } from "./BrandPicker";
import { Select } from "./parts";

/**
 * Fields of a shared card. `locked` disables what identifies the product when
 * suppliers already offer it (N3); the API enforces the same rules.
 */
export function CardFieldsForm({
  value: f,
  brandName,
  onChange,
  locked = false,
  disabled = false,
}: {
  value: CardFields;
  brandName: string | null;
  onChange: (f: CardFields, brandName?: string | null) => void;
  locked?: boolean;
  disabled?: boolean;
}) {
  const { t, locale } = useI18n();
  const c = t.catalogRequests;
  const tree = useApiQuery("category-tree", (signal) => api.categoryTree(signal));
  const categories = tree.data ? flattenCategories(tree.data, locale) : [];
  const set = (patch: Partial<CardFields>) => onChange({ ...f, ...patch });

  return (
    <fieldset disabled={disabled} className="grid gap-3 sm:grid-cols-2">
      <Select label={c.category} value={f.category_id ? String(f.category_id) : ""} disabled={locked} onChange={(v) => set({ category_id: Number(v) })}>
        <option value="" disabled>{c.choose}</option>
        {categories.some((x) => x.id === f.category_id) || !f.category_id ? null : <option value={f.category_id}>#{f.category_id}</option>}
        {categories.map((x) => (
          <option key={x.id} value={x.id}>{x.label}</option>
        ))}
      </Select>
      <Select label={c.unit} value={f.unit} disabled={locked} onChange={(v) => set({ unit: v as Unit })}>
        {UNITS.map((u) => (
          <option key={u} value={u}>{t.units[u]}</option>
        ))}
      </Select>
      <Field label={c.nameKk} value={f.name_kk} maxLength={200} required onChange={(e) => set({ name_kk: e.target.value })} />
      <Field label={c.nameRu} value={f.name_ru} maxLength={200} required onChange={(e) => set({ name_ru: e.target.value })} />
      <div className="sm:col-span-2">
        <BrandPicker
          label={c.brand}
          value={f.brand_id}
          current={brandName}
          allowNone
          disabled={locked && f.brand_id !== null}
          onChange={(id, name) => onChange({ ...f, brand_id: id }, name)}
        />
      </div>
      {(["description_kk", "description_ru"] as const).map((k) => (
        <TextArea key={k} label={k === "description_kk" ? c.descriptionKk : c.descriptionRu} value={f[k]} onChange={(v) => set({ [k]: v })} />
      ))}
    </fieldset>
  );
}

export function TextArea({ label, value, onChange, rows = 4, required, children }: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  rows?: number;
  required?: boolean;
  children?: ReactNode;
}) {
  return (
    <label className="block min-w-0 text-sm font-medium">
      {label}
      <textarea rows={rows} maxLength={2000} required={required} value={value} onChange={(e) => onChange(e.target.value)} className={`${controlClass} py-2`} />
      {children}
    </label>
  );
}

export function GtinFields({ value: g, onChange, gtinLabel }: { value: GtinConfirmation; onChange: (g: GtinConfirmation) => void; gtinLabel?: string }) {
  const { t } = useI18n();
  const c = t.catalogRequests;
  return (
    <div className="grid gap-3 sm:grid-cols-2">
      <Field label={gtinLabel ?? c.gtin} hint={c.gtinHint} value={g.gtin} inputMode="numeric" maxLength={20} required onChange={(e) => onChange({ ...g, gtin: e.target.value })} />
      <Select label={c.sourceKind} value={g.source_kind} onChange={(v) => onChange({ ...g, source_kind: v as GtinSourceKind })}>
        {GTIN_SOURCE_KINDS.map((k) => (
          <option key={k} value={k}>{c.sources[k]}</option>
        ))}
      </Select>
      <div className="sm:col-span-2">
        <Field label={c.source} hint={c.confirmHint} value={g.source} maxLength={500} required onChange={(e) => onChange({ ...g, source: e.target.value })} />
      </div>
    </div>
  );
}
