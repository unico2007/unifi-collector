import { useEffect, useState } from "react";
import { api, Overview, AiInsights, AiInsight } from "../lib/api";
import { AreaLine, Donut, StatCard } from "../components/charts";

function SvgIcon({ d }: { d: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" className="w-4 h-4">
      <path d={d} />
    </svg>
  );
}

const levelPill: Record<string, string> = {
  info: "bg-slate-100 text-slate-600",
  warn: "bg-amber-50 text-amber-700",
  error: "bg-red-50 text-red-700",
};

const insightAccent: Record<string, string> = {
  info: "border-l-slate-300",
  warn: "border-l-amber-400",
  error: "border-l-red-400",
};

function InsightIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" className="w-4 h-4">
      <path d="M12 3l1.9 4.8L18 9l-4.1 1.2L12 15l-1.9-4.8L6 9l4.1-1.2zM19 14l.9 2.1L22 17l-2.1.9L19 20l-.9-2.1L16 17l2.1-.9z" />
    </svg>
  );
}

function AiInsightsPanel() {
  const [d, setD] = useState<AiInsights | null>(null);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    api.aiInsights().then(setD).finally(() => setLoading(false));
  }, []);
  return (
    <div className="card p-4">
      <div className="flex items-center gap-2 mb-3">
        <span className="w-7 h-7 rounded-lg bg-brand-500 text-white grid place-items-center"><InsightIcon /></span>
        <span className="text-sm font-medium">AI Insights</span>
        <span className="text-xs text-muted ml-auto">avtomatik təhlil</span>
      </div>
      {loading && <div className="text-sm text-muted">Təhlil edilir...</div>}
      {d?.summary && <div className="text-sm text-ink/80 mb-3">{d.summary}</div>}
      <div className="space-y-2">
        {d?.insights.map((it: AiInsight, i) => (
          <div key={i} className={`border-l-2 ${insightAccent[it.level]} pl-3 py-0.5`}>
            <div className="flex items-center gap-2">
              <span className={`pill ${levelPill[it.level]}`}>{it.level}</span>
              <span className="text-sm font-medium">{it.title}</span>
            </div>
            <div className="text-xs text-muted mt-0.5">{it.detail}</div>
          </div>
        ))}
      </div>
    </div>
  );
}

export default function OverviewPage() {
  const [d, setD] = useState<Overview | null>(null);
  useEffect(() => {
    api.overview().then(setD);
  }, []);
  if (!d) return <div className="text-muted">Yüklənir...</div>;

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Cihazlar" value={d.devices.total} sub={`${d.devices.online} online · ${d.devices.offline} offline`} tone="brand" icon={<SvgIcon d="M3 4h18v6H3zM3 14h18v6H3zM7 7h.01M7 17h.01" />} />
        <StatCard label="Klientlər" value={d.clients} sub="qoşulu" tone="slate" spark={d.clientSeries} icon={<SvgIcon d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8z" />} />
        <StatCard label="Sağlamlıq" value={`${d.health}%`} sub="sistem" tone="green" icon={<SvgIcon d="M22 12h-4l-3 9L9 3l-3 9H2" />} />
        <StatCard label="Xəbərdarlıqlar" value={d.alerts} sub="aktiv" tone={d.alerts ? "amber" : "green"} icon={<SvgIcon d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9M13.7 21a2 2 0 0 1-3.4 0" />} />
      </div>

      <AiInsightsPanel />

      <div className="grid lg:grid-cols-3 gap-4">
        <div className="card p-4 lg:col-span-2">
          <div className="text-sm font-semibold mb-3">Klient sayı (24 saat)</div>
          <AreaLine data={d.clientSeries} height={160} />
        </div>
        <div className="card p-4 flex flex-col">
          <div className="text-sm font-semibold mb-2">Cihaz statusu</div>
          <div className="flex-1 grid place-content-center">
            <Donut value={d.devices.online} total={d.devices.total} label="online" sublabel={`${d.devices.online}/${d.devices.total} cihaz`} />
          </div>
        </div>
      </div>

      <div className="grid lg:grid-cols-3 gap-4">
        <div className="space-y-3">
          {d.vendorSplit.map((v) => (
            <div key={v.vendor} className="card p-4 flex items-center justify-between">
              <div>
                <span className={`pill ${v.vendor === "unifi" ? "bg-brand-50 text-brand-700" : "bg-orange-50 text-orange-700"}`}>
                  {v.vendor === "unifi" ? "UniFi" : "Kerio"}
                </span>
                <div className="text-sm text-muted mt-2">{v.devices} cihaz{v.clients ? ` · ${v.clients} klient` : ""}</div>
              </div>
            </div>
          ))}
        </div>

        <div className="card p-4 lg:col-span-2">
          <div className="text-sm font-medium mb-3">Son loglar</div>
          <div className="space-y-1">
            {d.recentLogs.map((l, i) => (
              <div key={i} className="flex items-center gap-3 text-sm py-1 border-b border-line last:border-0">
                <span className="font-mono text-xs text-muted w-16 shrink-0">{l.time}</span>
                <span className={`pill ${levelPill[l.level]}`}>{l.level}</span>
                <span className="truncate">{l.msg}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
