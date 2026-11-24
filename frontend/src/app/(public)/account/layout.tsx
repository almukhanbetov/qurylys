import type { Metadata } from "next";
import type { ReactNode } from "react";
import { redirect } from "next/navigation";
import { getI18n } from "@/lib/i18n/server";
import { getMe } from "@/lib/session";
import { AccountShell } from "@/features/account/AccountShell";

export const dynamic = "force-dynamic";

export async function generateMetadata(): Promise<Metadata> {
  const { t } = await getI18n();
  return { title: t.account.title, robots: { index: false } };
}

/** Hides the cabinet from anonymous visitors; every permission is enforced by the API. */
export default async function AccountLayout({ children }: { children: ReactNode }) {
  const me = await getMe();
  if (!me) redirect("/login?next=/account");
  return <AccountShell me={me}>{children}</AccountShell>;
}
