"use client";

import { useState } from "react";
import { brandApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { useApiQuery } from "@/hooks/useApiQuery";
import { selectClass } from "./parts";

/**
 * Chooses a brand from the directory with a search box. `value` is the chosen
 * id; `current` is its name, shown even when it is not among the results.
 */
export function BrandPicker({
  label,
  value,
  current,
  onChange,
  allowNone,
  disabled,
}: {
  label: string;
  value: number | null;
  current?: string | null;
  onChange: (id: number | null, name: string | null) => void;
  allowNone?: boolean;
  disabled?: boolean;
}) {
  const { t } = useI18n();
  const [q, setQ] = useState("");
  const result = useApiQuery(`brand-picker:${q}`, (signal) => brandApi.list(q, 1, signal));
  const items = result.data?.items ?? [];
  const known = value === null || items.some((b) => b.id === value);

  return (
    <fieldset className="min-w-0 text-sm font-medium text-ink-900" disabled={disabled}>
      <legend>{label}</legend>
      <div className="mt-1 grid gap-2 sm:grid-cols-2">
        <input
          type="search"
          value={q}
          maxLength={100}
          placeholder={t.admin.brands.searchLabel}
          aria-label={t.admin.brands.searchLabel}
          onChange={(e) => setQ(e.target.value)}
          className={selectClass}
        />
        <select
          value={value === null ? "" : String(value)}
          aria-label={label}
          onChange={(e) => {
            const id = e.target.value ? Number(e.target.value) : null;
            onChange(id, items.find((b) => b.id === id)?.name ?? null);
          }}
          className={selectClass}
        >
          {allowNone || value === null ? <option value="">{allowNone ? t.admin.products.brandNone : t.catalogRequests.choose}</option> : null}
          {!known ? <option value={String(value)}>{current ?? `#${value}`}</option> : null}
          {items.map((b) => (
            <option key={b.id} value={b.id}>
              {b.name}
            </option>
          ))}
        </select>
      </div>
      {result.status === "error" ? <p className="mt-1 text-xs text-red-700">{t.common.errorText}</p> : null}
    </fieldset>
  );
}
