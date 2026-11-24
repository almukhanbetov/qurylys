"use client";

import { companyApi } from "@/lib/api";
import { hardNavigate } from "@/lib/redirect";
import { useI18n } from "@/lib/i18n/client";
import { Card } from "@/components/ui/Field";
import { CompanyForm } from "./CompanyForm";
import { useMe } from "./MeContext";

export function NewCompanyView() {
  const { t } = useI18n();
  const me = useMe();
  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">{t.account.createCompany}</h1>
      <Card>
        <CompanyForm
          initial={{
            name: "", bin: "", city: "", legal_address: "", actual_address: "", work_phone: "",
            representative_name: me.full_name, is_buyer: true, is_supplier: false,
          }}
          submitLabel={t.account.createCompany}
          onSubmit={async (d) => {
            await companyApi.create(d);
            // The new company becomes active: reload with a clean cache.
            hardNavigate("/account");
          }}
        />
      </Card>
    </div>
  );
}
