import Link from "next/link";
import type { ButtonHTMLAttributes, ReactNode } from "react";

const variants = {
  primary: "bg-brand-700 text-white hover:bg-brand-800 disabled:bg-ink-300",
  secondary: "bg-brand-50 text-brand-800 hover:bg-brand-100 ring-1 ring-brand-200",
  ghost: "text-brand-800 hover:bg-brand-50",
} as const;

const base =
  "inline-flex items-center justify-center gap-2 rounded-xl px-4 py-2.5 text-sm font-semibold transition-colors " +
  "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-600 disabled:cursor-not-allowed";

type Variant = keyof typeof variants;

export function Button({
  variant = "primary",
  className = "",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant }) {
  return <button className={`${base} ${variants[variant]} ${className}`} {...props} />;
}

export function ButtonLink({
  href,
  variant = "primary",
  className = "",
  children,
}: {
  href: string;
  variant?: Variant;
  className?: string;
  children: ReactNode;
}) {
  return (
    <Link href={href} className={`${base} ${variants[variant]} ${className}`}>
      {children}
    </Link>
  );
}

/**
 * A control for a feature that does not exist yet. It is visibly disabled and
 * labelled "soon", so it never pretends to work.
 */
export function UnavailableButton({
  children,
  soonLabel,
  title,
  className = "",
}: {
  children: ReactNode;
  soonLabel: string;
  title: string;
  className?: string;
}) {
  return (
    <button
      type="button"
      disabled
      aria-disabled="true"
      title={title}
      className={`${base} cursor-not-allowed bg-ink-100 text-ink-500 ${className}`}
    >
      {children}
      <span className="rounded-full bg-white px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide text-ink-500">
        {soonLabel}
      </span>
    </button>
  );
}
