import { useEffect, useMemo, useState } from "react";
import { api, Device } from "../lib/api";
import { useAuth } from "../lib/auth";

function VendorBadge({ v }: { v: string }) {
  const cls = v === "unifi" ? "bg-brand-50 text-brand-700" : "bg-orange-50 text-orange-700";
  return <span className={`pill ${cls}`}>{v === "unifi" ? "UniFi" : "Kerio"}</span>;
}

function Bar({ pct }: { pct: number }) {
  const tone = pct > 80 ? "bg-red-500" : pct > 60 ? "bg-amber-500" : "bg-brand-500";
  return (
    <div className="flex items-center gap-2">
      <div className="w-16 h-1.5 rounded-full bg-line overflow-hidden">
        <div className={`h-full ${tone}`} style={{ width: `${pct}%` }} />
      </div>
      <span className="text-xs text-muted">{pct}%</span>
    </div>
  );
}

export default function DevicesPage() {
  const { user } = useAuth();
  const [devices, setDevices] = useState<Device[]>([]);
  const [vendor, setVendor] = useState("all");
  const [status, setStatus] = useState("all");
  const [q, setQ] = useState("");

  useEffect(() => {
    api.devices().then(setDevices);
  }, []);

  const rows = useMemo(
    () =>
      devices.filter(
        (d) =>
          (vendor === "all" || d.vendor === vendor) &&
          (status === "all" || d.state === status) &&
          (q === "" || d.name.toLowerCase().includes(q.toLowerCase()) || d.ip.includes(q)),
      ),
    [devices, vendor, status, q],
  );

  return (
    <div>
      <div className="flex items-center gap-2 mb-4 flex-wrap">
        <select className="input" value={vendor} onChange={(e) => setVendor(e.target.value)}>
          <option value="all">Bütün vendorlar</option>
          <option value="unifi">UniFi</option>
          <option value="kerio">Kerio</option>
        </select>
        <select className="input" value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="all">Bütün statuslar</option>
          <option value="online">Online</option>
          <option value="offline">Offline</option>
        </select>
        <input className="input flex-1 min-w-[160px]" placeholder="Cihaz axtar..." value={q} onChange={(e) => setQ(e.target.value)} />
        {user?.role === "admin" && <button className="btn btn-primary">Yenilə</button>}
      </div>

      <div className="card overflow-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-muted">
              {["Ad", "Vendor", "Tip", "IP", "MAC", "Status", "CPU", "Yaddaş", "Uptime"].map((h) => (
                <th key={h} className="font-medium px-3 py-2 border-b border-line whitespace-nowrap">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((d) => (
              <tr key={d.name} className="odd:bg-page/60 hover:bg-page">
                <td className="px-3 py-2 border-b border-line font-medium">{d.name}</td>
                <td className="px-3 py-2 border-b border-line"><VendorBadge v={d.vendor} /></td>
                <td className="px-3 py-2 border-b border-line text-muted">{d.type}</td>
                <td className="px-3 py-2 border-b border-line font-mono text-xs">{d.ip}</td>
                <td className="px-3 py-2 border-b border-line font-mono text-xs">{d.mac}</td>
                <td className="px-3 py-2 border-b border-line">
                  <span className={`pill ${d.state === "online" ? "bg-green-50 text-green-700" : "bg-red-50 text-red-700"}`}>
                    {d.state === "online" ? "Online" : "Offline"}
                  </span>
                </td>
                <td className="px-3 py-2 border-b border-line">{d.state === "online" ? <Bar pct={d.cpu} /> : <span className="text-muted">-</span>}</td>
                <td className="px-3 py-2 border-b border-line">{d.state === "online" ? <Bar pct={d.memory} /> : <span className="text-muted">-</span>}</td>
                <td className="px-3 py-2 border-b border-line text-muted">{d.uptime}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
