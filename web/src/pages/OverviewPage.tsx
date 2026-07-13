import { useState } from "react";
import { Link } from "react-router-dom";
import { api, Overview, AiInsights, AiInsight, AlertHistoryRow, TimeRange } from "../lib/api";
import { usePolling } from "../lib/refresh";
import { AreaLine, Donut, RangeSelector, rangeLabel, rangeTicks, seriesDelta, StatCard, TopBars } from "../components/charts";
import { PageSkeleton } from "../components/Skeleton";

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
  // AI insights run an LLM synthesis server-side, so poll them slower than data.
  const { data: d, loading } = usePolling<AiInsights>(() => api.aiInsights(), [], 60000);
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

// Compact "systems operational" strip: one glance says whether anything needs
// attention, with direct links to the offending pages.
function StatusStrip({ d }: { d: Overview }) {
  // Monitoring itself is down: don't paint a reassuring green/amber summary from
  // numbers we couldn't actually fetch — say so plainly.
  if (d.degraded) {
    return (
      <div className="card px-4 py-2.5 flex flex-wrap items-center gap-x-3 gap-y-1 border-l-4 border-l-red-500">
        <span className="w-2 h-2 rounded-full bg-red-500" />
        <span className="text-sm font-medium text-red-700">
          Monitorinq əlçatmazdır — Prometheus cavab vermir. Göstərilən rəqəmlər etibarlı deyil.
        </span>
      </div>
    );
  }

  const problems: { text: string; to: string }[] = [];
  if (d.devices.offline > 0) problems.push({ text: `${d.devices.offline} cihaz offline`, to: "/devices" });
  if (d.alerts > 0) problems.push({ text: `${d.alerts} aktiv alert`, to: "/alerts" });
  const ok = problems.length === 0;

  return (
    <div
      className={`card px-4 py-2.5 flex flex-wrap items-center gap-x-3 gap-y-1 border-l-4 ${
        ok ? "border-l-green-500" : "border-l-amber-400"
      }`}
    >
      <span className={`w-2 h-2 rounded-full ${ok ? "bg-green-500" : "bg-amber-400"}`} />
      {ok ? (
        <span className="text-sm font-medium">Bütün sistemlər işləyir</span>
      ) : (
        <span className="text-sm font-medium flex flex-wrap gap-x-2">
          {problems.map((p, i) => (
            <Link key={i} to={p.to} className="hover:underline">
              {p.text}
              {i < problems.length - 1 ? " ·" : ""}
            </Link>
          ))}
        </span>
      )}
      <span className="ml-auto flex items-center gap-2 text-xs text-muted">
        {d.vendorSplit.map((v) => (
          <span key={v.vendor} className="pill bg-page border border-line text-muted">
            {v.vendor === "unifi" ? "UniFi" : "Kerio"} · {v.devices}
          </span>
        ))}
      </span>
    </div>
  );
}

// Status-page style uptime strip: last 24h of fleet health in 30-min buckets.
function HealthStrip({ bars }: { bars: number[] }) {
  if (!bars.length) return null;
  const tone = (v: number) =>
    v >= 99 ? "bg-green-500" : v >= 95 ? "bg-green-300" : v >= 80 ? "bg-amber-400" : "bg-red-500";
  // Bucket i ends (bars.length - 1 - i) * 30min ago.
  const when = (i: number) => {
    const t = new Date(Date.now() - (bars.length - 1 - i) * 30 * 60 * 1000);
    return t.toLocaleTimeString("az", { hour: "2-digit", minute: "2-digit" });
  };
  return (
    <div className="card px-4 py-3">
      <div className="flex items-center mb-2">
        <span className="text-sm font-semibold">Sağlamlıq — son 24 saat</span>
        <span className="ml-auto flex items-center gap-3 text-[11px] text-muted">
          <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-sm bg-green-500" />100%</span>
          <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-sm bg-amber-400" />qismən</span>
          <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-sm bg-red-500" />problem</span>
        </span>
      </div>
      <div className="flex gap-[3px] items-end">
        {bars.map((v, i) => (
          <span
            key={i}
            title={`${when(i)} — ${Math.round(v)}%`}
            className={`flex-1 h-7 rounded-[3px] ${tone(v)} hover:opacity-75 transition-opacity`}
          />
        ))}
      </div>
      <div className="flex justify-between text-[10px] text-muted mt-1.5">
        <span>24 saat əvvəl</span>
        <span>indi</span>
      </div>
    </div>
  );
}

