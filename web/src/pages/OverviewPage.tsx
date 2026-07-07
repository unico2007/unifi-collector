import { useEffect, useState } from "react";
import { api, Overview, AiInsights, AiInsight } from "../lib/api";

function Kpi({ label, value, sub, tone }: { label: string; value: string; sub: string; tone?: string }) {
  return (
    <div className="card p-4">
      <div className="text-sm text-muted">{label}</div>
      <div className={`text-2xl font-semibold mt-1 ${tone ?? ""}`}>{value}</div>
      <div className="text-xs text-muted mt-1">{sub}</div>
    </div>
  );
}

function LineChart({ data }: { data: number[] }) {
  const w = 520, h = 140, pad = 6;
  const min = Math.min(...data), max = Math.max(...data);
  const nx = (i: number) => pad + (i * (w - pad * 2)) / (data.length - 1);
  const ny = (v: number) => h - pad - ((v - min) / (max - min || 1)) * (h - pad * 2);
  const line = data.map((v, i) => `${i ? "L" : "M"}${nx(i)},${ny(v)}`).join(" ");
  const area = `${line} L${nx(data.length - 1)},${h} L${nx(0)},${h} Z`;
  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="w-full h-36">
      <path d={area} fill="#1466d6" fillOpacity="0.08" />
      <path d={line} fill="none" stroke="#1466d6" strokeWidth="2" strokeLinecap="round" />
    </svg>
  );
}

function Donut({ online, offline }: { online: number; offline: number }) {
  const total = online + offline || 1;
  const frac = online / total;
  const c = 2 * Math.PI * 42;
  return (
    <div className="relative w-40 h-40 mx-auto">
      <svg viewBox="0 0 100 100" className="w-40 h-40 -rotate-90">
        <circle cx="50" cy="50" r="42" fill="none" stroke="#e9edf3" strokeWidth="12" />
        <circle cx="50" cy="50" r="42" fill="none" stroke="#16a34a" strokeWidth="12" strokeDasharray={`${frac * c} ${c}`} strokeLinecap="round" />
      </svg>
      <div className="absolute inset-0 grid place-content-center text-center">
        <div className="text-2xl font-semibold">{Math.round(frac * 100)}%</div>
        <div className="text-xs text-muted">online</div>
      </div>
    </div>
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
        <Kpi label="Cihazlar" value={String(d.devices.total)} sub={`${d.devices.online} online · ${d.devices.offline} offline`} />
        <Kpi label="Klientlər" value={String(d.clients)} sub="qoşulu" />
        <Kpi label="Sağlamlıq" value={`${d.health}%`} sub="sistem" tone="text-green-600" />
        <Kpi label="Xəbərdarlıqlar" value={String(d.alerts)} sub="aktiv" tone="text-amber-600" />
      </div>

      <AiInsightsPanel />

      <div className="grid lg:grid-cols-3 gap-4">
        <div className="card p-4 lg:col-span-2">
          <div className="text-sm font-medium mb-2">Klient sayı (24 saat)</div>
          <LineChart data={d.clientSeries} />
        </div>
        <div className="card p-4">
          <div className="text-sm font-medium mb-2">Cihaz statusu</div>
          <Donut online={d.devices.online} offline={d.devices.offline} />
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
