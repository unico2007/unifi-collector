import { ReactNode, useState } from "react";
import { NavLink, useLocation } from "react-router-dom";
import { useAuth } from "../lib/auth";
import { useRefresh } from "../lib/refresh";
import GlobalSearch from "./GlobalSearch";

const groups = [
  {
    name: "Ümumi",
    items: [
      { to: "/", label: "Ümumi baxış", icon: "grid" },
      { to: "/ai", label: "AI köməkçi", icon: "spark" },
      { to: "/traffic", label: "Trafik", icon: "activity" },
      { to: "/wifi", label: "WiFi analitika", icon: "wifi" },
      { to: "/reports", label: "Hesabatlar", icon: "file" },
    ],
  },
  {
    name: "İnventar",
    items: [
      { to: "/devices", label: "Cihazlar", icon: "server" },
      { to: "/clients", label: "Klientlər", icon: "users" },
    ],
  },
  {
    name: "Təhlükəsizlik",
    items: [
      { to: "/firewall", label: "Firewall", icon: "shield" },
      { to: "/alerts", label: "Alertlər", icon: "bell" },
      { to: "/topology", label: "Topologiya", icon: "share" },
      { to: "/logs", label: "Loglar", icon: "list" },
    ],
  },
];

const titles: Record<string, string> = {
  "/": "Ümumi baxış",
  "/ai": "AI köməkçi",
  "/traffic": "Trafik",
  "/wifi": "WiFi analitika",
  "/devices": "Cihazlar",
  "/clients": "Klientlər",
  "/firewall": "Firewall / Təhlükəsizlik",
  "/alerts": "Alertlər",
  "/topology": "Topologiya",
  "/reports": "Hesabatlar",
  "/logs": "Loglar",
};

function Icon({ name, className = "w-5 h-5" }: { name: string; className?: string }) {
  const p: Record<string, string> = {
    grid: "M3 3h7v7H3zM14 3h7v7h-7zM14 14h7v7h-7zM3 14h7v7H3z",
    server: "M3 4h18v6H3zM3 14h18v6H3zM7 7h.01M7 17h.01",
    users: "M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM23 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75",
    list: "M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01",
    logout: "M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4M16 17l5-5-5-5M21 12H9",
    search: "M11 19a8 8 0 1 0 0-16 8 8 0 0 0 0 16zM21 21l-4.35-4.35",
    refresh: "M23 4v6h-6M1 20v-6h6M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15",
    activity: "M22 12h-4l-3 9L9 3l-3 9H2",
    wifi: "M5 12.55a11 11 0 0 1 14.08 0M1.42 9a16 16 0 0 1 21.16 0M8.53 16.11a6 6 0 0 1 6.95 0M12 20h.01",
    shield: "M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z",
    spark: "M12 3l1.9 4.8L18 9l-4.1 1.2L12 15l-1.9-4.8L6 9l4.1-1.2zM19 14l.9 2.1L22 17l-2.1.9L19 20l-.9-2.1L16 17l2.1-.9z",
    bell: "M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9M13.7 21a2 2 0 0 1-3.4 0",
    share: "M18 8a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM6 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM18 22a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM8.6 13.5l6.8 3.9M15.4 6.6l-6.8 3.9",
    file: "M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8zM14 2v6h6M16 13H8M16 17H8M10 9H8",
    moon: "M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z",
    sun: "M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8z",
    menu: "M3 6h18M3 12h18M3 18h18",
    x: "M18 6L6 18M6 6l12 12",
  };
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" className={className}>
      <path d={p[name]} />
    </svg>
  );
}

