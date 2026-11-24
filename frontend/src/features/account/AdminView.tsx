"use client";

import { useState } from "react";
import { adminApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { useApiQuery } from "@/hooks/useApiQuery";
import { useSubmit } from "@/hooks/useSubmit";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Alert, Card, Field } from "@/components/ui/Field";
import { EmptyState } from "@/components/ui/StateMessage";
import { QueryState } from "./QueryState";

export function AdminView() {
  const { t } = useI18n();
  const [input, setInput] = useState("");
  const [q, setQ] = useState("");
  const result = useApiQuery(`admin-users:${q}`, (signal) => adminApi.users(q, signal));
  const { pending, error, run } = useSubmit();

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">{t.account.admin}</h1>
      <p className="text-sm text-ink-600">{t.account.adminText}</p>
      <Card>
        <form
          className="flex flex-wrap items-end gap-3"
          onSubmit={(e) => {
            e.preventDefault();
            setQ(input.trim());
          }}
        >
          <div className="min-w-0 flex-1 basis-60">
            <Field label={t.account.searchUsers} value={input} maxLength={60} onChange={(e) => setInput(e.target.value)} />
          </div>
          <Button type="submit" className="h-11">{t.header.searchButton}</Button>
        </form>
        {error ? <div className="mt-4"><Alert>{error}</Alert></div> : null}
        <div className="mt-4">
          <QueryState result={result}>
            {(users) =>
              users.length === 0 ? (
                <EmptyState title={t.account.usersEmpty} />
              ) : (
                <ul className="divide-y divide-ink-100">
                  {users.map((u) => {
                    const mrx = u.platform_roles.includes("mrx");
                    return (
                      <li key={u.id} className="flex flex-wrap items-center justify-between gap-3 py-3">
                        <div className="min-w-0">
                          <p className="font-semibold wrap-anywhere">{u.full_name}</p>
                          <p className="text-sm text-ink-500 wrap-anywhere">{[u.email, u.phone].filter(Boolean).join(" · ")}</p>
                          <p className="flex flex-wrap gap-2">
                            {u.platform_roles.map((r) => (
                              <Badge key={r}>{r === "mrx" ? t.account.mrx : t.account.adminRole}</Badge>
                            ))}
                          </p>
                        </div>
                        <Button
                          variant="secondary"
                          disabled={pending}
                          onClick={() => void run(() => adminApi.setMrx(u.id, !mrx)).then((ok) => ok && result.retry())}
                        >
                          {mrx ? t.account.revokeMrx : t.account.grantMrx}
                        </Button>
                      </li>
                    );
                  })}
                </ul>
              )
            }
          </QueryState>
        </div>
      </Card>
    </div>
  );
}
