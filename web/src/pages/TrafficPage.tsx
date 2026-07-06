import { useEffect, useState } from "react";
import { api, Traffic } from "../lib/api";
import { Card, DualArea, TopBars } from "../components/charts";

export default function TrafficPage() {
  const [t, setT] = useState<Traffic | null>(null);
  useEffect(() => {
    api.traffic().then(setT);
  }, []);
  if (!t) return <div className="text-muted">Yüklənir...</div>;

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <Kpi label="Cəmi endirmə (RX)" value={t.totalRx} />
        <Kpi label="Cəmi yükləmə (TX)" value={t.totalTx} />
        <Kpi label="Aktiv sessiyalar" value="1,204" />
        <Kpi label="Ən yüksək sürət" value="486 Mbps" />
      </div>

      <Card title="Bant genişliyi (24 saat)">
        <DualArea a={t.rx} b={t.tx} />
      </Card>

      <div className="grid lg:grid-cols-2 gap-4">
        <Card title="Ən çox trafik istifadə edənlər">
          <TopBars rows={t.topTalkers} />
        </Card>
        <Card title="AP-yə görə trafik (GB)">
          <TopBars rows={t.perAp} />
        </Card>
      </div>
    </div>
  );
}

function Kpi({ label, value }: { label: string; value: string }) {
  return (
    <div className="card p-4">
      <div className="text-sm text-muted">{label}</div>
      <div className="text-2xl font-semibold mt-1">{value}</div>
    </div>
  );
}
