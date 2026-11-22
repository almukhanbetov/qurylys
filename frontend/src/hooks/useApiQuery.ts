"use client";

import { useCallback, useEffect, useState } from "react";

type Settled<T> = { requestKey: string; data: T | undefined; error: Error | null };

export type QueryResult<T> =
  | { status: "loading"; data: T | undefined; error: null; retry: () => void }
  | { status: "success"; data: T; error: null; retry: () => void }
  | { status: "error"; data: T | undefined; error: Error; retry: () => void };

/**
 * Runs `fetcher` whenever `key` changes (or `retry()` is called) and cancels
 * stale requests. "loading" is derived: the last settled result belongs to an
 * older request. Previous data is kept while the next request loads, so lists
 * do not flicker between pages.
 */
export function useApiQuery<T>(key: string, fetcher: (signal: AbortSignal) => Promise<T>): QueryResult<T> {
  const [attempt, setAttempt] = useState(0);
  const [settled, setSettled] = useState<Settled<T> | null>(null);
  const requestKey = `${attempt}:${key}`;

  useEffect(() => {
    const controller = new AbortController();
    fetcher(controller.signal)
      .then((data) => setSettled({ requestKey, data, error: null }))
      .catch((err: unknown) => {
        if (controller.signal.aborted) return;
        const error = err instanceof Error ? err : new Error(String(err));
        setSettled((prev) => ({ requestKey, data: prev?.data, error }));
      });
    return () => controller.abort();
    // `requestKey` fully describes the request; `fetcher` is an inline closure.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [requestKey]);

  const retry = useCallback(() => setAttempt((n) => n + 1), []);

  if (!settled || settled.requestKey !== requestKey) {
    return { status: "loading", data: settled?.data, error: null, retry };
  }
  if (settled.error) {
    return { status: "error", data: settled.data, error: settled.error, retry };
  }
  return { status: "success", data: settled.data as T, error: null, retry };
}
