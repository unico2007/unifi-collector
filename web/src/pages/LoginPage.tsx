import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../lib/auth";

export default function LoginPage() {
  const { login } = useAuth();
  const nav = useNavigate();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [show, setShow] = useState(false);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setErr("");
    setBusy(true);
    try {
      await login(username, password);
      nav("/");
    } catch {
      setErr("İstifadəçi adı və ya parol yanlışdır");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="h-full flex">
      <div className="hidden md:flex w-2/5 bg-brand-600 text-white flex-col justify-between p-10">
        <div className="flex items-center gap-2">
          <span className="w-9 h-9 rounded-lg bg-white text-brand-600 grid place-items-center font-bold text-lg">U</span>
          <span className="text-xl font-semibold">unico</span>
        </div>
        <div>
          <h2 className="text-3xl font-semibold leading-tight">Şəbəkə Monitorinqi</h2>
          <p className="mt-3 text-brand-100 max-w-sm">UniFi və Kerio cihazları, klientlər və loglar — bir paneldə, real vaxtda.</p>
        </div>
        <p className="text-brand-200 text-sm">© Unico</p>
      </div>

      <div className="flex-1 grid place-items-center p-6">
        <form onSubmit={submit} className="w-full max-w-sm card p-7">
          <div className="flex items-center gap-2 mb-6">
            <span className="w-7 h-7 rounded-lg bg-brand-500 text-white grid place-items-center font-semibold">U</span>
            <span className="font-semibold text-lg">Daxil ol</span>
          </div>

          <label className="block text-sm text-muted mb-1">İstifadəçi adı</label>
          <input className="input w-full mb-4" value={username} onChange={(e) => setUsername(e.target.value)} placeholder="admin" autoFocus />

          <label className="block text-sm text-muted mb-1">Parol</label>
          <div className="relative mb-2">
            <input
              className="input w-full pr-10"
              type={show ? "text" : "password"}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
            />
            <button type="button" onClick={() => setShow((s) => !s)} className="absolute right-3 top-2.5 text-muted text-sm">
              {show ? "gizlət" : "göstər"}
            </button>
          </div>

          {err && <p className="text-sm text-red-600 mb-2">{err}</p>}

          <label className="flex items-center gap-2 text-sm text-muted my-3">
            <input type="checkbox" className="accent-brand-500" /> Məni xatırla
          </label>

          <button type="submit" disabled={busy} className="btn btn-primary w-full justify-center h-10">
            {busy ? "Yoxlanılır..." : "Daxil ol"}
          </button>

          <p className="text-xs text-muted text-center mt-4">Hesablar administrator tərəfindən yaradılır.</p>
        </form>
      </div>
    </div>
  );
}
