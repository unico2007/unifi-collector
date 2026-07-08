import { createContext, useContext, useEffect, useState, ReactNode } from "react";

export type Role = "admin" | "guest";
export interface User {
  username: string;
  role: Role;
}

interface AuthCtx {
  user: User | null;
  loading: boolean;
  login: (username: string, password: string, role: Role) => Promise<void>;
  logout: () => void;
}

const Ctx = createContext<AuthCtx>(null!);
export const useAuth = () => useContext(Ctx);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  // Start in a loading state so routing waits for auth to resolve — otherwise
  // a refresh on a sub-page briefly sees user=null and bounces to Overview.
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const raw = localStorage.getItem("unico_user");
    if (!raw) {
      setLoading(false);
      return;
    }
    setUser(JSON.parse(raw)); // optimistic, confirmed below
    // Verify the session cookie is still valid. If it's gone (e.g. server
    // restarted with the old in-memory sessions), force a fresh login instead
    // of silently showing mock data.
    fetch("/api/me", { credentials: "include" })
      .then((r) => {
        if (r.status === 401) {
          localStorage.removeItem("unico_user");
          setUser(null);
        } else if (r.ok) {
          r.json().then((u) => setUser(u as User)).catch(() => {});
        }
        // Network/other errors (e.g. standalone dev without backend): keep the
        // optimistic user so the mock UI stays browsable.
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  async function login(username: string, password: string, role: Role) {
    let u: User;
    try {
      const r = await fetch("/api/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ username, password, role }),
      });
      if (!r.ok) throw new Error("auth");
      u = (await r.json()) as User;
    } catch {
      // No backend yet — demo mode. Accept the chosen role locally.
      if (!username) throw new Error("İstifadəçi adı və ya parol yanlışdır");
      u = { username, role };
    }
    localStorage.setItem("unico_user", JSON.stringify(u));
    setUser(u);
  }

  function logout() {
    localStorage.removeItem("unico_user");
    setUser(null);
    fetch("/api/logout", { method: "POST", credentials: "include" }).catch(() => {});
  }

  return <Ctx.Provider value={{ user, loading, login, logout }}>{children}</Ctx.Provider>;
}
