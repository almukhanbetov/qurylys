"use client";

import type { ReactNode } from "react";
import { ApiError } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import type { QueryResult } from "@/hooks/useApiQuery";
import { Skeleton } from "@/components/ui/Skeleton";
import { ErrorState } from "@/components/ui/StateMessage";
import { Alert } from "@/components/ui/Field";
import { ButtonLink } from "@/components/ui/Button";

/** Loading and error states shared by cabinet screens. */
export function QueryState<T>({ result, children }: { result: QueryResult<T>; children: (data: T) => ReactNode }) {
  const { t } = useI18n();
  if (result.status === "error") {
    const code = result.error instanceof ApiError ? result.error.code : "";
    const messages: Record<string, string> = t.errors;
    const known = messages[code];
    if (known) {
      return (
        <div className="space-y-3">
          <Alert>{known}</Alert>
          {code === "unauthorized" ? <ButtonLink href="/login">{t.auth.login}</ButtonLink> : null}
        </div>
      );
    }
    return <ErrorState onRetry={result.retry} />;
  }
  if (result.data === undefined) {
    return (
      <div role="status" className="space-y-3">
        <span className="sr-only">{t.common.loading}</span>
        <Skeleton className="h-12 w-full" />
        <Skeleton className="h-12 w-full" />
      </div>
    );
  }
  return children(result.data);
}
