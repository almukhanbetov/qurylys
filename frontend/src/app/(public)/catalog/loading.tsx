import { getI18n } from "@/lib/i18n/server";
import { ProductGridSkeleton, Skeleton } from "@/components/ui/Skeleton";

export default async function Loading() {
  const { t } = await getI18n();
  return (
    <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6">
      <Skeleton className="h-8 w-64" />
      <Skeleton className="mt-3 h-4 w-40" />
      <div className="mt-8">
        <ProductGridSkeleton count={8} label={t.common.loading} />
      </div>
    </div>
  );
}
