"use client";

import type { ReactNode } from "react";
import { useI18n } from "@/lib/i18n/client";
import { Button } from "@/components/ui/Button";
import { Field } from "@/components/ui/Field";

export const selectClass =
  "mt-1 h-11 w-full rounded-xl border border-ink-300 bg-white px-3 text-base text-ink-900 focus:border-brand-700 focus:outline-none focus:ring-2 focus:ring-brand-200 disabled:bg-ink-50";

export function Select({
  label,
  value,
  onChange,
  children,
  disabled,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  children: ReactNode;
  disabled?: boolean;
}) {
  return (
    <label className="block min-w-0 text-sm font-medium text-ink-900">
      {label}
      <select value={value} disabled={disabled} onChange={(e) => onChange(e.target.value)} className={selectClass}>
        {children}
      </select>
    </label>
  );
}

/** Search box that applies on submit; filters are rendered next to it. */
export function SearchBar({
  label,
  initial,
  onSearch,
  children,
}: {
  label: string;
  initial: string;
  onSearch: (q: string) => void;
  children?: ReactNode;
}) {
  const { t } = useI18n();
  return (
    <form
      className="grid gap-3 rounded-2xl border border-ink-200 bg-white p-4 sm:grid-cols-2 lg:grid-cols-4 lg:items-end"
      onSubmit={(e) => {
        e.preventDefault();
        const q = new FormData(e.currentTarget).get("q");
        onSearch(typeof q === "string" ? q.trim() : "");
      }}
    >
      <div className="min-w-0 sm:col-span-2">
        <Field label={label} name="q" defaultValue={initial} maxLength={100} />
      </div>
      {children}
      <div className="flex gap-2 sm:col-span-2 lg:col-span-1">
        <Button type="submit" className="h-11 flex-1">{t.admin.search}</Button>
      </div>
    </form>
  );
}
