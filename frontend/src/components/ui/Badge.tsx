import type { ReactNode } from "react";

const tones = {
  brand: "bg-brand-100 text-brand-800",
  demo: "bg-amber-100 text-amber-900 ring-1 ring-amber-200",
  muted: "bg-ink-100 text-ink-600",
  danger: "bg-red-50 text-red-700",
} as const;

export function Badge({ tone = "brand", children }: { tone?: keyof typeof tones; children: ReactNode }) {
  return (
    <span className={`inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-semibold ${tones[tone]}`}>
      {children}
    </span>
  );
}
