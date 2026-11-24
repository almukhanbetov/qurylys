import Link from "next/link";
import { getI18n } from "@/lib/i18n/server";
import { format } from "@/lib/i18n";
import { Logo } from "./Logo";

export async function Footer() {
  const { t } = await getI18n();
  const year = new Date().getFullYear();

  return (
    <footer className="mt-16 bg-brand-950 text-brand-100">
      <div className="mx-auto grid max-w-7xl gap-10 px-4 py-12 sm:px-6 md:grid-cols-3">
        <div className="space-y-4">
          <div className="inline-flex">
            <Logo suffix={t.header.logoSuffix} />
          </div>
          <p className="max-w-sm text-sm text-brand-200">{t.footer.about}</p>
        </div>

        <div>
          <p className="text-sm font-semibold text-white">{t.footer.sections}</p>
          <ul className="mt-3 space-y-2 text-sm">
            <li><Link className="hover:text-white" href="/">{t.nav.home}</Link></li>
            <li><Link className="hover:text-white" href="/catalog">{t.nav.catalog}</Link></li>
            <li><Link className="hover:text-white" href="/categories">{t.nav.categories}</Link></li>
          </ul>
        </div>

        <div>
          <p className="text-sm font-semibold text-white">{t.footer.forBusiness}</p>
          <ul className="mt-3 space-y-2 text-sm text-brand-300">
            <li>{t.footer.buyers} · {t.common.soon}</li>
            <li>{t.footer.suppliers} · {t.common.soon}</li>
          </ul>
        </div>
      </div>
      <div className="border-t border-brand-900">
        <div className="mx-auto flex max-w-7xl flex-col gap-2 px-4 py-5 text-xs text-brand-300 sm:px-6 md:flex-row md:justify-between">
          <span>{format(t.footer.rights, { year })}</span>
          <span>{t.common.demoNotice}</span>
        </div>
      </div>
    </footer>
  );
}
