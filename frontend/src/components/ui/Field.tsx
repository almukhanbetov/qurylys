import type { InputHTMLAttributes, ReactNode } from "react";

export function Field({ label, hint, ...props }: { label: string; hint?: string } & InputHTMLAttributes<HTMLInputElement>) {
  return (
    <label className="block">
      <span className="text-sm font-medium text-ink-900">{label}</span>
      {/* 16px text: iOS zooms the page into smaller inputs. */}
      <input
        {...props}
        className="mt-1 h-11 w-full rounded-xl border border-ink-300 bg-white px-3 text-base text-ink-900 focus:border-brand-700 focus:outline-none focus:ring-2 focus:ring-brand-200 disabled:bg-ink-50 disabled:text-ink-600"
      />
      {hint ? <span className="mt-1 block text-xs text-ink-500">{hint}</span> : null}
    </label>
  );
}

export function Alert({ tone = "error", children }: { tone?: "error" | "success" | "info"; children: ReactNode }) {
  const tones = {
    error: "bg-red-50 text-red-800 ring-red-200",
    success: "bg-brand-100 text-brand-900 ring-brand-200",
    info: "bg-amber-50 text-amber-900 ring-amber-200",
  };
  return (
    <p role={tone === "error" ? "alert" : "status"} className={`rounded-xl px-4 py-3 text-sm ring-1 ${tones[tone]}`}>
      {children}
    </p>
  );
}

export function Card({ title, children }: { title?: string; children: ReactNode }) {
  return (
    <section className="rounded-2xl border border-ink-200 bg-white p-4 sm:p-6">
      {title ? <h2 className="mb-4 text-lg font-bold">{title}</h2> : null}
      {children}
    </section>
  );
}
