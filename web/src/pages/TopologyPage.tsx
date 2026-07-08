import { useState } from "react";
import { api, Topology, TopoNode } from "../lib/api";
import { usePolling } from "../lib/refresh";
import { PageSkeleton } from "../components/Skeleton";

function StateDot({ state }: { state: string }) {
  const on = state === "online";
  return <span className={`inline-block w-2 h-2 rounded-full ${on ? "bg-green-500" : "bg-red-500"}`} />;
}

function rssiTone(rssi: number) {
  if (rssi >= -60) return "text-green-600";
  if (rssi >= -72) return "text-amber-600";
  return "text-red-600";
}

function TierLabel({ children }: { children: React.ReactNode }) {
  return <div className="text-[11px] uppercase tracking-wide text-slate-400 mb-2">{children}</div>;
}

function Connector() {
  return (
    <div className="flex flex-col items-center py-1 text-slate-300">
      <div className="w-px h-5 bg-line" />
    </div>
  );
}

function DeviceCard({ n, onClick, active }: { n: TopoNode; onClick?: () => void; active?: boolean }) {
  const clickable = !!onClick;
  return (
    <button
      onClick={onClick}
      disabled={!clickable}
      className={`card p-3 text-left transition-colors min-w-[150px] ${
        clickable ? "hover:border-brand-300 cursor-pointer" : "cursor-default"
      } ${active ? "border-brand-500 ring-2 ring-brand-100" : ""}`}
    >
      <div className="flex items-center gap-2">
        <StateDot state={n.state} />
        <span className="font-medium text-sm truncate">{n.name}</span>
      </div>
      <div className="text-xs text-muted mt-1">{n.model !== "-" ? n.model : n.type}</div>
      <div className="flex items-center gap-2 mt-2">
        {n.ip !== "-" && <span className="font-mono text-[11px] text-muted">{n.ip}</span>}
        {n.type === "uap" && (
          <span className="pill bg-brand-50 text-brand-700 ml-auto">{n.clients} klient</span>
        )}
      </div>
    </button>
  );
}

export default function TopologyPage() {
  const { data: d } = usePolling<Topology>(() => api.topology());
  const [sel, setSel] = useState<string | null>(null);
  if (!d) return <PageSkeleton stats={0} cards={2} />;

  const selClients = sel ? d.clientsByAp[sel] ?? [] : [];

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="card p-4"><div className="text-sm text-muted">Şəbəkə çıxışı</div><div className="text-2xl font-semibold mt-1">{d.edge.length}</div></div>
        <div className="card p-4"><div className="text-sm text-muted">Switch-lər</div><div className="text-2xl font-semibold mt-1">{d.stats.switches}</div></div>
        <div className="card p-4"><div className="text-sm text-muted">Access Point-lər</div><div className="text-2xl font-semibold mt-1">{d.stats.aps}</div></div>
        <div className="card p-4"><div className="text-sm text-muted">Klientlər</div><div className="text-2xl font-semibold mt-1">{d.stats.clients}</div></div>
      </div>

      <div className="card p-6">
        {/* Tier 1: edge / firewall */}
        <TierLabel>Şəbəkə çıxışı (Firewall / Gateway)</TierLabel>
        <div className="flex flex-wrap gap-3 justify-center">
          {d.edge.map((n) => <DeviceCard key={n.name} n={n} />)}
        </div>

        {(d.switches.length > 0) && (
          <>
            <Connector />
            <TierLabel>Switch-lər</TierLabel>
            <div className="flex flex-wrap gap-3 justify-center">
              {d.switches.map((n) => <DeviceCard key={n.name} n={n} />)}
            </div>
          </>
        )}

        <Connector />
        <TierLabel>Access Point-lər — klientləri görmək üçün kliklə</TierLabel>
        <div className="flex flex-wrap gap-3 justify-center">
          {d.aps.map((n) => (
            <DeviceCard
              key={n.name}
              n={n}
              active={sel === n.name}
              onClick={() => setSel(sel === n.name ? null : n.name)}
            />
          ))}
        </div>
      </div>

      {/* Selected AP's clients */}
      {sel && (
        <div className="card">
          <div className="px-4 py-3 border-b border-line flex items-center gap-2">
            <span className="text-sm font-medium">{sel}</span>
            <span className="text-xs text-muted">— {selClients.length} klient</span>
            <button onClick={() => setSel(null)} className="ml-auto text-xs text-muted hover:text-ink">bağla ✕</button>
          </div>
          {selClients.length === 0 ? (
            <div className="p-6 text-center text-muted text-sm">Bu AP-də klient yoxdur.</div>
          ) : (
            <div className="divide-y divide-line">
              {selClients.map((c) => (
                <div key={c.mac} className="flex items-center gap-3 px-4 py-2.5">
                  <span className="text-sm truncate flex-1">{c.name || c.mac}</span>
                  <span className="font-mono text-xs text-muted">{c.mac}</span>
                  <span className={`font-mono text-sm w-16 text-right ${rssiTone(c.rssi)}`}>{c.rssi} dBm</span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
