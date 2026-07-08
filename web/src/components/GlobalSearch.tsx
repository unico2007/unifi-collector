import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, Device, Client } from "../lib/api";

// Pages the search can jump to (mirrors the sidebar).
const pages = [
  { label: "Ümumi baxış", to: "/" },
  { label: "AI köməkçi", to: "/ai" },
  { label: "Trafik", to: "/traffic" },
  { label: "WiFi analitika", to: "/wifi" },
  { label: "Hesabatlar", to: "/reports" },
  { label: "Cihazlar", to: "/devices" },
  { label: "Klientlər", to: "/clients" },
  { label: "Firewall", to: "/firewall" },
  { label: "Alertlər", to: "/alerts" },
  { label: "Topologiya", to: "/topology" },
  { label: "Loglar", to: "/logs" },
];

type Item = { label: string; sub?: string; to: string };

// GlobalSearch is the header quick-search: type to find pages, devices and
// clients; Enter or click jumps there. Inventory is fetched lazily on first
// focus so it costs nothing until used.
export default function GlobalSearch() {
  const nav = useNavigate();
  const [q, setQ] = useState("");
  const [open, setOpen] = useState(false);
  const [devices, setDevices] = useState<Device[]>([]);
  const [clients, setClients] = useState<Client[]>([]);
  const [loaded, setLoaded] = useState(false);
  const boxRef = useRef<HTMLDivElement>(null);

  function ensureData() {
    if (loaded) return;
    setLoaded(true);
    api.devices().then(setDevices).catch(() => {});
    api.clients().then(setClients).catch(() => {});
  }

  useEffect(() => {
    function onDoc(e: MouseEvent) {
      if (boxRef.current && !boxRef.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, []);

  const groups = useMemo(() => {
    const term = q.trim().toLowerCase();
    if (!term) return [] as { title: string; items: Item[] }[];
    const has = (s: string) => s.toLowerCase().includes(term);
    const out: { title: string; items: Item[] }[] = [];
    const pageHits = pages.filter((p) => has(p.label)).map((p) => ({ label: p.label, to: p.to }));
    const devHits = devices
      .filter((d) => has(d.name))
      .slice(0, 6)
      .map((d) => ({ label: d.name, sub: d.model || d.type, to: `/devices/${encodeURIComponent(d.name)}` }));
    const cliHits = clients
      .filter((c) => has(c.name || "") || has(c.mac || ""))
      .slice(0, 6)
      .map((c) => ({ label: c.name || c.mac, sub: c.ap, to: "/clients" }));
    if (pageHits.length) out.push({ title: "Səhifələr", items: pageHits });
    if (devHits.length) out.push({ title: "Cihazlar", items: devHits });
    if (cliHits.length) out.push({ title: "Klientlər", items: cliHits });
    return out;
  }, [q, devices, clients]);

  const flat = groups.flatMap((g) => g.items);

  function go(to: string) {
    nav(to);
    setQ("");
    setOpen(false);
  }

  return (
    <div ref={boxRef} className="relative hidden md:block">
      <svg
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.8"
        strokeLinecap="round"
        strokeLinejoin="round"
        className="w-4 h-4 absolute left-2.5 top-1/2 -translate-y-1/2 text-slate-400 pointer-events-none"
      >
        <path d="M11 19a8 8 0 1 0 0-16 8 8 0 0 0 0 16zM21 21l-4.35-4.35" />
      </svg>
      <input
        className="input w-56 pl-8"
        placeholder="Axtar..."
        value={q}
        onFocus={() => {
          ensureData();
          setOpen(true);
        }}
        onChange={(e) => {
          setQ(e.target.value);
          setOpen(true);
        }}
        onKeyDown={(e) => {
          if (e.key === "Escape") {
            setOpen(false);
            (e.target as HTMLInputElement).blur();
          } else if (e.key === "Enter" && flat[0]) {
            go(flat[0].to);
          }
        }}
      />
      {open && q.trim() && (
        <div className="absolute right-0 mt-1 w-72 card p-1 max-h-96 overflow-auto z-20">
          {flat.length === 0 && <div className="px-3 py-2 text-sm text-muted">Nəticə yoxdur</div>}
          {groups.map((g) => (
            <div key={g.title} className="mb-1 last:mb-0">
              <div className="px-2 py-1 text-[11px] uppercase tracking-wide text-slate-400">{g.title}</div>
              {g.items.map((it, i) => (
                <button
                  key={`${g.title}-${i}`}
                  onClick={() => go(it.to)}
                  className="w-full flex items-center justify-between gap-2 px-2 py-1.5 rounded-lg text-sm text-left hover:bg-page"
                >
                  <span className="truncate">{it.label}</span>
                  {it.sub && <span className="text-xs text-slate-400 truncate shrink-0">{it.sub}</span>}
                </button>
              ))}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
