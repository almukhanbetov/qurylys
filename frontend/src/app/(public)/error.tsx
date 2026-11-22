"use client";

import { useRouter } from "next/navigation";
import { startTransition, useEffect } from "react";
import { ErrorState } from "@/components/ui/StateMessage";

/** Route-level error boundary: retry re-fetches the server data of the segment. */
export default function PublicError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  const router = useRouter();

  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <div className="mx-auto max-w-3xl px-4 py-16 sm:px-6">
      <ErrorState
        detail={error.digest ? `ref: ${error.digest}` : undefined}
        onRetry={() =>
          startTransition(() => {
            router.refresh();
            reset();
          })
        }
      />
    </div>
  );
}
