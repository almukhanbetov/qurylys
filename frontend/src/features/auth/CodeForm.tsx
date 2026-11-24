"use client";

import { useState, type ReactNode } from "react";
import { useI18n } from "@/lib/i18n/client";
import { format } from "@/lib/i18n";
import { useSubmit } from "@/hooks/useSubmit";
import { Button } from "@/components/ui/Button";
import { Alert, Field } from "@/components/ui/Field";

/** Second step of every flow that confirms a contact. */
export function CodeForm({
  contact,
  onConfirm,
  onBack,
  children,
}: {
  contact: string;
  onConfirm: (code: string) => Promise<void>;
  onBack: () => void;
  children?: ReactNode;
}) {
  const { t } = useI18n();
  const { pending, error, run } = useSubmit();
  const [code, setCode] = useState("");

  return (
    <form
      className="space-y-4"
      onSubmit={(e) => {
        e.preventDefault();
        void run(() => onConfirm(code));
      }}
    >
      <p className="text-sm text-ink-600 wrap-anywhere">{format(t.auth.codeText, { contact })}</p>
      <Field
        label={t.auth.code}
        value={code}
        onChange={(e) => setCode(e.target.value.replace(/\D/g, ""))}
        inputMode="numeric"
        autoComplete="one-time-code"
        maxLength={6}
        required
        autoFocus
      />
      {children}
      {error ? <Alert>{error}</Alert> : null}
      <div className="flex gap-3">
        <Button type="submit" disabled={pending} className="flex-1">
          {pending ? t.common.loading : t.auth.confirm}
        </Button>
        <Button variant="secondary" onClick={onBack} type="button">
          {t.auth.back}
        </Button>
      </div>
    </form>
  );
}
