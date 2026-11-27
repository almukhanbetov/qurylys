"use client";

import { useState } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { useApiQuery } from "@/hooks/useApiQuery";
import { useSubmit } from "@/hooks/useSubmit";
import { GTIN_SOURCE_KINDS, UNITS, type CatalogRequestFields, type GtinSourceKind, type Unit } from "@/types/api";
import { Button } from "@/components/ui/Button";
import { Alert, Field } from "@/components/ui/Field";
import { QueryState } from "@/features/account/QueryState";
import { controlClass, flattenCategories } from "./parts";

export const emptyFields: CatalogRequestFields = {
  category_id: null, name_kk: "", name_ru: "", brand: "", unit: "", description_kk: "", description_ru: "",
  gtin: null, gtin_source_kind: null, gtin_source: "",
};

/** Saves a draft; "submit" saves and then sends it to a moderator. */
export function RequestForm({
  initial,
  onSave,
  onSubmit,
}: {
  initial: CatalogRequestFields;
  onSave: (f: CatalogRequestFields) => Promise<void>;
  onSubmit: (f: CatalogRequestFields) => Promise<void>;
}) {
  const { t, locale } = useI18n();
  const c = t.catalogRequests;
  const tree = useApiQuery("category-tree", (signal) => api.categoryTree(signal));
  const { pending, error, run } = useSubmit();
  const [f, setF] = useState(initial);
  const [saved, setSaved] = useState(false);
  const set = (patch: Partial<CatalogRequestFields>) => {
    setSaved(false);
    setF((prev) => ({ ...prev, ...patch }));
  };

  return (
    <QueryState result={tree}>
      {(nodes) => (
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            void run(() => onSave(f)).then(setSaved);
          }}
        >
          <fieldset disabled={pending} className="grid gap-4 sm:grid-cols-2">
            <Field label={c.nameKk} name="name_kk" value={f.name_kk} maxLength={200} onChange={(e) => set({ name_kk: e.target.value })} />
            <Field label={c.nameRu} name="name_ru" value={f.name_ru} maxLength={200} onChange={(e) => set({ name_ru: e.target.value })} />
            <label className="min-w-0 text-sm font-medium">
              {c.category}
              <select
                name="category_id"
                value={f.category_id ?? ""}
                onChange={(e) => set({ category_id: e.target.value ? Number(e.target.value) : null })}
                className={controlClass}
              >
                <option value="">{c.choose}</option>
                {flattenCategories(nodes, locale).map((x) => (
                  <option key={x.id} value={x.id}>{x.label}</option>
                ))}
              </select>
            </label>
            <label className="min-w-0 text-sm font-medium">
              {c.unit}
              <select name="unit" value={f.unit} onChange={(e) => set({ unit: e.target.value as Unit | "" })} className={controlClass}>
                <option value="">{c.choose}</option>
                {UNITS.map((u) => (
                  <option key={u} value={u}>{t.units[u]}</option>
                ))}
              </select>
            </label>
            <Field label={c.brand} name="brand" value={f.brand} maxLength={100} onChange={(e) => set({ brand: e.target.value })} />
            <Field
              label={c.gtin}
              hint={c.gtinHint}
              name="gtin"
              inputMode="numeric"
              value={f.gtin ?? ""}
              maxLength={20}
              onChange={(e) => set({ gtin: e.target.value || null })}
            />
            <label className="min-w-0 text-sm font-medium">
              {c.sourceKind}
              <select
                name="gtin_source_kind"
                value={f.gtin_source_kind ?? ""}
                onChange={(e) => set({ gtin_source_kind: (e.target.value || null) as GtinSourceKind | null })}
                className={controlClass}
              >
                <option value="">{c.choose}</option>
                {GTIN_SOURCE_KINDS.map((k) => (
                  <option key={k} value={k}>{c.sources[k]}</option>
                ))}
              </select>
            </label>
            <Field label={c.source} name="gtin_source" value={f.gtin_source} maxLength={500} onChange={(e) => set({ gtin_source: e.target.value })} />
            {(["description_kk", "description_ru"] as const).map((k) => (
              <label key={k} className="min-w-0 text-sm font-medium">
                {k === "description_kk" ? c.descriptionKk : c.descriptionRu}
                <textarea
                  name={k}
                  rows={4}
                  maxLength={2000}
                  value={f[k]}
                  onChange={(e) => set({ [k]: e.target.value })}
                  className={`${controlClass} py-2`}
                />
              </label>
            ))}
          </fieldset>
          {error ? <Alert>{error}</Alert> : null}
          {saved ? <Alert tone="success">{t.account.saved}</Alert> : null}
          <div className="flex flex-wrap gap-3">
            <Button
              type="button"
              disabled={pending}
              onClick={() => {
                if (window.confirm(c.confirmSubmit)) void run(() => onSubmit(f));
              }}
            >
              {c.submit}
            </Button>
            <Button type="submit" variant="secondary" disabled={pending}>{c.saveDraft}</Button>
          </div>
        </form>
      )}
    </QueryState>
  );
}
