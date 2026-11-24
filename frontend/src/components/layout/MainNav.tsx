"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import { useI18n } from "@/lib/i18n/client";
import { Icon } from "@/components/ui/Icon";

type Item = { href: string | null; label: string };

/** Sections without a page yet are shown disabled with "soon". */
function NavItems({ onNavigate }: { onNavigate?: () => void }) {
  const { t } = useI18n();
  const pathname = usePathname();

  const items: Item[] = [
    { href: "/", label: t.nav.home },
    { href: "/catalog", label: t.nav.catalog },
    { href: "/categories", label: t.nav.categories },
    { href: null, label: t.nav.brands },
    { href: null, label: t.nav.suppliers },
  ];

  const isActive = (href: string) => (href === "/" ? pathname === "/" : pathname.startsWith(href));
  const base = "flex items-center gap-1.5 rounded-lg px-3 py-3 text-base font-medium md:py-1.5 md:text-sm";

  return items.map((it) =>
    it.href ? (
      <Link
        key={it.label}
        href={it.href}
        onClick={onNavigate}
        aria-current={isActive(it.href) ? "page" : undefined}
        className={`${base} transition-colors focus-visible:outline-2 focus-visible:outline-accent-400 ${
          isActive(it.href) ? "bg-brand-100 text-brand-900" : "text-white hover:bg-white/10"
        }`}
      >
        {it.label}
      </Link>
    ) : (
      <span
        key={it.label}
        aria-disabled="true"
        title={t.common.stageNotice}
        className={`${base} cursor-not-allowed text-brand-300`}
      >
        {it.label}
        <span className="rounded-full bg-white/10 px-1.5 py-0.5 text-[10px] font-bold uppercase">{t.common.soon}</span>
      </span>
    ),
  );
}

/** Desktop navigation row. */
export function MainNav() {
  const { t } = useI18n();
  return (
    <nav aria-label={t.header.menu} className="hidden border-t border-white/10 md:block">
      <div className="mx-auto flex max-w-7xl items-center gap-1 px-4 py-1.5 sm:px-6">
        <NavItems />
      </div>
    </nav>
  );
}

/** Phone menu: a button in the header row; the panel drops below the sticky header. */
export function MobileMenu({ signedIn }: { signedIn: boolean }) {
  const { t } = useI18n();
  const [open, setOpen] = useState(false);
  const close = () => setOpen(false);

  return (
    <div className="md:hidden" onKeyDown={(e) => e.key === "Escape" && close()}>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-controls="mobile-menu"
        aria-label={t.header.menu}
        className="grid h-11 w-11 cursor-pointer place-items-center rounded-xl bg-white/10 focus-visible:outline-2 focus-visible:outline-accent-400"
      >
        <Icon name={open ? "close" : "menu"} className="h-6 w-6" />
      </button>
      <nav
        id="mobile-menu"
        aria-label={t.header.menu}
        hidden={!open}
        className="absolute inset-x-0 top-full max-h-[70vh] overflow-y-auto border-t border-white/10 bg-brand-900 px-4 py-2 shadow-lg"
      >
        <NavItems onNavigate={close} />
        <Link
          href={signedIn ? "/account" : "/login"}
          onClick={close}
          className="mt-1 flex items-center gap-2 border-t border-white/10 px-3 py-3 text-base font-semibold text-accent-400"
        >
          <Icon name="user" className="h-5 w-5" />
          {signedIn ? t.account.title : t.header.login}
        </Link>
      </nav>
    </div>
  );
}
