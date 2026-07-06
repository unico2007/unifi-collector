import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api, DeviceDetail } from "../lib/api";
import { Card, AreaLine, Gauge } from "../components/charts";

export default function DeviceDetailPage() {
  const { name = "" } = useParams();
  const [d, setD] = useState<DeviceDetail | null>(null);
  useEffect(() => {
    api.device(name).then(setD);
  }, [name]);
  if (!d) return <div className="text-muted">Yüklənir...</div>;

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

      <div className="grid lg:grid-cols-3 gap-4">
        <Card title="CPU"><div className="grid place-items-center"><Gauge value={dev.cpu} label="indi" /></div></Card>
        <Card title="Yaddaş"><div className="grid place-items-center"><Gauge value={dev.memory} label="indi" tone="#f59e0b" /></div></Card>
        <Card title="Trafik (RX/TX indi)">
          <div className="grid place-items-center h-32 text-center">
            <div>
              <div className="text-2xl font-semibold text-brand-600">42 <span className="text-sm text-muted">Mbps ↓</span></div>
              <div className="text-2xl font-semibold text-green-600 mt-1">14 <span className="text-sm text-muted">Mbps ↑</span></div>
            </div>
          </div>
        </Card>
      </div>

      <div className="grid lg:grid-cols-2 gap-4">
        <Card title="CPU (24 saat)"><AreaLine data={d.cpu} /></Card>
        <Card title="Yaddaş (24 saat)"><AreaLine data={d.memory} color="#f59e0b" /></Card>
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
