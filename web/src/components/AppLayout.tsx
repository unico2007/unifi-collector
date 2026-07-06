import { ReactNode } from "react";
import { NavLink, useLocation } from "react-router-dom";
import { useAuth } from "../lib/auth";

const groups = [
  {
    name: "Ümumi",
    items: [
      { to: "/", label: "Ümumi baxış", icon: "grid" },
      { to: "/traffic", label: "Trafik", icon: "activity" },
      { to: "/wifi", label: "WiFi analitika", icon: "wifi" },
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
      { to: "/logs", label: "Loglar", icon: "list" },
    ],
  },
];

const titles: Record<string, string> = {
  "/": "Ümumi baxış",
  "/traffic": "Trafik",
  "/wifi": "WiFi analitika",
  "/devices": "Cihazlar",
  "/clients": "Klientlər",
  "/firewall": "Firewall / Təhlükəsizlik",
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
  };
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" className={className}>
      <path d={p[name]} />
    </svg>
  );
}

export default function AppLayout({ children }: { children: ReactNode }) {
  const { user, logout } = useAuth();
  const loc = useLocation();
  const title = titles[loc.pathname] ?? (loc.pathname.startsWith("/devices/") ? "Cihaz detalı" : "");
  const initials = (user?.username ?? "?").slice(0, 2).toUpperCase();

  return (
    <div className="flex h-full">
      <aside className="w-60 shrink-0 bg-white border-r border-line flex flex-col">
        <div className="h-14 flex items-center gap-2 px-4 border-b border-line">
          <span className="w-7 h-7 rounded-lg bg-brand-500 text-white grid place-items-center font-semibold">U</span>
          <span className="font-semibold">unico</span>
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
        <header className="h-14 shrink-0 bg-white border-b border-line flex items-center gap-3 px-5">
          <h1 className="text-lg font-semibold">{title}</h1>
          <div className="ml-auto flex items-center gap-3">
            <div className="hidden sm:flex items-center gap-2 h-9 px-3 rounded-lg border border-line text-muted text-sm">
              <Icon name="search" className="w-4 h-4" />
              <span>Axtar...</span>
            </div>
            <button className="btn" aria-label="Yenilə"><Icon name="refresh" className="w-4 h-4" /></button>
            <div className="flex items-center gap-2">
              {user?.role === "guest" && (
                <span className="pill bg-amber-50 text-amber-700">Yalnız baxış</span>
              )}
              <span className="w-8 h-8 rounded-full bg-brand-100 text-brand-700 grid place-items-center text-sm font-medium">{initials}</span>
            </div>
          </div>
        </header>
        <main className="flex-1 overflow-auto p-5">{children}</main>
      </div>
    </div>
  );
}
