"use client";

import Link from "next/link";
import { useI18n } from "@/lib/i18n/client";
import { format } from "@/lib/i18n";
import { Badge } from "@/components/ui/Badge";
import { ButtonLink } from "@/components/ui/Button";
import { Alert, Card } from "@/components/ui/Field";
import { useActiveCompany, useMe } from "./MeContext";
import { VerificationBadge } from "./VerificationParts";

export function Overview() {
  const { t } = useI18n();
  const me = useMe();
  const company = useActiveCompany();

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold wrap-anywhere">{format(t.account.hello, { name: me.full_name })}</h1>
      <p className="text-sm text-ink-600">{t.account.stageText}</p>
      {me.platform_roles.length > 0 ? (
        <p className="flex flex-wrap gap-2">
          {me.platform_roles.map((r) => (
            <Badge key={r}>{r === "mrx" ? t.account.mrx : t.account.adminRole}</Badge>
          ))}
        </p>
      ) : null}
      {me.pending_invitations > 0 ? (
        <Alert tone="success">
          <Link href="/account/invitations" className="font-semibold underline">
            {format(t.account.pendingInvitations, { count: me.pending_invitations })}
          </Link>
        </Alert>
      ) : null}
      {company ? (
        <Card title={company.name}>
          <p className="flex flex-wrap gap-2">
            {company.is_buyer ? <Badge>{t.account.buyer}</Badge> : null}
            {company.is_supplier ? <Badge>{t.account.supplier}</Badge> : null}
            {company.status === "suspended" ? <Badge tone="danger">{t.account.suspended}</Badge> : null}
            <VerificationBadge status={company.verification} />
          </p>
          <p className="mt-3 text-sm text-ink-600">
            {t.account.yourRole}: {t.account[company.role]}
          </p>
          <p className="mt-1 text-sm text-ink-600">{t.verification.hint[company.verification]}</p>
          <Link href="/account/verification" className="mt-3 inline-block text-sm font-semibold text-brand-700 hover:underline">
            {t.verification.title}
          </Link>
        </Card>
      ) : (
        <Card title={me.companies.length ? t.account.chooseCompany : t.account.noCompany}>
          {me.companies.length ? null : <p className="text-sm text-ink-600">{t.account.noCompanyText}</p>}
        </Card>
      )}
      <ButtonLink href="/account/company/new" variant={me.companies.length ? "secondary" : "primary"}>
        {t.account.createCompany}
      </ButtonLink>
    </div>
  );
}
