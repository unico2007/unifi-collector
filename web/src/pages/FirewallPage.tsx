import { useEffect, useState } from "react";
import { api, Firewall } from "../lib/api";
import { Card, DualArea, TopBars } from "../components/charts";

export default function FirewallPage() {
  const [f, setF] = useState<Firewall | null>(null);
  useEffect(() => {
    api.firewall().then(setF);
  }, []);
  if (!f) return <div className="text-muted">Yüklənir...</div>;

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <Kpi label="Bu gün bloklanan" value={f.blockedToday.toLocaleString()} tone="text-red-600" />
        <Kpi label="İcazə verilən" value={f.allow.reduce((a, b) => a + b, 0).toLocaleString()} tone="text-green-600" />
        <Kpi label="Aşkarlanan hücum" value={String(f.attacks.length)} tone="text-amber-600" />
        <Kpi label="Aktiv qaydalar" value={String(f.topRules.length)} />
      </div>

      <Card title="İcazə / Blok (24 saat)">
        <DualArea a={f.allow} b={f.deny} labelA="İcazə" labelB="Blok" />
      </Card>

      <div className="grid lg:grid-cols-3 gap-4">
        <Card title="Ən çox bloklanan IP-lər">
          <TopBars rows={f.topBlockedIps} />
        </Card>
        <Card title="Ən çox işləyən qaydalar">
          <TopBars rows={f.topRules} />
        </Card>
        <Card title="Veb kateqoriyaları">
          <TopBars rows={f.webCategories} />
        </Card>
      </div>

      <Card title="Son hücumlar / IPS hadisələri">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-muted">
              {["Vaxt", "Növ", "Mənbə", "Əməl"].map((h) => (
                <th key={h} className="font-medium px-3 py-2 border-b border-line">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {f.attacks.map((a, i) => (
              <tr key={i} className="odd:bg-page/60">
                <td className="px-3 py-2 border-b border-line font-mono text-xs">{a.time}</td>
                <td className="px-3 py-2 border-b border-line">{a.type}</td>
                <td className="px-3 py-2 border-b border-line font-mono text-xs">{a.source}</td>
                <td className="px-3 py-2 border-b border-line"><span className="pill bg-red-50 text-red-700">{a.action}</span></td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
    </div>
  );
}

function Kpi({ label, value, tone }: { label: string; value: string; tone?: string }) {
  return (
    <div className="card p-4">
      <div className="text-sm text-muted">{label}</div>
      <div className={`text-2xl font-semibold mt-1 ${tone ?? ""}`}>{value}</div>
    </div>
  );
}
