import { useMemo, useState } from "react";
import { api, Client } from "../lib/api";
import { usePolling } from "../lib/refresh";
import { Accessor, bytesToNum, rateToNum, SortTh, useSort } from "../components/sortable";

const clientCols = ["Ad / MAC", "AP", "VLAN", "Siqnal", "Endirmə", "Yükləmə", "Data", "IP", "Qoşulma vaxtı"];
const clientAccessors: Accessor<Client>[] = [
  (c) => c.name || c.mac,
  (c) => c.ap,
  (c) => c.vlan,
  (c) => c.rssi,
  (c) => rateToNum(c.rx),
  (c) => rateToNum(c.tx),
  (c) => bytesToNum(c.data),
  (c) => c.ip,
  (c) => c.since,
];

function Signal({ rssi }: { rssi: number }) {
  // -50 great, -70 ok, -80 poor
  const bars = rssi >= -55 ? 4 : rssi >= -65 ? 3 : rssi >= -75 ? 2 : 1;
  const tone = bars >= 3 ? "bg-green-500" : bars === 2 ? "bg-amber-500" : "bg-red-500";
  return (
    <div className="flex items-center gap-2">
      <div className="flex items-end gap-0.5 h-4">
        {[1, 2, 3, 4].map((b) => (
          <span key={b} className={`w-1 rounded-sm ${b <= bars ? tone : "bg-line"}`} style={{ height: `${b * 3 + 2}px` }} />
        ))}
      </div>
      <span className="text-xs text-muted">{rssi} dBm</span>
    </div>
  );
}

export default function ClientsPage() {
  const { data } = usePolling<Client[]>(() => api.clients());
  const clients = data ?? [];
  const [q, setQ] = useState("");

  const rows = useMemo(
    () => clients.filter((c) => q === "" || c.name.toLowerCase().includes(q.toLowerCase()) || c.mac.includes(q)),
    [clients, q],
  );
  const { sorted, sort } = useSort(rows, clientAccessors);

  return (
    <div>
      <div className="flex items-center gap-2 mb-4">
        <input className="input flex-1 max-w-md" placeholder="Klient axtar (ad, MAC)..." value={q} onChange={(e) => setQ(e.target.value)} />
        <span className="pill bg-brand-50 text-brand-700">{clients.length} klient</span>
      </div>

      <div className="card overflow-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-muted">
              {clientCols.map((h, i) => (
                <SortTh key={h} label={h} i={i} sort={sort} />
              ))}
            </tr>
          </thead>
          <tbody>
            {sorted.map((c) => (
              <tr key={c.mac} className="odd:bg-page/60 hover:bg-page">
                <td className="px-3 py-2 border-b border-line">
                  <div className="font-medium">{c.name}</div>
                  <div className="font-mono text-xs text-muted">{c.mac}</div>
                </td>
                <td className="px-3 py-2 border-b border-line">{c.ap}</td>
                <td className="px-3 py-2 border-b border-line text-muted">{c.vlan}</td>
                <td className="px-3 py-2 border-b border-line"><Signal rssi={c.rssi} /></td>
                <td className="px-3 py-2 border-b border-line">{c.rx}</td>
                <td className="px-3 py-2 border-b border-line">{c.tx}</td>
                <td className="px-3 py-2 border-b border-line tabular-nums">{c.data}</td>
                <td className="px-3 py-2 border-b border-line font-mono text-xs">{c.ip}</td>
                <td className="px-3 py-2 border-b border-line text-muted">{c.since}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
