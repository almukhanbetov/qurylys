"use client";

import { useState } from "react";
import { useI18n } from "@/lib/i18n/client";
import { useSubmit } from "@/hooks/useSubmit";
import type { CompanyDetails } from "@/types/api";
import { Button } from "@/components/ui/Button";
import { Alert, Field } from "@/components/ui/Field";

export function CompanyForm({
  initial,
  disabled = false,
  submitLabel,
  onSubmit,
}: {
  initial: CompanyDetails;
  disabled?: boolean;
  submitLabel: string;
  onSubmit: (d: CompanyDetails) => Promise<void>;
}) {
  const { t } = useI18n();
  const { pending, error, run } = useSubmit();
  const [d, setD] = useState(initial);
  const [saved, setSaved] = useState(false);

  function set(patch: Partial<CompanyDetails>) {
    setSaved(false);
    setD((prev) => ({ ...prev, ...patch }));
  }

  const a = t.account;
  const text: { key: Exclude<keyof CompanyDetails, "is_buyer" | "is_supplier">; label: string; max: number; tel?: boolean }[] = [
    { key: "name", label: a.name, max: 200 },
    { key: "bin", label: a.bin, max: 12 },
    { key: "city", label: a.city, max: 100 },
    { key: "work_phone", label: a.workPhone, max: 24, tel: true },
    { key: "legal_address", label: a.legalAddress, max: 300 },
    { key: "actual_address", label: a.actualAddress, max: 300 },
    { key: "representative_name", label: a.representative, max: 120 },
  ];

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        void run(() => onSubmit(d)).then(setSaved);
      }}
    >
      <fieldset disabled={disabled || pending} className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-2">
          {text.map((f) => (
            <Field
              key={f.key}
              name={f.key}
              label={f.label}
              value={d[f.key]}
              maxLength={f.max}
              inputMode={f.key === "bin" ? "numeric" : f.tel ? "tel" : undefined}
              onChange={(e) => set({ [f.key]: e.target.value })}
              required
            />
          ))}
        </div>
        <fieldset>
          <legend className="text-sm font-medium">{a.companyType}</legend>
          <div className="mt-2 flex flex-wrap gap-4">
            <label className="flex min-h-11 items-center gap-2 text-sm">
              <input type="checkbox" className="h-5 w-5 accent-brand-700" checked={d.is_buyer} onChange={(e) => set({ is_buyer: e.target.checked })} />
              {a.buyer}
            </label>
            <label className="flex min-h-11 items-center gap-2 text-sm">
              <input type="checkbox" className="h-5 w-5 accent-brand-700" checked={d.is_supplier} onChange={(e) => set({ is_supplier: e.target.checked })} />
              {a.supplier}
            </label>
          </div>
        </fieldset>
        {error ? <Alert>{error}</Alert> : null}
        {saved ? <Alert tone="success">{a.saved}</Alert> : null}
        {disabled ? null : <Button type="submit">{pending ? t.common.loading : submitLabel}</Button>}
      </fieldset>
    </form>
  );
}
