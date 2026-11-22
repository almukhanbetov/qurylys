"use client";

import { useRouter } from "next/navigation";
import { useTransition } from "react";
import { useI18n } from "@/lib/i18n/client";
import { Icon } from "./Icon";

/** Re-runs the server components of the current route (used by server-rendered sections). */
export function RetryButton() {
  const router = useRouter();
  const { t } = useI18n();
  const [pending, startTransition] = useTransition();
  return (
    <button
      type="button"
      disabled={pending}
      onClick={() => startTransition(() => router.refresh())}
      className="inline-flex items-center gap-2 rounded-xl bg-brand-50 px-4 py-2.5 text-sm font-semibold text-brand-800 ring-1 ring-brand-200 hover:bg-brand-100 disabled:opacity-60"
    >
      <Icon name="refresh" className={`h-4 w-4 ${pending ? "animate-spin" : ""}`} />
      {pending ? t.common.loading : t.common.retry}
    </button>
  );
}

/** Inline error block for a server-rendered section that failed to load. */
export function SectionError({ title, text }: { title: string; text: string }) {
  return (
    <div role="alert" className="flex flex-col items-center gap-3 rounded-2xl bg-red-50/60 px-6 py-10 text-center ring-1 ring-red-100">
      <Icon name="alert" className="h-7 w-7 text-red-600" />
      <p className="font-semibold text-ink-900">{title}</p>
      <p className="max-w-md text-sm text-ink-600">{text}</p>
      <RetryButton />
    </div>
  );
}
