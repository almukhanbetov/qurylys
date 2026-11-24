import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getI18n } from "@/lib/i18n/server";
import { getMeOrNull } from "@/lib/session";
import { AuthCard } from "@/features/auth/AuthCard";
import { RegisterForm } from "@/features/auth/RegisterForm";

export async function generateMetadata(): Promise<Metadata> {
  const { t } = await getI18n();
  return { title: t.auth.registerTitle };
}

export default async function Page() {
  const { t } = await getI18n();
  if (await getMeOrNull()) redirect("/account");
  return (
    <AuthCard title={t.auth.registerTitle}>
      <RegisterForm />
    </AuthCard>
  );
}
