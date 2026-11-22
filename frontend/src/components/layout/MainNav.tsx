"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import { useI18n } from "@/lib/i18n/client";
import { Icon } from "@/components/ui/Icon";

type Item = { href: string | null; label: string };

/** Main navigation. Sections without a page yet are shown disabled with "soon". */
export function MainNav() {
  const { t } = useI18n();
  const pathname = usePathname();
  const [open, setOpen] = useState(false);

  const items: Item[] = [
    { href: "/", label: t.nav.home },
    { href: "/catalog", label: t.nav.catalog },
    { href: "/categories", label: t.nav.categories },
    { href: null, label: t.nav.brands },
    { href: null, label: t.nav.suppliers },
  ];

  const isActive = (href: string) => (href === "/" ? pathname === "/" : pathname.startsWith(href));

  const render = (mobile: boolean) =>
    items.map((it) =>
      it.href ? (
        <Link
          key={it.label}
          href={it.href}
          onClick={() => setOpen(false)}
          aria-current={isActive(it.href) ? "page" : undefined}
          className={`rounded-lg px-3 py-2 text-sm font-medium transition-colors ${
            isActive(it.href) ? "bg-brand-50 text-brand-800" : "text-ink-600 hover:bg-ink-100 hover:text-brand-800"
          } ${mobile ? "block" : ""}`}
        >
          {it.label}
        </Link>
      ) : (
        <span
          key={it.label}
          aria-disabled="true"
          title={t.common.stageNotice}
          className="flex cursor-not-allowed items-center gap-1.5 rounded-lg px-3 py-2 text-sm font-medium text-ink-400"
        >
          {it.label}
          <span className="rounded-full bg-ink-100 px-1.5 py-0.5 text-[10px] font-bold uppercase text-ink-500">
            {t.common.soon}
          </span>
        </span>
      ),
    );

  return (
    <nav aria-label={t.header.menu} className="border-t border-ink-100">
      <div className="mx-auto max-w-7xl px-4 sm:px-6">
        <div className="hidden items-center gap-1 py-2 md:flex">{render(false)}</div>
        <div className="md:hidden">
          <button
            type="button"
            onClick={() => setOpen((v) => !v)}
            aria-expanded={open}
            className="flex w-full items-center gap-2 py-3 text-sm font-semibold text-brand-800"
          >
            <Icon name={open ? "close" : "menu"} className="h-5 w-5" />
            {t.header.menu}
          </button>
          {open ? <div className="flex flex-col gap-1 pb-3">{render(true)}</div> : null}
        </div>
      </div>
    </nav>
  );
}
