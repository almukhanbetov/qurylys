"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { cardApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { useSubmit } from "@/hooks/useSubmit";
import type { CardFields, GtinConfirmation } from "@/types/api";
import { Button } from "@/components/ui/Button";
import { Alert, Card } from "@/components/ui/Field";
import { CardFieldsForm, GtinFields } from "./CardForm";

export function NewCardView() {
  const { t } = useI18n();
  const a = t.admin;
  const router = useRouter();
  const [f, setF] = useState<CardFields>({ category_id: 0, name_kk: "", name_ru: "", description_kk: "", description_ru: "", brand_id: null, unit: "piece" });
  const [brandName, setBrandName] = useState<string | null>(null);
  const [g, setG] = useState<GtinConfirmation>({ gtin: "", source_kind: "manufacturer", source: "" });
  const { pending, error, run } = useSubmit();

  return (
    <div className="space-y-4">
      <Link href="/account/admin/products" className="text-sm font-semibold text-brand-700 hover:underline">
        {a.back}
      </Link>
      <h1 className="text-2xl font-bold">{a.card.newTitle}</h1>
      <Alert tone="info">{a.card.newText}</Alert>
      <form
        className="space-y-4"
        onSubmit={(e) => {
          e.preventDefault();
          void run(async () => {
            const { id } = await cardApi.create(f, g);
            router.push(`/account/admin/products/${id}`);
          });
        }}
      >
        <Card title={a.card.fields}>
          <CardFieldsForm
            value={f}
            brandName={brandName}
            disabled={pending}
            onChange={(next, name) => {
              setF(next);
              if (name !== undefined) setBrandName(name);
            }}
          />
        </Card>
        <Card title={a.card.gtinTitle}>
          <GtinFields value={g} onChange={setG} />
        </Card>
        {error ? <Alert>{error}</Alert> : null}
        <Button type="submit" disabled={pending || !f.category_id}>{pending ? t.common.loading : a.products.create}</Button>
      </form>
    </div>
  );
}
