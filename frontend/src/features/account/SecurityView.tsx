"use client";

import { useState } from "react";
import { accountApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { format } from "@/lib/i18n";
import { formatDate } from "@/lib/format";
import { useApiQuery } from "@/hooks/useApiQuery";
import { useSubmit } from "@/hooks/useSubmit";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Alert, Card, Field } from "@/components/ui/Field";
import { QueryState } from "./QueryState";

export function SecurityView() {
  const { t, locale } = useI18n();
  const sessions = useApiQuery("sessions", (signal) => accountApi.sessions(signal));
  const password = useSubmit();
  const ending = useSubmit();
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [changed, setChanged] = useState(false);
  const end = (fn: () => Promise<void>) => void ending.run(fn).then((ok) => ok && sessions.retry());

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">{t.account.security}</h1>
      <Card title={t.account.changePassword}>
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            setChanged(false);
            void password.run(() => accountApi.changePassword(current, next)).then((ok) => {
              if (!ok) return;
              setChanged(true);
              setCurrent("");
              setNext("");
              sessions.retry();
            });
          }}
        >
          <Field label={t.account.currentPassword} type="password" autoComplete="current-password" value={current} onChange={(e) => setCurrent(e.target.value)} required />
          <Field label={t.auth.newPassword} hint={t.auth.passwordHint} type="password" autoComplete="new-password" minLength={10} maxLength={128} value={next} onChange={(e) => setNext(e.target.value)} required />
          {password.error ? <Alert>{password.error}</Alert> : null}
          {changed ? <Alert tone="success">{t.account.passwordChanged}</Alert> : null}
          <Button type="submit" disabled={password.pending}>{t.account.changePassword}</Button>
        </form>
      </Card>
      <Card title={t.account.sessions}>
        {ending.error ? <div className="mb-3"><Alert>{ending.error}</Alert></div> : null}
        <QueryState result={sessions}>
          {(list) => (
            <>
              <ul className="divide-y divide-ink-100">
                {list.map((s) => (
                  <li key={s.id} className="flex flex-wrap items-center justify-between gap-2 py-3">
                    <div className="min-w-0 flex-1 basis-60 text-sm">
                      <p className="wrap-anywhere">{s.user_agent || t.account.unknownDevice}</p>
                      <p className="text-ink-500">{format(t.account.lastSeen, { date: formatDate(s.last_seen_at, locale) })}</p>
                    </div>
                    {s.current ? (
                      <Badge>{t.account.currentSession}</Badge>
                    ) : (
                      <Button variant="ghost" disabled={ending.pending} onClick={() => end(() => accountApi.endSession(s.id))}>
                        {t.account.endSession}
                      </Button>
                    )}
                  </li>
                ))}
              </ul>
              {list.length > 1 ? (
                <Button variant="secondary" disabled={ending.pending} onClick={() => end(() => accountApi.endOtherSessions())}>
                  {t.account.endOthers}
                </Button>
              ) : null}
            </>
          )}
        </QueryState>
      </Card>
    </div>
  );
}
