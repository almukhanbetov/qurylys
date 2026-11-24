import { getI18n } from "@/lib/i18n/server";
import { Skeleton } from "@/components/ui/Skeleton";

export default async function Loading() {
  const { t } = await getI18n();
  return (
    <div role="status" className="space-y-4">
      <span className="sr-only">{t.common.loading}</span>
      <Skeleton className="h-8 w-1/2" />
      <Skeleton className="h-48 w-full" />
    </div>
  );
}
