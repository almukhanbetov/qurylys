"use client";

import { authApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { useApiQuery } from "@/hooks/useApiQuery";
import { Alert, Field } from "@/components/ui/Field";

/**
 * Contact that will receive a confirmation code. Offers only the channels the
 * API can use; the API enforces the same rule.
 */
export function ContactField({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const { t } = useI18n();
  const { data } = useApiQuery("auth-options", (signal) => authApi.options(signal));
  const emailOnly = data?.email === true && !data.phone;

  return (
    <>
      {data && !data.email && !data.phone ? <Alert tone="info">{t.errors.delivery_unavailable}</Alert> : null}
      <Field
        label={emailOnly ? t.auth.emailLabel : t.auth.contact}
        hint={t.auth.contactHint}
        type={emailOnly ? "email" : "text"}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        autoComplete={emailOnly ? "email" : "username"}
        maxLength={254}
        required
      />
    </>
  );
}
