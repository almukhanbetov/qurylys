import { getI18n } from "@/lib/i18n/server";
import { Icon } from "@/components/ui/Icon";
import { Logo } from "./Logo";
import { SearchForm } from "./SearchForm";
import { LanguageSwitcher } from "./LanguageSwitcher";
import { MainNav } from "./MainNav";

export async function Header() {
  const { t } = await getI18n();

  return (
    <header className="sticky top-0 z-30 border-b border-ink-100 bg-white/95 backdrop-blur">
      <div className="mx-auto flex max-w-7xl flex-wrap items-center gap-3 px-4 py-3 sm:px-6 md:flex-nowrap md:gap-6">
        <Logo suffix={t.header.logoSuffix} />

        <div className="order-last w-full md:order-none md:flex-1">
          <SearchForm placeholder={t.header.searchPlaceholder} buttonLabel={t.header.searchButton} />
        </div>

        <div className="ml-auto flex items-center gap-2">
          {/* Cart and account arrive in stage 2: visibly disabled, not fake. */}
          <span
            aria-disabled="true"
            title={`${t.header.cart} — ${t.common.stageNotice}`}
            className="relative grid h-11 w-11 cursor-not-allowed place-items-center rounded-xl text-ink-400"
          >
            <Icon name="cart" className="h-6 w-6" />
            <span className="sr-only">
              {t.header.cart} — {t.common.stageNotice}
            </span>
          </span>
          <span
            aria-disabled="true"
            title={t.common.stageNotice}
            className="hidden cursor-not-allowed items-center gap-2 rounded-xl bg-ink-100 px-3 py-2.5 text-sm font-semibold text-ink-500 lg:flex"
          >
            <Icon name="user" className="h-4 w-4" />
            {t.header.login}
            <span className="rounded-full bg-white px-1.5 py-0.5 text-[10px] font-bold uppercase">{t.common.soon}</span>
          </span>
          <LanguageSwitcher />
        </div>
      </div>
      <MainNav />
    </header>
  );
}
