"use client";

import type { ReactNode } from "react";
import { useI18n } from "@/lib/i18n/client";
import { format } from "@/lib/i18n";
import { ButtonLink } from "@/components/ui/Button";
import { Card } from "@/components/ui/Field";
import { useMe } from "@/features/account/MeContext";
import { SECTION_PERMISSION, type AdminSection } from "./access";

/**
 * A catalogue section without its permission explains why and where rights are
 * managed. Nothing is granted here; the API refuses such requests anyway.
 */
export function SectionGate({ section, children }: { section: AdminSection; children: ReactNode }) {
  const { t } = useI18n();
  const g = t.admin.gate;
  const me = useMe();
  const perm = SECTION_PERMISSION[section];
  const admin = me.platform_roles.includes("admin");
  if (!perm) {
    // Users and staff: the administrator role only, granted by the server command.
    if (admin) return children;
    return (
      <div className="space-y-4">
        <h1 className="text-2xl font-bold">{t.admin.sections[section]}</h1>
        <Card title={g.title}>
          <p className="text-sm text-ink-700">{g.adminOnly}</p>
        </Card>
      </div>
    );
  }
  if (me.platform_permissions.includes(perm)) return children;
  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">{t.admin.sections[section]}</h1>
      <Card title={g.title}>
        <p className="text-sm text-ink-700">{format(g.text, { permission: t.admin.permissions[perm] })}</p>
        <p className="mt-2 text-sm text-ink-600">{admin ? g.adminHint : g.otherHint}</p>
        {admin ? (
          <div className="mt-4">
            <ButtonLink href={`/account/admin/users/${me.id}`}>{g.manage}</ButtonLink>
          </div>
        ) : null}
      </Card>
    </div>
  );
}
