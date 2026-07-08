import { api, Wifi } from "../lib/api";
import { usePolling } from "../lib/refresh";
import { Card, Histogram, TopBars } from "../components/charts";
import { PageSkeleton } from "../components/Skeleton";

export default function WifiPage() {
  const { data: w } = usePolling<Wifi>(() => api.wifi());
  if (!w) return <PageSkeleton stats={0} cards={3} />;

  const totalQ = w.quality.good + w.quality.fair + w.quality.poor || 1;

  return (
    <div className="space-y-4">
      <div className="grid lg:grid-cols-3 gap-4">
        <Card title="Siqnal keyfiyyəti" className="lg:col-span-1">
          <div className="space-y-2 mt-1">
            <QRow label="Yaxşı" value={w.quality.good} total={totalQ} tone="bg-green-500" />
            <QRow label="Orta" value={w.quality.fair} total={totalQ} tone="bg-amber-500" />
            <QRow label="Zəif" value={w.quality.poor} total={totalQ} tone="bg-red-500" />
          </div>
        </Card>
        <Card title="RSSI paylanması (dBm)" className="lg:col-span-2">
          <Histogram bins={w.rssiBins} labels={w.rssiLabels} />
        </Card>
      </div>

      <div className="grid lg:grid-cols-3 gap-4">
        <Card title="AP-yə görə klientlər">
          <TopBars rows={w.clientsPerAp} />
        </Card>
        <Card title="Tezlik zolağı">
          <TopBars rows={w.bandSplit} />
        </Card>
        <Card title="VLAN bölgüsü">
          <TopBars rows={w.vlanSplit} />
        </Card>
      </div>
    </div>
  );
}

function QRow({ label, value, total, tone }: { label: string; value: number; total: number; tone: string }) {
  return (
    <div className="flex items-center gap-3 text-sm">
      <div className="w-12 shrink-0">{label}</div>
      <div className="flex-1 h-2 rounded-full bg-line overflow-hidden">
        <div className={`h-full ${tone}`} style={{ width: `${(value / total) * 100}%` }} />
      </div>
      <div className="w-10 text-right text-muted text-xs">{value}</div>
    </div>
  );
}
