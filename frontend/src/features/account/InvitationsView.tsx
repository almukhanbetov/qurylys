"use client";

import { accountApi } from "@/lib/api";
import { hardNavigate } from "@/lib/redirect";
import { useI18n } from "@/lib/i18n/client";
import { format } from "@/lib/i18n";
import { formatDate } from "@/lib/format";
import { useApiQuery } from "@/hooks/useApiQuery";
import { useSubmit } from "@/hooks/useSubmit";
import { Button } from "@/components/ui/Button";
import { Alert, Card } from "@/components/ui/Field";
import { EmptyState } from "@/components/ui/StateMessage";
import { QueryState } from "./QueryState";

export function InvitationsView() {
  const { t, locale } = useI18n();
  const result = useApiQuery("my-invitations", (signal) => accountApi.invitations(signal));
  const { pending, error, run } = useSubmit();
  const answer = (id: number, accept: boolean) =>
    void run(async () => {
      await accountApi.answerInvitation(id, accept);
      hardNavigate("/account");
    });

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">{t.account.invitations}</h1>
      {error ? <Alert>{error}</Alert> : null}
      <QueryState result={result}>
        {(list) =>
          list.length === 0 ? (
            <EmptyState title={t.account.invitationsEmpty} />
          ) : (
            <div className="space-y-3">
              {list.map((i) => (
                <Card key={i.id} title={i.company_name}>
                  <p className="text-sm text-ink-600 wrap-anywhere">
                    {i.bin ? `${t.account.bin}: ${i.bin} · ` : ""}
                    {i.invited_by ? `${format(t.account.invitedBy, { name: i.invited_by })} · ` : ""}
                    {format(t.account.until, { date: formatDate(i.expires_at, locale) })}
                  </p>
                  <div className="mt-4 flex gap-2">
                    <Button disabled={pending} onClick={() => answer(i.id, true)}>{t.account.accept}</Button>
                    <Button variant="secondary" disabled={pending} onClick={() => answer(i.id, false)}>{t.account.decline}</Button>
                  </div>
                </Card>
              ))}
            </div>
          )
        }
      </QueryState>
    </div>
  );
}
