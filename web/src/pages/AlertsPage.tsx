import { api, AlertsData } from "../lib/api";
import { usePolling } from "../lib/refresh";
import { StatCard } from "../components/charts";
import { PageSkeleton } from "../components/Skeleton";

const levelStyle: Record<string, { stripe: string; pill: string; label: string }> = {
  critical: { stripe: "bg-red-500", pill: "bg-red-50 text-red-700", label: "Kritik" },
  warning: { stripe: "bg-amber-500", pill: "bg-amber-50 text-amber-700", label: "Xəbərdarlıq" },
};

export default function AlertsPage() {
  const { data: d } = usePolling<AlertsData>(() => api.alerts());
  if (!d) return <PageSkeleton stats={2} cards={2} />;

  const healthy = d.active.length === 0;

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Kritik" value={d.counts.critical} tone={d.counts.critical ? "red" : "green"} icon={<Ico d="M12 9v4M12 17h.01M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />} />
        <StatCard label="Xəbərdarlıq" value={d.counts.warning} tone={d.counts.warning ? "amber" : "green"} icon={<Ico d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9M13.7 21a2 2 0 0 1-3.4 0" />} />
        <StatCard label="Aktiv qaydalar" value={d.rules.length} tone="slate" icon={<Ico d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01" />} />
        <StatCard label="Vəziyyət" value={healthy ? "Qaydasında" : "Diqqət"} tone={healthy ? "green" : "amber"} sub={healthy ? "bütün cihazlar həddlərdə" : "yoxlama tələb olunur"} icon={<Ico d={healthy ? "M20 6 9 17l-5-5" : "M12 9v4M12 17h.01"} />} />
      </div>

      {/* Active alerts */}
      <div className="card">
        <div className="px-4 py-3 border-b border-line text-sm font-medium">Aktiv alertlər</div>
        {healthy ? (
          <div className="p-8 text-center text-muted">
            <div className="text-2xl mb-1">✓</div>
            Heç bir aktiv alert yoxdur. Bütün cihazlar həddlər daxilindədir.
          </div>
        ) : (
          <div>
            {d.active.map((a, i) => {
              const s = levelStyle[a.level] ?? levelStyle.warning;
              return (
                <div key={i} className="flex items-stretch gap-3 px-4 py-3 border-b border-line last:border-0">
                  <div className={`w-1 rounded-full ${s.stripe}`} />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className={`pill ${s.pill}`}>{s.label}</span>
                      <span className="text-xs text-muted">{a.rule}</span>
                    </div>
                    <div className="text-sm mt-1 truncate">{a.message}</div>
                  </div>
                  <div className="flex items-center">
                    <span className="font-mono text-sm text-muted">{a.value}</span>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Rules */}
      <div className="card">
        <div className="px-4 py-3 border-b border-line text-sm font-medium">Qaydalar (həddlər)</div>
        <div className="divide-y divide-line">
          {d.rules.map((r, i) => (
            <div key={i} className="flex items-center gap-3 px-4 py-2.5">
              <span className={`pill ${(levelStyle[r.level] ?? levelStyle.warning).pill}`}>
                {(levelStyle[r.level] ?? levelStyle.warning).label}
              </span>
              <span className="text-sm">{r.name}</span>
              <span className="ml-auto font-mono text-xs text-muted">{r.condition}</span>
            </div>
          ))}
        </div>
      </div>
      <p className="text-xs text-muted">
        Qaydalar hər açılışda Prometheus-dan canlı qiymətləndirilir. Konfiqurasiya olunan həddlər və tarixçə növbəti mərhələdədir.
      </p>
    </div>
  );
}

function Ico({ d }: { d: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" className="w-4 h-4">
      <path d={d} />
    </svg>
  );
}
