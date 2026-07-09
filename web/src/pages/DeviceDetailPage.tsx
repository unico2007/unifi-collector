import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api, DeviceDetail, TimeRange } from "../lib/api";
import { usePolling } from "../lib/refresh";
import { Card, AreaLine, Gauge, RangeSelector, rangeLabel, rangeTicks } from "../components/charts";
import { PageSkeleton } from "../components/Skeleton";

const fmtMbps = (v: number) => (v >= 10 ? Math.round(v) : Math.round(v * 10) / 10);

export default function DeviceDetailPage() {
  const { name = "" } = useParams();
  const [range, setRange] = useState<TimeRange>("24h");
  const { data: d } = usePolling<DeviceDetail>(() => api.device(name, range), [name, range]);
  if (!d) return <PageSkeleton stats={4} cards={2} />;

  const dev = d.device;
  const last = (a: number[]) => (a.length ? a[a.length - 1] : 0);
  const curRx = fmtMbps(last(d.rx));
  const curTx = fmtMbps(last(d.tx));
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

      <div className="grid lg:grid-cols-3 gap-4">
        <Card title="CPU"><div className="grid place-items-center"><Gauge value={dev.cpu} label="indi" /></div></Card>
        <Card title="Yaddaş"><div className="grid place-items-center"><Gauge value={dev.memory} label="indi" tone="#f59e0b" /></div></Card>
        <Card title="Trafik (RX/TX indi)">
          <div className="grid place-items-center h-32 text-center">
            <div>
              <div className="text-2xl font-semibold text-brand-600">{curRx} <span className="text-sm text-muted">Mbps ↓</span></div>
              <div className="text-2xl font-semibold text-green-600 mt-1">{curTx} <span className="text-sm text-muted">Mbps ↑</span></div>
            </div>
          </div>
        </Card>
      </div>

      <div className="flex justify-end">
        <RangeSelector value={range} onChange={setRange} />
      </div>
      <div className="grid lg:grid-cols-2 gap-4">
        <Card title={`CPU (${rangeLabel[range]})`}><AreaLine data={d.cpu} xLabels={rangeTicks(range)} /></Card>
        <Card title={`Yaddaş (${rangeLabel[range]})`}><AreaLine data={d.memory} color="#f59e0b" xLabels={rangeTicks(range)} /></Card>
      </div>

      <Card title="Qoşulu klientlər">
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
      </Card>
    </div>
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
