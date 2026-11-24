"use client";

import Link from "next/link";
import { useState } from "react";
import { authApi } from "@/lib/api";
import { hardNavigate } from "@/lib/redirect";
import { useI18n } from "@/lib/i18n/client";
import { useSubmit } from "@/hooks/useSubmit";
import { Button } from "@/components/ui/Button";
import { Alert, Field } from "@/components/ui/Field";
import { CodeForm } from "./CodeForm";
import { ContactField } from "./ContactField";

export function RegisterForm() {
  const { t } = useI18n();
  const { pending, error, run } = useSubmit();
  const [fullName, setFullName] = useState("");
  const [contact, setContact] = useState("");
  const [password, setPassword] = useState("");
  const [challenge, setChallenge] = useState<string | null>(null);

  if (challenge) {
    return (
      <CodeForm
        contact={contact}
        onBack={() => setChallenge(null)}
        onConfirm={async (code) => {
          await authApi.confirmRegistration({ challenge, code });
          hardNavigate("/account");
        }}
      />
    );
  }

  return (
    <form
      className="space-y-4"
      onSubmit={(e) => {
        e.preventDefault();
        void run(async () => {
          const r = await authApi.register({ full_name: fullName, contact, password });
          setChallenge(r.challenge);
        });
      }}
    >
      <Field label={t.auth.fullName} value={fullName} onChange={(e) => setFullName(e.target.value)} autoComplete="name" maxLength={120} required />
      <ContactField value={contact} onChange={setContact} />
      <Field
        label={t.auth.password}
        hint={t.auth.passwordHint}
        type="password"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        autoComplete="new-password"
        minLength={10}
        maxLength={128}
        required
      />
      <Alert tone="info">{t.auth.warning}</Alert>
      {error ? <Alert>{error}</Alert> : null}
      <Button type="submit" disabled={pending} className="w-full">
        {pending ? t.common.loading : t.auth.sendCode}
      </Button>
      <p className="text-sm">
        {t.auth.haveAccount}{" "}
        <Link href="/login" className="font-semibold text-brand-700 hover:underline">{t.auth.login}</Link>
      </p>
    </form>
  );
}
