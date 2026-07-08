import { createContext, useContext, useEffect, useState, ReactNode } from "react";

export type Role = "admin" | "guest";
export interface User {
  username: string;
  role: Role;
}

interface AuthCtx {
  user: User | null;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
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

  async function login(username: string, password: string) {
    let r: Response;
    try {
      r = await fetch("/api/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ username, password }),
      });
    } catch {
      // fetch threw = the backend is unreachable (standalone dev with no BFF).
      // Only THEN fall back to demo mode. A reachable backend that rejects the
      // credentials returns a 401 response (not a throw) and must be honored —
      // otherwise any password would "log in", which is the whole bug this
      // guards against.
      if (!username) throw new Error("İstifadəçi adı və ya parol yanlışdır");
      const demo: User = { username, role: "admin" }; // dev-only: full UI browsable
      localStorage.setItem("unico_user", JSON.stringify(demo));
      setUser(demo);
      return;
    }
    // Backend answered: trust its verdict. 401/any non-2xx = wrong credentials.
    if (!r.ok) throw new Error("İstifadəçi adı və ya parol yanlışdır");
    // Role comes from the server (the account's DB role), never the UI toggle.
    const u = (await r.json()) as User;
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
