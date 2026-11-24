"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { accountApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { useSubmit } from "@/hooks/useSubmit";
import { Button } from "@/components/ui/Button";
import { Alert, Card, Field } from "@/components/ui/Field";
import { CodeForm } from "@/features/auth/CodeForm";
import { ContactField } from "@/features/auth/ContactField";
import { useMe } from "./MeContext";

export function ProfileView() {
  const { t } = useI18n();
  const me = useMe();
  const router = useRouter();
  const { pending, error, run } = useSubmit();
  const [name, setName] = useState(me.full_name);
  const [saved, setSaved] = useState(false);

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">{t.account.profile}</h1>
      <Card>
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            void run(() => accountApi.updateProfile(name)).then((ok) => {
              setSaved(ok);
              if (ok) router.refresh();
            });
          }}
        >
          <Field label={t.auth.fullName} value={name} maxLength={120} onChange={(e) => { setSaved(false); setName(e.target.value); }} required />
          {error ? <Alert>{error}</Alert> : null}
          {saved ? <Alert tone="success">{t.account.saved}</Alert> : null}
          <Button type="submit" disabled={pending}>{t.account.save}</Button>
        </form>
      </Card>
      <Card title={t.account.contacts}>
        <dl className="grid gap-3 text-sm sm:grid-cols-2">
          <div><dt className="text-ink-500">{t.account.email}</dt><dd className="font-medium wrap-anywhere">{me.email ?? t.account.notSet}</dd></div>
          <div><dt className="text-ink-500">{t.account.phone}</dt><dd className="font-medium">{me.phone ?? t.account.notSet}</dd></div>
        </dl>
        <div className="mt-6"><ContactForm /></div>
      </Card>
    </div>
  );
}

function ContactForm() {
  const { t } = useI18n();
  const router = useRouter();
  const { pending, error, run } = useSubmit();
  const [contact, setContact] = useState("");
  const [password, setPassword] = useState("");
  const [challenge, setChallenge] = useState<string | null>(null);

  if (challenge) {
    return (
      <CodeForm
        contact={contact}
        onBack={() => setChallenge(null)}
        onConfirm={async (code) => {
          await accountApi.confirmContact({ challenge, code });
          setChallenge(null);
          setContact("");
          setPassword("");
          router.refresh();
        }}
      />
    );
  }
  return (
    <form
      className="space-y-4"
      onSubmit={(e) => {
        e.preventDefault();
        void run(async () => setChallenge((await accountApi.startContact(contact, password)).challenge));
      }}
    >
      <h3 className="font-semibold">{t.account.changeContact}</h3>
      <ContactField value={contact} onChange={setContact} />
      <Field label={t.account.currentPassword} type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} required />
      {error ? <Alert>{error}</Alert> : null}
      <Button type="submit" disabled={pending}>{t.auth.sendCode}</Button>
    </form>
  );
}
