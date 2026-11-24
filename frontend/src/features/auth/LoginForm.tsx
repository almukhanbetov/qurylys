"use client";

import Link from "next/link";
import { useState } from "react";
import { authApi } from "@/lib/api";
import { hardNavigate } from "@/lib/redirect";
import { useI18n } from "@/lib/i18n/client";
import { useSubmit } from "@/hooks/useSubmit";
import { Button } from "@/components/ui/Button";
import { Alert, Field } from "@/components/ui/Field";

export function LoginForm({ next }: { next: string }) {
  const { t } = useI18n();
  const { pending, error, run } = useSubmit();
  const [contact, setContact] = useState("");
  const [password, setPassword] = useState("");

  return (
    <form
      className="space-y-4"
      onSubmit={(e) => {
        e.preventDefault();
        void run(async () => {
          await authApi.login({ contact, password });
          // Full load: nothing cached for the previous visitor survives.
          hardNavigate(next);
        });
      }}
    >
      <Field label={t.auth.contact} value={contact} onChange={(e) => setContact(e.target.value)} autoComplete="username" required />
      <Field
        label={t.auth.password}
        type="password"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        autoComplete="current-password"
        required
      />
      {error ? <Alert>{error}</Alert> : null}
      <Button type="submit" disabled={pending} className="w-full">
        {pending ? t.common.loading : t.auth.login}
      </Button>
      <div className="flex flex-wrap justify-between gap-2 text-sm">
        <Link href="/forgot" className="font-semibold text-brand-700 hover:underline">{t.auth.forgotLink}</Link>
        <span>
          {t.auth.noAccount}{" "}
          <Link href="/register" className="font-semibold text-brand-700 hover:underline">{t.auth.register}</Link>
        </span>
      </div>
    </form>
  );
}
