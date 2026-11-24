import Link from "next/link";

/** Light logo for dark-green surfaces (header, footer). */
export function Logo({ suffix, href = "/" }: { suffix: string; href?: string }) {
  return (
    <Link
      href={href}
      className="flex shrink-0 items-center gap-2 rounded-lg focus-visible:outline-2 focus-visible:outline-accent-400"
      aria-label={`QURYLYS ${suffix}`}
    >
      <svg viewBox="0 0 24 24" className="h-9 w-9 text-accent-400" fill="none" stroke="currentColor" strokeWidth={1.7} aria-hidden="true">
        <path d="M2 21h20M4 21V11l5-2v12M9 21V5l5-2 2 1v17M16 10l4 2v9M12 8v1.5M12 12v1.5M12 16v1.5" strokeLinejoin="round" strokeLinecap="round" />
      </svg>
      <span className="leading-none">
        <span className="block text-lg font-extrabold leading-none tracking-tight text-white">QURYLYS</span>
        <span className="mt-0.5 block text-sm font-extrabold leading-none text-accent-400">{suffix}</span>
      </span>
    </Link>
  );
}
