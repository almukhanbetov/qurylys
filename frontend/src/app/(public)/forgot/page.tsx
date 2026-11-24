import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getI18n } from "@/lib/i18n/server";
import { getMeOrNull } from "@/lib/session";
import { AuthCard } from "@/features/auth/AuthCard";
import { ForgotForm } from "@/features/auth/ForgotForm";

export async function generateMetadata(): Promise<Metadata> {
  const { t } = await getI18n();
  return { title: t.auth.forgotTitle };
}

export default async function Page() {
  const { t } = await getI18n();
  if (await getMeOrNull()) redirect("/account");
  return (
    <AuthCard title={t.auth.forgotTitle}>
      <ForgotForm />
    </AuthCard>
  );
}
