import { api, Firewall } from "../lib/api";
import { usePolling } from "../lib/refresh";
import { Card, DualArea, StatCard, TopBars } from "../components/charts";
import { PageSkeleton } from "../components/Skeleton";

export default function FirewallPage() {
  const { data: f } = usePolling<Firewall>(() => api.firewall());
  if (!f) return <PageSkeleton stats={4} cards={2} />;

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Bu gün bloklanan" value={f.blockedToday.toLocaleString()} tone="red" spark={f.deny} icon={<Ico d="M18 6 6 18M6 6l12 12" />} />
        <StatCard label="İcazə verilən" value={f.allow.reduce((a, b) => a + b, 0).toLocaleString()} tone="green" spark={f.allow} icon={<Ico d="M20 6 9 17l-5-5" />} />
        <StatCard label="Aşkarlanan hücum" value={f.attacks.length} tone="amber" icon={<Ico d="M12 9v4M12 17h.01M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />} />
        <StatCard label="Aktiv qaydalar" value={f.topRules.length} tone="slate" icon={<Ico d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />} />
      </div>

      <Card title="İcazə / Blok (24 saat)" subtitle="Firewall qərarları">
        <DualArea a={f.allow} b={f.deny} labelA="İcazə" labelB="Blok" colorA="#16a34a" colorB="#ef4444" />
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

function Ico({ d }: { d: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" className="w-4 h-4">
      <path d={d} />
    </svg>
  );
}
