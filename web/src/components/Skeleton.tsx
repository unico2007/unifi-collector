// Lightweight, dependency-free loading placeholders. Used in place of the old
// "Yüklənir..." text so each page shows its shape while data loads.

export function Skeleton({ className = "" }: { className?: string }) {
  return <div className={`animate-pulse rounded bg-slate-200/70 ${className}`} />;
}

function StatCardSkeleton() {
  return (
    <div className="card p-4">
      <Skeleton className="h-3 w-20" />
      <Skeleton className="h-7 w-24 mt-3" />
      <Skeleton className="h-3 w-16 mt-2" />
    </div>
  );
}

function CardSkeleton() {
  return (
    <div className="card p-4">
      <Skeleton className="h-4 w-32 mb-4" />
      <Skeleton className="h-40 w-full" />
    </div>
  );
}

// PageSkeleton approximates a data page: an optional KPI row plus a few content
// cards. stats/cards let each page match its own layout.
export function PageSkeleton({ stats = 4, cards = 2 }: { stats?: number; cards?: number }) {
  return (
    <div className="space-y-4">
      {stats > 0 && (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          {Array.from({ length: stats }).map((_, i) => (
            <StatCardSkeleton key={i} />
          ))}
        </div>
      )}
      {cards > 0 && (
        <div className="grid lg:grid-cols-2 gap-4">
          {Array.from({ length: cards }).map((_, i) => (
            <CardSkeleton key={i} />
          ))}
        </div>
      )}
    </div>
  );
}
