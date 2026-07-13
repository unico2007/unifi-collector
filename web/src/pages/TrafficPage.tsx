import { useState } from "react";
import { api, Traffic, TimeRange } from "../lib/api";
import { usePolling } from "../lib/refresh";
import { Card, DualArea, RangeSelector, rangeLabel, rangeTicks, StatCard, TopBars } from "../components/charts";
import { PageSkeleton } from "../components/Skeleton";

const fmtMbps = (v: number) => `${v >= 10 ? Math.round(v) : Math.round(v * 10) / 10} Mbps`;

export default function TrafficPage() {
  const [range, setRange] = useState<TimeRange>("24h");
  const { data: t } = usePolling<Traffic>(() => api.traffic(range), [range]);
  if (!t) return <PageSkeleton stats={4} cards={2} />;

  // Derived from the real Mbps series the BFF returns (no hardcoded figures).
  const last = (a: number[]) => (a.length ? a[a.length - 1] : 0);
  const current = last(t.rx) + last(t.tx);
  const peak = t.rx.length ? Math.max(...t.rx.map((v, i) => v + (t.tx[i] ?? 0))) : 0;

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Endirmə (RX)" value={t.totalRx} tone="brand" spark={t.rx} icon={<Ico d="M12 5v14M19 12l-7 7-7-7" />} />
        <StatCard label="Yükləmə (TX)" value={t.totalTx} tone="green" spark={t.tx} icon={<Ico d="M12 19V5M5 12l7-7 7 7" />} />
        <StatCard label="Cari sürət" value={fmtMbps(current)} sub="RX+TX indi" tone="slate" icon={<Ico d="M22 12h-4l-3 9L9 3l-3 9H2" />} />
        <StatCard label="Ən yüksək sürət" value={fmtMbps(peak)} sub={`${rangeLabel[range]} zirvə`} tone="amber" icon={<Ico d="M13 2L3 14h9l-1 8 10-12h-9z" />} />
      </div>

      <Card title={`Bant genişliyi (${rangeLabel[range]})`} subtitle="Endirmə və yükləmə" right={<RangeSelector value={range} onChange={setRange} />}>
        <DualArea a={t.rx} b={t.tx} xLabels={rangeTicks(range)} />
      </Card>

      <div className="grid lg:grid-cols-2 gap-4">
        <Card title="Ən çox trafik istifadə edənlər">
          <TopBars rows={t.topTalkers} />
        </Card>
        <Card title="AP-yə görə trafik" subtitle="Mbps (cari)">
          <TopBars rows={t.perAp} unit=" Mbps" />
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
