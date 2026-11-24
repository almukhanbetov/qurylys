import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getI18n } from "@/lib/i18n/server";
import { getMeOrNull } from "@/lib/session";
import { safeNext } from "@/lib/redirect";
import { AuthCard } from "@/features/auth/AuthCard";
import { LoginForm } from "@/features/auth/LoginForm";

export async function generateMetadata(): Promise<Metadata> {
  const { t } = await getI18n();
  return { title: t.auth.loginTitle };
}

export default async function Page({ searchParams }: { searchParams: Promise<{ next?: string | string[] }> }) {
  const { t } = await getI18n();
  const next = safeNext((await searchParams).next);
  if (await getMeOrNull()) redirect(next);
  return (
    <AuthCard title={t.auth.loginTitle}>
      <LoginForm next={next} />
    </AuthCard>
  );
}