export default function AppLayout({ children }: { children: ReactNode }) {
  const { user, logout } = useAuth();
  const { refresh, refreshing, lastUpdated } = useRefresh();
  const loc = useLocation();
  const title = titles[loc.pathname] ?? (loc.pathname.startsWith("/devices/") ? "Cihaz detalı" : "");
  const initials = (user?.username ?? "?").slice(0, 2).toUpperCase();
  const updatedAt = lastUpdated
    ? lastUpdated.toLocaleTimeString("az", { hour: "2-digit", minute: "2-digit", second: "2-digit" })
    : "—";

  const [dark, setDark] = useState(() => document.documentElement.classList.contains("dark"));
  function toggleTheme() {
    const next = !dark;
    document.documentElement.classList.toggle("dark", next);
    localStorage.setItem("unico_theme", next ? "dark" : "light");
    setDark(next);
  }

  // On mobile the sidebar is an off-canvas drawer toggled by a header hamburger.
  const [menuOpen, setMenuOpen] = useState(false);

  return (
    <div className="flex h-full">
      {menuOpen && (
        <div className="fixed inset-0 z-30 bg-black/40 md:hidden" onClick={() => setMenuOpen(false)} />
      )}
      <aside
        className={`fixed md:static inset-y-0 left-0 z-40 w-60 shrink-0 bg-card border-r border-line flex flex-col transition-transform md:translate-x-0 ${
          menuOpen ? "translate-x-0" : "-translate-x-full"
        }`}
      >
        <div className="h-14 flex items-center gap-2 px-4 border-b border-line">
          <span className="w-7 h-7 rounded-lg bg-brand-500 text-white grid place-items-center font-semibold">U</span>
          <span className="font-semibold">unico</span>
          <button className="btn ml-auto md:hidden px-2" aria-label="Bağla" onClick={() => setMenuOpen(false)}>
            <Icon name="x" className="w-4 h-4" />
          </button>
        </div>
        <nav className="flex-1 p-2 overflow-y-auto">
          {groups.map((g) => (
            <div key={g.name} className="mb-3">
              <div className="px-3 py-1 text-[11px] uppercase tracking-wide text-slate-400">{g.name}</div>
              <div className="space-y-0.5">
                {g.items.map((n) => (
                  <NavLink
                    key={n.to}
                    to={n.to}
                    end={n.to === "/"}
                    onClick={() => setMenuOpen(false)}
                    className={({ isActive }) =>
                      `flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors ${
                        isActive ? "bg-brand-50 text-brand-700 font-medium" : "text-muted hover:bg-page"
                      }`
                    }
                  >
                    <Icon name={n.icon} />
                    {n.label}
                  </NavLink>
                ))}
              </div>
            </div>
          ))}
        </nav>
        <div className="p-2 border-t border-line">
          <button onClick={logout} className="flex items-center gap-3 px-3 py-2 rounded-lg text-sm text-muted hover:bg-page w-full">
            <Icon name="logout" />
            Çıxış
          </button>
        </div>
      </aside>

      <div className="flex-1 flex flex-col min-w-0">
        <header className="h-14 shrink-0 bg-card border-b border-line flex items-center gap-3 px-3 md:px-5">
          <button className="btn md:hidden px-2" aria-label="Menyu" onClick={() => setMenuOpen(true)}>
            <Icon name="menu" className="w-5 h-5" />
          </button>
          <h1 className="text-lg font-semibold truncate">{title}</h1>
          <div className="ml-auto flex items-center gap-3">
            <GlobalSearch />
            <div className="hidden md:flex items-center gap-1.5 text-xs text-muted">
              <span className="w-1.5 h-1.5 rounded-full bg-green-500" />
              <span>Son yeniləmə {updatedAt}</span>
            </div>
            <button className="btn" aria-label="Tema" title={dark ? "İşıqlı rejim" : "Qaranlıq rejim"} onClick={toggleTheme}>
              <Icon name={dark ? "sun" : "moon"} className="w-4 h-4" />
            </button>
            <button className="btn" aria-label="Yenilə" title="Yenilə" onClick={refresh} disabled={refreshing}>
              <Icon name="refresh" className={`w-4 h-4 ${refreshing ? "animate-spin" : ""}`} />
            </button>
            <div className="flex items-center gap-2">
              {user?.role === "guest" && (
                <span className="pill bg-amber-50 text-amber-700">Yalnız baxış</span>
              )}
              <span className="w-8 h-8 rounded-full bg-brand-100 text-brand-700 grid place-items-center text-sm font-medium">{initials}</span>
            </div>
          </div>
        </header>
        <main className="flex-1 overflow-auto p-3 md:p-5">{children}</main>
      </div>
    </div>
  );
}
