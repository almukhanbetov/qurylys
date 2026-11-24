"use client";

import { useState } from "react";
import { companyApi } from "@/lib/api";
import { hardNavigate } from "@/lib/redirect";
import { useI18n } from "@/lib/i18n/client";
import { format } from "@/lib/i18n";
import { formatDate } from "@/lib/format";
import { useApiQuery } from "@/hooks/useApiQuery";
import { useSubmit } from "@/hooks/useSubmit";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Alert, Card, Field } from "@/components/ui/Field";
import { NoCompany } from "./CompanyView";
import { useActiveCompany } from "./MeContext";
import { QueryState } from "./QueryState";

export function MembersView() {
  const { t } = useI18n();
  const active = useActiveCompany();
  if (!active) return <NoCompany />;
  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">{t.account.members}</h1>
      <Members id={active.id} owner={active.role === "owner"} />
      {active.role === "owner" ? <Invitations id={active.id} /> : null}
    </div>
  );
}

function Members({ id, owner }: { id: number; owner: boolean }) {
  const { t } = useI18n();
  const result = useApiQuery(`members:${id}`, (signal) => companyApi.members(id, signal));
  const { pending, error, run } = useSubmit();
  const act = (fn: () => Promise<void>) => void run(fn).then((ok) => ok && result.retry());

  return (
    <Card>
      {error ? <div className="mb-4"><Alert>{error}</Alert></div> : null}
      <QueryState result={result}>
        {(members) => (
          <ul className="divide-y divide-ink-100">
            {members.map((m) => (
              <li key={m.user_id} className="flex flex-wrap items-center justify-between gap-3 py-3">
                <div className="min-w-0">
                  <p className="font-semibold wrap-anywhere">
                    {m.full_name} {m.is_me ? <span className="font-normal text-ink-500">({t.account.you})</span> : null}
                  </p>
                  <p className="text-sm text-ink-500 wrap-anywhere">{[m.email, m.phone].filter(Boolean).join(" · ")}</p>
                  <Badge tone={m.role === "owner" ? "brand" : "muted"}>{t.account[m.role]}</Badge>
                </div>
                <div className="flex flex-wrap gap-2">
                  {owner && !m.is_me ? (
                    <Button
                      variant="secondary"
                      disabled={pending}
                      onClick={() => act(() => companyApi.setRole(id, m.user_id, m.role === "owner" ? "employee" : "owner"))}
                    >
                      {m.role === "owner" ? t.account.makeEmployee : t.account.makeOwner}
                    </Button>
                  ) : null}
                  {owner || m.is_me ? (
                    <Button
                      variant="ghost"
                      disabled={pending}
                      onClick={() => {
                        if (!window.confirm(m.is_me ? t.account.confirmLeave : t.account.confirmRemove)) return;
                        act(async () => {
                          await companyApi.removeMember(id, m.user_id);
                          if (m.is_me) hardNavigate("/account");
                        });
                      }}
                    >
                      {m.is_me ? t.account.leave : t.account.remove}
                    </Button>
                  ) : null}
                </div>
              </li>
            ))}
          </ul>
        )}
      </QueryState>
    </Card>
  );
}

function Invitations({ id }: { id: number }) {
  const { t, locale } = useI18n();
  const result = useApiQuery(`invitations:${id}`, (signal) => companyApi.invitations(id, signal));
  const { pending, error, run } = useSubmit();
  const [contact, setContact] = useState("");

  return (
    <Card title={t.account.pending}>
      <form
        className="space-y-3"
        onSubmit={(e) => {
          e.preventDefault();
          void run(() => companyApi.invite(id, contact)).then((ok) => {
            if (!ok) return;
            setContact("");
            result.retry();
          });
        }}
      >
        <Field label={t.auth.contact} hint={t.account.inviteText} value={contact} onChange={(e) => setContact(e.target.value)} required />
        {error ? <Alert>{error}</Alert> : null}
        <Button type="submit" disabled={pending}>{t.account.invite}</Button>
      </form>
      <div className="mt-4">
        <QueryState result={result}>
          {(list) =>
            list.length === 0 ? (
              <p className="text-sm text-ink-500">{t.account.pendingEmpty}</p>
            ) : (
              <ul className="divide-y divide-ink-100">
                {list.map((i) => (
                  <li key={i.id} className="flex flex-wrap items-center justify-between gap-2 py-2">
                    <span className="min-w-0 text-sm wrap-anywhere">
                      {i.contact} <span className="text-ink-500">{format(t.account.until, { date: formatDate(i.expires_at, locale) })}</span>
                    </span>
                    <Button
                      variant="ghost"
                      disabled={pending}
                      onClick={() => void run(() => companyApi.cancelInvitation(id, i.id)).then((ok) => ok && result.retry())}
                    >
                      {t.account.cancel}
                    </Button>
                  </li>
                ))}
              </ul>
            )
          }
        </QueryState>
      </div>
    </Card>
  );
}
