import Link from "next/link";
import { getMeOrNull } from "@/lib/session";
import { getI18n } from "@/lib/i18n/server";
import { Icon } from "@/components/ui/Icon";
import { Logo } from "./Logo";
import { SearchForm } from "./SearchForm";
import { LanguageSwitcher } from "./LanguageSwitcher";
import { MainNav, MobileMenu } from "./MainNav";

export async function Header() {
  const { t } = await getI18n();
  const signedIn = (await getMeOrNull()) !== null;

  return (
    <header className="sticky top-0 z-30 bg-brand-900 text-white">
      <div className="mx-auto flex max-w-7xl flex-wrap items-center gap-x-3 gap-y-2 px-4 py-2.5 sm:px-6 md:flex-nowrap md:gap-6">
        <Logo suffix={t.header.logoSuffix} />

        <div className="order-last w-full md:order-none md:flex-1">
          <SearchForm placeholder={t.header.searchPlaceholder} buttonLabel={t.header.searchButton} />
        </div>

        <div className="ml-auto flex items-center gap-2">
          {/* The cart arrives in a later stage: visibly disabled, not fake. */}
          <span
            aria-disabled="true"
            title={`${t.header.cart} — ${t.common.stageNotice}`}
            className="hidden h-11 w-11 cursor-not-allowed place-items-center rounded-xl text-brand-300 sm:grid"
          >
            <Icon name="cart" className="h-6 w-6" />
            <span className="sr-only">
              {t.header.cart} — {t.common.stageNotice}
            </span>
          </span>
          <Link
            href={signedIn ? "/account" : "/login"}
            className="hidden min-h-11 items-center gap-2 rounded-xl bg-white/10 px-3 text-sm font-semibold hover:bg-white/20 focus-visible:outline-2 focus-visible:outline-accent-400 md:flex"
          >
            <Icon name="user" className="h-4 w-4" />
            {signedIn ? t.account.title : t.header.login}
          </Link>
          <LanguageSwitcher />
          <MobileMenu signedIn={signedIn} />
        </div>
      </div>
      <MainNav />
    </header>
  );
}
