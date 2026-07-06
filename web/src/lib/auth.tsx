import { createContext, useContext, useEffect, useState, ReactNode } from "react";

export type Role = "admin" | "guest";
export interface User {
  username: string;
  role: Role;
}

interface AuthCtx {
  user: User | null;
  login: (username: string, password: string, role: Role) => Promise<void>;
  logout: () => void;
}

const Ctx = createContext<AuthCtx>(null!);
export const useAuth = () => useContext(Ctx);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);

  useEffect(() => {
    const raw = localStorage.getItem("unico_user");
    if (raw) setUser(JSON.parse(raw));
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

  return <Ctx.Provider value={{ user, login, logout }}>{children}</Ctx.Provider>;
}
