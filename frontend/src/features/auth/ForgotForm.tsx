"use client";

import { useState } from "react";
import { authApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { useSubmit } from "@/hooks/useSubmit";
import { Button, ButtonLink } from "@/components/ui/Button";
import { Alert, Field } from "@/components/ui/Field";
import { CodeForm } from "./CodeForm";
import { ContactField } from "./ContactField";

export function ForgotForm() {
  const { t } = useI18n();
  const { pending, error, run } = useSubmit();
  const [contact, setContact] = useState("");
  const [password, setPassword] = useState("");
  const [challenge, setChallenge] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  if (done) {
    return (
      <div className="space-y-4">
        <Alert tone="success">{t.auth.resetDone}</Alert>
        <ButtonLink href="/login" className="w-full">{t.auth.login}</ButtonLink>
      </div>
    );
  }

  if (challenge) {
    return (
      <CodeForm
        contact={contact}
        onBack={() => setChallenge(null)}
        onConfirm={async (code) => {
          await authApi.reset({ challenge, code, password });
          setDone(true);
        }}
      >
        <Field
          label={t.auth.newPassword}
          hint={t.auth.passwordHint}
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete="new-password"
          minLength={10}
          maxLength={128}
          required
        />
      </CodeForm>
    );
  }

  return (
    <form
      className="space-y-4"
      onSubmit={(e) => {
        e.preventDefault();
        void run(async () => setChallenge((await authApi.forgot(contact)).challenge));
      }}
    >
      <ContactField value={contact} onChange={setContact} />
      {error ? <Alert>{error}</Alert> : null}
      <Button type="submit" disabled={pending} className="w-full">
        {pending ? t.common.loading : t.auth.sendCode}
      </Button>
    </form>
  );
}
