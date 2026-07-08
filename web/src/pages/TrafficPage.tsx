import { useEffect, useState } from "react";
import { api, Traffic } from "../lib/api";
import { Card, DualArea, StatCard, TopBars } from "../components/charts";

export default function TrafficPage() {
  const [t, setT] = useState<Traffic | null>(null);
  useEffect(() => {
    api.traffic().then(setT);
  }, []);
  if (!t) return <div className="text-muted">Yüklənir...</div>;

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Endirmə (RX)" value={t.totalRx} tone="brand" spark={t.rx} icon={<Ico d="M12 5v14M19 12l-7 7-7-7" />} />
        <StatCard label="Yükləmə (TX)" value={t.totalTx} tone="green" spark={t.tx} icon={<Ico d="M12 19V5M5 12l7-7 7 7" />} />
        <StatCard label="Aktiv sessiyalar" value="1,204" tone="slate" icon={<Ico d="M22 12h-4l-3 9L9 3l-3 9H2" />} />
        <StatCard label="Ən yüksək sürət" value="486 Mbps" tone="amber" icon={<Ico d="M13 2L3 14h9l-1 8 10-12h-9z" />} />
      </div>

      <Card title="Bant genişliyi (24 saat)" subtitle="Endirmə və yükləmə">
        <DualArea a={t.rx} b={t.tx} />
      </Card>

      <div className="grid lg:grid-cols-2 gap-4">
        <Card title="Ən çox trafik istifadə edənlər">
          <TopBars rows={t.topTalkers} />
        </Card>
        <Card title="AP-yə görə trafik" subtitle="GB">
          <TopBars rows={t.perAp} unit=" GB" />
        </Card>
      </div>
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
