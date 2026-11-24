"use client";

import { useState } from "react";
import { ApiError } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";

/** Runs a mutation with pending state and a translated error. */
export function useSubmit() {
  const { t } = useI18n();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function run(action: () => Promise<unknown>): Promise<boolean> {
    setPending(true);
    setError(null);
    try {
      await action();
      return true;
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "";
      const messages: Record<string, string> = t.errors;
      setError(messages[code] ?? t.common.errorText);
      return false;
    } finally {
      setPending(false);
    }
  }

  return { pending, error, run };
}
