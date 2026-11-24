"use client";

import { useRouter } from "next/navigation";
import { companyApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { useApiQuery } from "@/hooks/useApiQuery";
import type { Company } from "@/types/api";
import { Alert, Card } from "@/components/ui/Field";
import { EmptyState } from "@/components/ui/StateMessage";
import { ButtonLink } from "@/components/ui/Button";
import { CompanyForm } from "./CompanyForm";
import { useActiveCompany } from "./MeContext";
import { QueryState } from "./QueryState";

export function CompanyView() {
  const { t } = useI18n();
  const active = useActiveCompany();
  if (!active) return <NoCompany />;
  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">{t.account.editCompany}</h1>
      <Loaded key={active.id} id={active.id} />
    </div>
  );
}

export function NoCompany() {
  const { t } = useI18n();
  return (
    <EmptyState
      title={t.account.noCompany}
      text={t.account.noCompanyText}
      action={<ButtonLink href="/account/company/new">{t.account.createCompany}</ButtonLink>}
    />
  );
}

function Loaded({ id }: { id: number }) {
  const { t } = useI18n();
  const router = useRouter();
  const result = useApiQuery(`company:${id}`, (signal) => companyApi.get(id, signal));
  return (
    <QueryState result={result}>
      {(c: Company) => (
        <Card>
          {c.role === "owner" ? null : (
            <div className="mb-4">
              <Alert tone="info">{t.account.readOnly}</Alert>
            </div>
          )}
          <CompanyForm
            initial={{
              name: c.name, bin: c.bin, city: c.city, legal_address: c.legal_address, actual_address: c.actual_address,
              work_phone: c.work_phone, representative_name: c.representative_name, is_buyer: c.is_buyer, is_supplier: c.is_supplier,
            }}
            disabled={c.role !== "owner" || c.status !== "active"}
            submitLabel={t.account.save}
            onSubmit={async (d) => {
              await companyApi.update(id, d);
              router.refresh(); // the shell shows the company name
            }}
          />
        </Card>
      )}
    </QueryState>
  );
}
