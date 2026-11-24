import { getI18n } from "@/lib/i18n/server";
import { ButtonLink } from "@/components/ui/Button";
import { Logo } from "@/components/layout/Logo";

export default async function NotFound() {
  const { t } = await getI18n();
  return (
    <div className="mx-auto flex min-h-[60vh] max-w-xl flex-col items-center justify-center gap-4 px-4 py-16 text-center">
      <div className="rounded-xl bg-brand-900 px-4 py-3">
        <Logo suffix={t.header.logoSuffix} />
      </div>
      <p className="text-6xl font-extrabold text-brand-200">404</p>
      <h1 className="text-2xl font-bold">{t.common.notFoundTitle}</h1>
      <p className="text-ink-600">{t.common.notFoundText}</p>
      <div className="flex gap-3">
        <ButtonLink href="/">{t.common.toHome}</ButtonLink>
        <ButtonLink href="/catalog" variant="secondary">{t.common.toCatalog}</ButtonLink>
      </div>
    </div>
  );
}
