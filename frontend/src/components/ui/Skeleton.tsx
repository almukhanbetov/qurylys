export function Skeleton({ className = "" }: { className?: string }) {
  return <div className={`animate-pulse rounded-xl bg-ink-100 ${className}`} />;
}

export function ProductGridSkeleton({ count = 8, label }: { count?: number; label: string }) {
  return (
    <div role="status" aria-live="polite" className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      <span className="sr-only">{label}</span>
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className="rounded-2xl bg-white p-4 ring-1 ring-ink-100">
          <Skeleton className="aspect-[4/3] w-full" />
          <Skeleton className="mt-4 h-4 w-3/4" />
          <Skeleton className="mt-2 h-4 w-1/2" />
          <Skeleton className="mt-6 h-6 w-1/3" />
        </div>
      ))}
    </div>
  );
}
