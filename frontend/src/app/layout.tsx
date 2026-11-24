import type { Metadata, Viewport } from "next";
import type { ReactNode } from "react";
import { getI18n } from "@/lib/i18n/server";
import { I18nProvider } from "@/lib/i18n/client";
import { htmlLang } from "@/lib/format";
import "./globals.css";

export async function generateMetadata(): Promise<Metadata> {
  const { t } = await getI18n();
  return {
    title: { default: t.meta.title, template: "%s · QURYLYS" },
    description: t.meta.description,
  };
}

export const viewport: Viewport = { themeColor: "#123d32" };

export default async function RootLayout({ children }: { children: ReactNode }) {
  const { locale } = await getI18n();
  return (
    <html lang={htmlLang(locale)}>
      <body className="min-h-screen">
        <I18nProvider locale={locale}>{children}</I18nProvider>
      </body>
    </html>
  );
}
