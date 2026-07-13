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
      <div className="hidden md:flex w-2/5 relative overflow-hidden bg-gradient-to-br from-brand-500 via-brand-600 to-brand-700 text-white flex-col justify-between p-10">
        {/* decorative rings */}
        <div className="absolute -top-24 -right-24 w-80 h-80 rounded-full border-[28px] border-white/10" />
        <div className="absolute -bottom-32 -left-16 w-96 h-96 rounded-full border-[36px] border-white/5" />
        <div className="relative flex items-center gap-2">
          <span className="w-9 h-9 rounded-lg bg-white text-brand-600 grid place-items-center font-bold text-lg">U</span>
          <span className="text-xl font-semibold tracking-wide">unico</span>
        </div>
        <div className="relative">
          <h2 className="text-3xl font-semibold leading-tight">Şəbəkə Monitorinqi</h2>
          <p className="mt-3 text-brand-100 max-w-sm">UniFi və Kerio cihazları, klientlər və loglar — bir paneldə, real vaxtda.</p>
          <ul className="mt-6 space-y-2.5 text-sm text-brand-100">
            {["Canlı metriklər və alert bildirişləri", "AI köməkçi — sualını öz dilində ver", "Brendli Excel/PDF hesabatlar"].map((f) => (
              <li key={f} className="flex items-center gap-2.5">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" className="w-4 h-4 shrink-0 text-white">
                  <path d="M20 6L9 17l-5-5" />
                </svg>
                {f}
              </li>
            ))}
          </ul>
        </div>
        <p className="relative text-brand-200 text-sm">© Unico · unico.az</p>
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

          <button type="submit" disabled={busy} className="btn btn-primary w-full justify-center h-10 mt-3">
            {busy ? "Yoxlanılır..." : "Daxil ol"}
          </button>

          <p className="text-xs text-muted text-center mt-4">Hesablar administrator tərəfindən yaradılır.</p>
        </form>
      </div>
    </div>
  );
}
