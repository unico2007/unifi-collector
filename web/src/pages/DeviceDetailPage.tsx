import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api, AlertHistoryRow, DeviceDetail, TimeRange } from "../lib/api";
import { usePolling } from "../lib/refresh";
import { Card, AreaLine, DualArea, Gauge, RangeSelector, rangeLabel, rangeTicks } from "../components/charts";
import { PageSkeleton } from "../components/Skeleton";

const fmtMbps = (v: number) => (v >= 10 ? Math.round(v) : Math.round(v * 10) / 10);

const levelPill: Record<string, string> = {
  info: "bg-slate-100 text-slate-600",
  warn: "bg-amber-50 text-amber-700",
  error: "bg-red-50 text-red-700",
};

type Tab = "overview" | "metrics" | "logs" | "alerts";
const tabs: { id: Tab; label: string }[] = [
  { id: "overview", label: "Ümumi" },
  { id: "metrics", label: "Metrikalar" },
  { id: "logs", label: "Loglar" },
  { id: "alerts", label: "Alertlər" },
];

export default function DeviceDetailPage() {
  const { name = "" } = useParams();
  const [range, setRange] = useState<TimeRange>("24h");
  const [tab, setTab] = useState<Tab>("overview");
  const { data: d } = usePolling<DeviceDetail>(() => api.device(name, range), [name, range]);
  if (!d) return <PageSkeleton stats={4} cards={2} />;

  const dev = d.device;
  return (
    <div className="space-y-4">
      <Link to="/devices" className="text-sm text-brand-600 hover:underline">← Cihazlar</Link>

      <div className="card p-4 flex flex-wrap items-center gap-x-8 gap-y-2">
        <div>
          <div className="text-xl font-semibold">{dev.name}</div>
          <div className="text-sm text-muted">
            <span className={`pill mr-2 ${dev.vendor === "unifi" ? "bg-brand-50 text-brand-700" : "bg-orange-50 text-orange-700"}`}>
              {dev.vendor === "unifi" ? "UniFi" : "Kerio"}
            </span>
            {dev.model} · {dev.type}
          </div>
        </div>
        <Meta label="Status" value={dev.state === "online" ? "Online" : "Offline"} tone={dev.state === "online" ? "text-green-600" : "text-red-600"} />
        <Meta label="IP" value={dev.ip} mono />
        <Meta label="MAC" value={dev.mac} mono />
        <Meta label="Uptime" value={dev.uptime} />
      </div>

      <div className="flex items-center gap-1 border-b border-line">
        {tabs.map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors ${
              tab === t.id ? "border-brand-500 text-brand-600" : "border-transparent text-muted hover:text-ink"
            }`}
          >
            {t.label}
          </button>
        ))}
        {(tab === "overview" || tab === "metrics") && (
          <div className="ml-auto pb-1.5">
            <RangeSelector value={range} onChange={setRange} />
          </div>
        )}
      </div>

      {tab === "overview" && <OverviewTab d={d} />}
      {tab === "metrics" && <MetricsTab d={d} range={range} />}
      {tab === "logs" && <LogsTab d={d} />}
      {tab === "alerts" && <AlertsTab target={dev.name} />}
    </div>
  );
}

function OverviewTab({ d }: { d: DeviceDetail }) {
  const last = (a: number[]) => (a.length ? a[a.length - 1] : 0);
  const curRx = fmtMbps(last(d.rx));
  const curTx = fmtMbps(last(d.tx));
  const dev = d.device;
  return (
    <div className="space-y-4">
      <div className="grid lg:grid-cols-3 gap-4">
        <Card title="CPU"><div className="grid place-items-center"><Gauge value={dev.cpu} label="indi" /></div></Card>
        <Card title="Yaddaş"><div className="grid place-items-center"><Gauge value={dev.memory} label="indi" tone="#f59e0b" /></div></Card>
        <Card title="Trafik (RX/TX indi)">
          <div className="grid place-items-center h-32 text-center">
            <div>
              <div className="text-2xl font-semibold text-brand-600 tabular-nums">{curRx} <span className="text-sm text-muted">Mbps ↓</span></div>
              <div className="text-2xl font-semibold text-green-600 tabular-nums mt-1">{curTx} <span className="text-sm text-muted">Mbps ↑</span></div>
            </div>
          </div>
        </Card>
      </div>

      <Card title="Qoşulu klientlər" subtitle={d.clients.length ? `${d.clients.length} klient` : undefined}>
        {d.clients.length === 0 ? (
          <div className="text-sm text-muted py-2">Bu cihaza qoşulu klient yoxdur</div>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-muted">
                {["Ad", "MAC", "Siqnal", "Endirmə", "Yükləmə"].map((h) => (
                  <th key={h} className="font-medium px-3 py-2 border-b border-line">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {d.clients.map((c) => (
                <tr key={c.mac} className="odd:bg-page/60">
                  <td className="px-3 py-2 border-b border-line font-medium">{c.name}</td>
                  <td className="px-3 py-2 border-b border-line font-mono text-xs">{c.mac}</td>
                  <td className="px-3 py-2 border-b border-line">{c.rssi} dBm</td>
                  <td className="px-3 py-2 border-b border-line">{c.rx}</td>
                  <td className="px-3 py-2 border-b border-line">{c.tx}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Card>
    </div>
  );
}

function MetricsTab({ d, range }: { d: DeviceDetail; range: TimeRange }) {
  return (
    <div className="space-y-4">
      <div className="grid lg:grid-cols-2 gap-4">
        <Card title={`CPU (${rangeLabel[range]})`}><AreaLine data={d.cpu} unit="%" xLabels={rangeTicks(range)} /></Card>
        <Card title={`Yaddaş (${rangeLabel[range]})`}><AreaLine data={d.memory} color="#f59e0b" unit="%" xLabels={rangeTicks(range)} /></Card>
      </div>
      <Card title={`Trafik (${rangeLabel[range]})`} subtitle="Mbps — endirmə / yükləmə">
        <DualArea a={d.rx} b={d.tx} height={180} unit=" Mbps" xLabels={rangeTicks(range)} />
      </Card>
    </div>
  );
}

function LogsTab({ d }: { d: DeviceDetail }) {
  return (
    <Card title="Cihazın logları" subtitle="son 24 saat — cihazın adı keçən sətirlər">
      {d.logs.length === 0 ? (
        <div className="text-sm text-muted py-2">Bu cihaz üçün log tapılmadı</div>
      ) : (
        <div className="space-y-1">
          {d.logs.map((l, i) => (
            <div key={i} className="flex items-center gap-3 text-sm py-1 border-b border-line last:border-0">
              <span className="font-mono text-xs text-muted w-16 shrink-0">{l.time}</span>
              <span className={`pill ${levelPill[l.level] ?? levelPill.info}`}>{l.level}</span>
              <span className="truncate">{l.msg}</span>
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}

function AlertsTab({ target }: { target: string }) {
  const { data } = usePolling<AlertHistoryRow[]>(() => api.alertHistory(), [], 30000);
  const rows = (data ?? []).filter((a) => a.target === target).slice(0, 20);
  const fmt = (ts: number) =>
    ts ? new Date(ts * 1000).toLocaleString("az", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" }) : "—";
  return (
    <Card title="Alert tarixçəsi" subtitle="yalnız bu cihaza aid">
      {rows.length === 0 ? (
        <div className="text-sm text-muted py-2">Bu cihaz üçün alert qeydə alınmayıb ✓</div>
      ) : (
        <div className="space-y-2.5">
          {rows.map((a, i) => (
            <div key={i} className="flex gap-2.5">
              <span className={`mt-1.5 w-2 h-2 rounded-full shrink-0 ${a.level === "critical" ? "bg-red-500" : "bg-amber-400"}`} />
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium truncate">{a.rule}</span>
                  {a.resolvedAt === 0 ? (
                    <span className="pill bg-red-50 text-red-600 shrink-0">aktiv</span>
                  ) : (
                    <span className="pill bg-green-50 text-green-700 shrink-0">həll olunub</span>
                  )}
                </div>
                <div className="text-xs text-muted truncate">{a.message}</div>
                <div className="text-[11px] text-muted tabular-nums">
                  {fmt(a.firedAt)}{a.resolvedAt ? ` → ${fmt(a.resolvedAt)}` : ""}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}

function Meta({ label, value, tone, mono }: { label: string; value: string; tone?: string; mono?: boolean }) {
  return (
    <div>
      <div className="text-xs text-muted">{label}</div>
      <div className={`text-sm ${tone ?? ""} ${mono ? "font-mono" : ""}`}>{value}</div>
    </div>
  );
}
