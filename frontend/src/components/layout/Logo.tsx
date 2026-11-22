import Link from "next/link";

export function Logo({ suffix, href = "/" }: { suffix: string; href?: string }) {
  return (
    <Link href={href} className="flex shrink-0 items-center gap-2" aria-label="QURYLYS B2B">
      <span className="grid h-9 w-9 place-items-center rounded-xl bg-brand-700 text-white">
        <svg viewBox="0 0 24 24" className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={2} aria-hidden="true">
          <path d="M3 21h18M5 21V10l7-5 7 5v11M9 21v-6h6v6" strokeLinejoin="round" />
        </svg>
      </span>
      <span className="flex items-baseline gap-1.5">
        <span className="text-lg font-extrabold tracking-tight text-brand-900">QURYLYS</span>
        <span className="rounded-md bg-brand-100 px-1.5 py-0.5 text-[11px] font-bold text-brand-800">{suffix}</span>
      </span>
    </Link>
  );
}
