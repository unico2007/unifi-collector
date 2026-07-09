import { useState } from "react";
import { api, Topology, TopoNode } from "../lib/api";
import { usePolling } from "../lib/refresh";
import { PageSkeleton } from "../components/Skeleton";

function rssiTone(rssi: number) {
  if (rssi >= -60) return "text-green-600";
  if (rssi >= -72) return "text-amber-600";
  return "text-red-600";
}

function trunc(s: string, n: number) {
  return s.length > n ? s.slice(0, n - 1) + "…" : s;
}

// TopoGraph draws the network as a dependency-free node-link SVG: edge (gateway)
// on top, optional switch tier, then the APs fanning out below. Links are drawn
// from each tier to the centre of the tier above (the collector groups devices by
// type, not by physical port, so this shows the logical hierarchy). Click an AP.
function TopoGraph({
  d,
  sel,
  onSelect,
}: {
  d: Topology;
  sel: string | null;
  onSelect: (name: string) => void;
}) {
  const tiers: { key: string; nodes: TopoNode[]; selectable: boolean }[] = [
    { key: "edge", nodes: d.edge, selectable: false },
    ...(d.switches.length ? [{ key: "sw", nodes: d.switches, selectable: false }] : []),
    { key: "ap", nodes: d.aps, selectable: true },
  ];

  const slotW = 122;
  const nodeW = 106;
  const nodeH = 48;
  const rowGap = 82;
  const padY = 14;
  const maxCols = Math.max(...tiers.map((t) => t.nodes.length), 1);
  const width = maxCols * slotW;
  const height = tiers.length * (nodeH + rowGap) - rowGap + padY * 2;
  const rowY = (i: number) => padY + i * (nodeH + rowGap);
  const colX = (count: number, idx: number) => (width - count * slotW) / 2 + idx * slotW + slotW / 2;
  const hubX = width / 2;

  return (
    <div className="overflow-x-auto">
      <svg width={width} height={height} className="block mx-auto">
        {/* links: each node fans up to the centre of the tier above */}
        {tiers.map((t, ti) =>
          ti === 0
            ? null
            : t.nodes.map((n, ni) => {
                const x = colX(t.nodes.length, ni);
                const y = rowY(ti);
                const py = rowY(ti - 1) + nodeH;
                const mid = (py + y) / 2;
                return (
                  <path
                    key={`l-${t.key}-${ni}`}
                    d={`M${hubX},${py} C${hubX},${mid} ${x},${mid} ${x},${y}`}
                    className={sel === n.name ? "stroke-brand-400" : "stroke-line"}
                    fill="none"
                    strokeWidth="1.5"
                  />
                );
              }),
        )}

        {/* nodes */}
        {tiers.map((t, ti) =>
          t.nodes.map((n, ni) => {
            const x = colX(t.nodes.length, ni) - nodeW / 2;
            const y = rowY(ti);
            const active = sel === n.name;
            const on = n.state === "online";
            const sub = n.type === "uap" ? `${n.clients} klient` : n.model !== "-" ? n.model : n.type;
            return (
              <g
                key={`${t.key}-${n.name}`}
                transform={`translate(${x},${y})`}
                onClick={t.selectable ? () => onSelect(n.name) : undefined}
                className={t.selectable ? "cursor-pointer" : ""}
              >
                <rect
                  width={nodeW}
                  height={nodeH}
                  rx="10"
                  className={`fill-card ${active ? "stroke-brand-500" : "stroke-line"}`}
                  strokeWidth={active ? 2 : 1}
                />
                <circle cx="13" cy="16" r="3.5" className={on ? "fill-green-500" : "fill-red-500"} />
                <text x="24" y="19.5" className="fill-ink" style={{ fontSize: 11, fontWeight: 600 }}>
                  {trunc(n.name, 13)}
                </text>
                <text x="13" y="36" className="fill-muted" style={{ fontSize: 9.5 }}>
                  {trunc(sub, 15)}
                </text>
              </g>
            );
          }),
        )}
      </svg>
    </div>
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

      <div className="card p-4">
        <div className="text-[11px] uppercase tracking-wide text-slate-400 mb-2">
          Şəbəkə xəritəsi — AP-yə kliklə (klientləri gör)
        </div>
        <TopoGraph d={d} sel={sel} onSelect={(name) => setSel(sel === name ? null : name)} />
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
