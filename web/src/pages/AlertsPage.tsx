import { useEffect, useState } from "react";
import { api, AlertsData } from "../lib/api";

const levelStyle: Record<string, { stripe: string; pill: string; label: string }> = {
  critical: { stripe: "bg-red-500", pill: "bg-red-50 text-red-700", label: "Kritik" },
  warning: { stripe: "bg-amber-500", pill: "bg-amber-50 text-amber-700", label: "Xəbərdarlıq" },
};

function Kpi({ label, value, tone }: { label: string; value: number; tone: string }) {
  return (
    <div className="card p-4">
      <div className="text-sm text-muted">{label}</div>
      <div className={`text-3xl font-semibold mt-1 ${tone}`}>{value}</div>
    </div>
  );
}

export default function AlertsPage() {
  const [d, setD] = useState<AlertsData | null>(null);
  useEffect(() => {
    api.alerts().then(setD);
  }, []);
  if (!d) return <div className="text-muted">Yüklənir...</div>;

  const healthy = d.active.length === 0;

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <Kpi label="Kritik" value={d.counts.critical} tone={d.counts.critical ? "text-red-600" : "text-green-600"} />
        <Kpi label="Xəbərdarlıq" value={d.counts.warning} tone={d.counts.warning ? "text-amber-600" : "text-green-600"} />
        <Kpi label="Aktiv qaydalar" value={d.rules.length} tone="" />
        <div className="card p-4">
          <div className="text-sm text-muted">Vəziyyət</div>
          <div className={`text-lg font-semibold mt-2 ${healthy ? "text-green-600" : "text-amber-600"}`}>
            {healthy ? "✓ Hər şey qaydasında" : "Diqqət tələb olunur"}
          </div>
        </div>
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
