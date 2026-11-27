import Link from "next/link";
import { Icon } from "@/components/ui/Icon";

/** Breadcrumbs for public pages; the last item is the current page. Server and client safe. */
export function Breadcrumbs({ items }: { items: { href?: string; label: string }[] }) {
  return (
    <nav aria-label="breadcrumb" className="flex flex-wrap items-center gap-1 text-sm text-ink-500">
      {items.map((it, i) => (
        <span key={i} className="flex min-w-0 items-center gap-1">
          {i > 0 ? <Icon name="chevronRight" className="h-3.5 w-3.5 shrink-0" /> : null}
          {it.href ? (
            <Link href={it.href} className="hover:text-brand-800">{it.label}</Link>
          ) : (
            <span aria-current="page" className="wrap-anywhere text-ink-700">{it.label}</span>
          )}
        </span>
      ))}
    </nav>
  );
}
