"use client";

import type { ReactNode } from "react";
import { useI18n } from "@/lib/i18n/client";
import { Icon } from "./Icon";
import { Button } from "./Button";

/** Error block with a retry button. Used by client lists and route error boundaries. */
export function ErrorState({ onRetry, detail }: { onRetry: () => void; detail?: string }) {
  const { t } = useI18n();
  return (
    <div role="alert" className="flex flex-col items-center gap-3 rounded-2xl bg-red-50/60 px-6 py-10 text-center ring-1 ring-red-100">
      <Icon name="alert" className="h-8 w-8 text-red-600" />
      <p className="text-base font-semibold text-ink-900">{t.common.errorTitle}</p>
      <p className="max-w-md text-sm text-ink-600">{t.common.errorText}</p>
      {detail ? <p className="font-mono text-xs text-ink-400">{detail}</p> : null}
      <Button onClick={onRetry} variant="secondary">
        <Icon name="refresh" className="h-4 w-4" />
        {t.common.retry}
      </Button>
    </div>
  );
}

export function EmptyState({ title, text, action }: { title: string; text?: string; action?: ReactNode }) {
  return (
    <div className="flex flex-col items-center gap-3 rounded-2xl bg-brand-50/60 px-6 py-12 text-center ring-1 ring-brand-100">
      <Icon name="search" className="h-8 w-8 text-brand-600" />
      <p className="text-base font-semibold text-ink-900">{title}</p>
      {text ? <p className="max-w-md text-sm text-ink-600">{text}</p> : null}
      {action}
    </div>
  );
}
