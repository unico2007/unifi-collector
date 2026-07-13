import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, Device, Client } from "../lib/api";
import { useRefresh } from "../lib/refresh";

// Pages the palette can jump to (mirrors the sidebar).
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

type Item = { label: string; sub?: string; run: () => void };
const INVENTORY_TTL = 60000; // refetch devices/clients when older than this

// GlobalSearch is the header quick-search, upgraded to a ⌘K-style palette:
// Cmd/Ctrl+K focuses it, an empty query lists pages + actions, arrows move the
// selection, Enter runs it. Inventory is fetched lazily and refreshed when
// stale so newly-appeared devices are findable mid-session.
export default function GlobalSearch() {
  const nav = useNavigate();
  const { refresh } = useRefresh();
  const [q, setQ] = useState("");
  const [open, setOpen] = useState(false);
  const [sel, setSel] = useState(0);
  const [devices, setDevices] = useState<Device[]>([]);
  const [clients, setClients] = useState<Client[]>([]);
  const loadedAt = useRef(0);
  const boxRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  function ensureData() {
    if (Date.now() - loadedAt.current < INVENTORY_TTL) return;
    loadedAt.current = Date.now();
    api.devices().then(setDevices).catch(() => {});
    api.clients().then(setClients).catch(() => {});
  }

  useEffect(() => {
    function onDoc(e: MouseEvent) {
      if (boxRef.current && !boxRef.current.contains(e.target as Node)) setOpen(false);
    }
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        ensureData();
        setOpen(true);
        inputRef.current?.focus();
      }
    }
    document.addEventListener("mousedown", onDoc);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDoc);
      document.removeEventListener("keydown", onKey);
    };
  }, []);

  function close() {
    setQ("");
    setSel(0);
    setOpen(false);
    inputRef.current?.blur();
  }

  const groups = useMemo(() => {
    const term = q.trim().toLowerCase();
    const has = (s: string) => s.toLowerCase().includes(term);
    const go = (to: string) => () => {
      nav(to);
      close();
    };
    const out: { title: string; items: Item[] }[] = [];

    const pageHits = pages.filter((p) => !term || has(p.label)).map((p) => ({ label: p.label, run: go(p.to) }));
    if (pageHits.length) out.push({ title: "Səhifələr", items: term ? pageHits : pageHits.slice(0, 6) });

    if (term) {
      const devHits = devices
        .filter((d) => has(d.name) || d.ip.includes(term))
        .slice(0, 6)
        .map((d) => ({ label: d.name, sub: d.model || d.type, run: go(`/devices/${encodeURIComponent(d.name)}`) }));
      const cliHits = clients
        .filter((c) => has(c.name || "") || has(c.mac || "") || (c.ip || "").includes(term))
        .slice(0, 6)
        .map((c) => ({ label: c.name || c.mac, sub: c.ap, run: go("/clients") }));
      if (devHits.length) out.push({ title: "Cihazlar", items: devHits });
      if (cliHits.length) out.push({ title: "Klientlər", items: cliHits });
    }

    const actions: Item[] = [
      {
        label: "Temanı dəyiş",
        sub: "işıqlı / qaranlıq",
        run: () => {
          window.dispatchEvent(new CustomEvent("unico:toggle-theme"));
          close();
        },
      },
      {
        label: "Datanı yenilə",
        sub: "bütün panellər",
        run: () => {
          refresh();
          close();
        },
      },
    ].filter((a) => !term || has(a.label));
    if (actions.length) out.push({ title: "Əməliyyatlar", items: actions });

    return out;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [q, devices, clients]);

  const flat = groups.flatMap((g) => g.items);
  const selClamped = Math.min(sel, Math.max(0, flat.length - 1));

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
        ref={inputRef}
        className="input w-56 pl-8 pr-10"
        placeholder="Axtar..."
        value={q}
        onFocus={() => {
          ensureData();
          setOpen(true);
        }}
        onChange={(e) => {
          setQ(e.target.value);
          setSel(0);
          setOpen(true);
        }}
        onKeyDown={(e) => {
          if (e.key === "Escape") {
            close();
          } else if (e.key === "ArrowDown") {
            e.preventDefault();
            setSel((s) => Math.min(s + 1, flat.length - 1));
          } else if (e.key === "ArrowUp") {
            e.preventDefault();
            setSel((s) => Math.max(s - 1, 0));
          } else if (e.key === "Enter" && flat[selClamped]) {
            flat[selClamped].run();
          }
        }}
      />
      <kbd className="absolute right-2 top-1/2 -translate-y-1/2 pointer-events-none text-[10px] text-slate-400 border border-line rounded px-1 py-0.5 bg-page">
        ⌘K
      </kbd>
      {open && (
        <div className="absolute right-0 mt-1 w-80 card p-1 max-h-96 overflow-auto z-20 shadow-lg">
          {flat.length === 0 && <div className="px-3 py-2 text-sm text-muted">Nəticə yoxdur</div>}
          {(() => {
            let idx = -1;
            return groups.map((g) => (
              <div key={g.title} className="mb-1 last:mb-0">
                <div className="px-2 py-1 text-[11px] uppercase tracking-wide text-slate-400">{g.title}</div>
                {g.items.map((it, i) => {
                  idx += 1;
                  const active = idx === selClamped;
                  const myIdx = idx;
                  return (
                    <button
                      key={`${g.title}-${i}`}
                      onClick={it.run}
                      onMouseEnter={() => setSel(myIdx)}
                      className={`w-full flex items-center justify-between gap-2 px-2 py-1.5 rounded-lg text-sm text-left ${
                        active ? "bg-brand-50 text-brand-700" : "hover:bg-page"
                      }`}
                    >
                      <span className="truncate">{it.label}</span>
                      {it.sub && <span className="text-xs text-slate-400 truncate shrink-0">{it.sub}</span>}
                    </button>
                  );
                })}
              </div>
            ));
          })()}
        </div>
      )}
    </div>
  );
}