// Compact per-vendor infrastructure summary with an online/total ratio bar.
function InfraCard({ d }: { d: Overview }) {
  return (
    <div className="card p-4">
      <div className="text-sm font-semibold mb-3">İnfrastruktur</div>
      <div className="space-y-4">
        {d.vendorSplit.map((v) => {
          const pct = v.devices ? Math.round((v.online / v.devices) * 100) : 100;
          return (
            <div key={v.vendor}>
              <div className="flex items-center gap-2">
                <span className={`pill ${v.vendor === "unifi" ? "bg-brand-50 text-brand-700" : "bg-orange-50 text-orange-700"}`}>
                  {v.vendor === "unifi" ? "UniFi" : "Kerio"}
                </span>
                <span className="text-sm font-medium tabular-nums">
                  {v.online}/{v.devices} online
                </span>
                <span className="ml-auto text-xs text-muted tabular-nums">
                  {v.clients ? `${v.clients} klient` : "—"}
                </span>
              </div>
              <div className="mt-2 h-1.5 rounded-full bg-line overflow-hidden">
                <div
                  className={`h-full ${pct === 100 ? "bg-green-500" : pct >= 80 ? "bg-amber-400" : "bg-red-500"}`}
                  style={{ width: `${pct}%` }}
                />
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

// Recent alert activity (fired/resolved), from the same history the Alerts
// page shows — so the Overview tells the incident story, not just log noise.
function AlertFeed() {
  const { data } = usePolling<AlertHistoryRow[]>(() => api.alertHistory(), [], 30000);
  const rows = (data ?? []).slice(0, 5);
  const fmt = (ts: number) =>
    new Date(ts * 1000).toLocaleString("az", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });
  return (
    <div className="card p-4 flex flex-col">
      <div className="flex items-center mb-3">
        <span className="text-sm font-semibold">Son alertlər</span>
        <Link to="/alerts" className="ml-auto text-xs text-brand-600 hover:underline">
          hamısı →
        </Link>
      </div>
      {rows.length === 0 && <div className="text-sm text-muted">Alert tarixçəsi boşdur ✓</div>}
      <div className="space-y-2.5">
        {rows.map((a, i) => (
          <div key={i} className="flex gap-2.5">
            <span className={`mt-1.5 w-2 h-2 rounded-full shrink-0 ${a.level === "critical" ? "bg-red-500" : "bg-amber-400"}`} />
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium truncate">{a.rule} — {a.target}</span>
                {a.resolvedAt === 0 ? (
                  <span className="pill bg-red-50 text-red-600 shrink-0">aktiv</span>
                ) : (
                  <span className="pill bg-green-50 text-green-700 shrink-0">həll olunub</span>
                )}
              </div>
              <div className="text-[11px] text-muted tabular-nums">{fmt(a.firedAt)}</div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

export default function OverviewPage() {
  const [range, setRange] = useState<TimeRange>("24h");
  const { data: d } = usePolling<Overview>(() => api.overview(range), [range]);
  if (!d) return <PageSkeleton stats={4} cards={2} />;

  const deltaLabel = `${rangeLabel[range]} ərzində dəyişmə`;

  return (
    <div className="space-y-4">
      <StatusStrip d={d} />

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Cihazlar" value={d.devices.total} sub={`${d.devices.online} online · ${d.devices.offline} offline`} tone="brand" to="/devices" spark={d.deviceSeries} icon={<SvgIcon d="M3 4h18v6H3zM3 14h18v6H3zM7 7h.01M7 17h.01" />} />
        <StatCard label="Klientlər" value={d.clients} sub="qoşulu" tone="slate" to="/clients" spark={d.clientSeries} delta={seriesDelta(d.clientSeries)} deltaLabel={deltaLabel} icon={<SvgIcon d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8z" />} />
        <StatCard label="Sağlamlıq" value={d.health < 0 ? "—" : `${d.health}%`} sub={d.health < 0 ? "məlumat yoxdur" : "cihazların online payı"} tone={d.health < 0 ? "slate" : d.health >= 90 ? "green" : d.health >= 70 ? "amber" : "red"} to="/devices" spark={d.healthSeries} icon={<SvgIcon d="M22 12h-4l-3 9L9 3l-3 9H2" />} />
        <StatCard label="Xəbərdarlıqlar" value={d.alerts} sub="aktiv" tone={d.alerts ? "amber" : "green"} to="/alerts" icon={<SvgIcon d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9M13.7 21a2 2 0 0 1-3.4 0" />} />
      </div>

      <AiInsightsPanel />

      <div className="grid lg:grid-cols-3 gap-4">
        <div className="card p-4 lg:col-span-2">
          <div className="flex items-center mb-3">
            <div className="text-sm font-semibold">Klient sayı ({rangeLabel[range]})</div>
            <div className="ml-auto"><RangeSelector value={range} onChange={setRange} /></div>
          </div>
          <AreaLine data={d.clientSeries} height={160} xLabels={rangeTicks(range)} />
        </div>
        <div className="card p-4 flex flex-col">
          <div className="text-sm font-semibold mb-2">Cihaz statusu</div>
          <div className="flex-1 grid place-content-center">
            <Donut value={d.devices.online} total={d.devices.total} label="online" sublabel={`${d.devices.online}/${d.devices.total} cihaz`} />
          </div>
        </div>
      </div>

      <HealthStrip bars={d.healthBars} />

      <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-4">
        <InfraCard d={d} />
        <div className="card p-4">
          <div className="flex items-center mb-3">
            <span className="text-sm font-semibold">Ən aktiv klientlər</span>
            <Link to="/traffic" className="ml-auto text-xs text-brand-600 hover:underline">trafik →</Link>
          </div>
          {d.topClients.length === 0 ? (
            <div className="text-sm text-muted">Hazırda aktiv trafik yoxdur</div>
          ) : (
            <TopBars rows={d.topClients.map((c) => ({ label: c.label, value: c.value }))} unit=" Mbps" />
          )}
        </div>
        <AlertFeed />
      </div>

      <div className="card p-4">
        <div className="flex items-center mb-3">
          <span className="text-sm font-semibold">Son loglar</span>
          <Link to="/logs" className="ml-auto text-xs text-brand-600 hover:underline">hamısı →</Link>
        </div>
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
  );
}
