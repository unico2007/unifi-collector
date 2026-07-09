import { useEffect, useState } from "react";
import { api, AlertsData, AlertHistoryRow, AlertThresholds } from "../lib/api";
import { usePolling, useRefresh } from "../lib/refresh";
import { useAuth } from "../lib/auth";
import { StatCard } from "../components/charts";
import { PageSkeleton } from "../components/Skeleton";

const levelStyle: Record<string, { stripe: string; pill: string; label: string }> = {
  critical: { stripe: "bg-red-500", pill: "bg-red-50 text-red-700", label: "Kritik" },
  warning: { stripe: "bg-amber-500", pill: "bg-amber-50 text-amber-700", label: "Xəbərdarlıq" },
};

const fmtTime = (s: number) => new Date(s * 1000).toLocaleTimeString("az", { hour: "2-digit", minute: "2-digit" });

function fmtDuration(sec: number): string {
  if (sec < 60) return `${sec} san`;
  const m = Math.floor(sec / 60);
  if (m < 60) return `${m} dəq`;
  const h = Math.floor(m / 60);
  const rm = m % 60;
  return rm ? `${h} sa ${rm} dəq` : `${h} sa`;
}

export default function AlertsPage() {
  const { user } = useAuth();
  const { data: d } = usePolling<AlertsData>(() => api.alerts());
  const { data: history } = usePolling<AlertHistoryRow[]>(() => api.alertHistory());
  if (!d) return <PageSkeleton stats={4} cards={2} />;

  const healthy = d.active.length === 0;

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Kritik" value={d.counts.critical} tone={d.counts.critical ? "red" : "green"} icon={<Ico d="M12 9v4M12 17h.01M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />} />
        <StatCard label="Xəbərdarlıq" value={d.counts.warning} tone={d.counts.warning ? "amber" : "green"} icon={<Ico d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9M13.7 21a2 2 0 0 1-3.4 0" />} />
        <StatCard label="Aktiv qaydalar" value={d.rules.length} tone="slate" icon={<Ico d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01" />} />
        <StatCard label="Vəziyyət" value={healthy ? "Qaydasında" : "Diqqət"} tone={healthy ? "green" : "amber"} sub={healthy ? "bütün cihazlar həddlərdə" : "yoxlama tələb olunur"} icon={<Ico d={healthy ? "M20 6 9 17l-5-5" : "M12 9v4M12 17h.01"} />} />
      </div>

      {/* Active alerts */}
      <div className="card">
        <div className="px-4 py-3 border-b border-line text-sm font-medium">Aktiv alertlər</div>
        {healthy ? (
          <div className="p-8 text-center text-muted">
            <div className="text-2xl mb-1">✓</div>
            Heç bir aktiv alert yoxdur. Bütün cihazlar həddlər daxilindədir.
          </div>
        ) : (
          <div>
            {d.active.map((a, i) => {
              const s = levelStyle[a.level] ?? levelStyle.warning;
              return (
                <div key={i} className="flex items-stretch gap-3 px-4 py-3 border-b border-line last:border-0">
                  <div className={`w-1 rounded-full ${s.stripe}`} />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className={`pill ${s.pill}`}>{s.label}</span>
                      <span className="text-xs text-muted">{a.rule}</span>
                    </div>
                    <div className="text-sm mt-1 truncate">{a.message}</div>
                  </div>
                  <div className="flex items-center">
                    <span className="font-mono text-sm text-muted">{a.value}</span>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

      <div className="grid lg:grid-cols-2 gap-4">
        {/* Rules + editable thresholds */}
        <div className="card">
          <div className="px-4 py-3 border-b border-line text-sm font-medium">Qaydalar (həddlər)</div>
          <ThresholdEditor thresholds={d.thresholds} isAdmin={user?.role === "admin"} />
          <div className="divide-y divide-line">
            {d.rules.map((r, i) => (
              <div key={i} className="flex items-center gap-3 px-4 py-2.5">
                <span className={`pill ${(levelStyle[r.level] ?? levelStyle.warning).pill}`}>
                  {(levelStyle[r.level] ?? levelStyle.warning).label}
                </span>
                <span className="text-sm">{r.name}</span>
                <span className="ml-auto font-mono text-xs text-muted">{r.condition}</span>
              </div>
            ))}
          </div>
        </div>

        {/* History timeline */}
        <div className="card">
          <div className="px-4 py-3 border-b border-line text-sm font-medium">Alert tarixçəsi</div>
          {!history || history.length === 0 ? (
            <div className="p-8 text-center text-muted text-sm">Hələ qeyd olunmuş alert yoxdur.</div>
          ) : (
            <div className="divide-y divide-line max-h-96 overflow-auto">
              {history.map((h, i) => {
                const s = levelStyle[h.level] ?? levelStyle.warning;
                const active = h.resolvedAt === 0;
                const end = active ? Math.floor(Date.now() / 1000) : h.resolvedAt;
                return (
                  <div key={i} className="flex items-start gap-3 px-4 py-2.5">
                    <div className={`w-1.5 h-1.5 mt-1.5 rounded-full shrink-0 ${s.stripe}`} />
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="text-sm font-medium truncate">{h.target}</span>
                        <span className="text-xs text-muted">{h.rule}</span>
                        {active && <span className="pill bg-red-50 text-red-700">aktiv</span>}
                      </div>
                      <div className="text-xs text-muted mt-0.5 truncate">{h.message}</div>
                    </div>
                    <div className="text-right shrink-0">
                      <div className="text-xs font-mono text-muted whitespace-nowrap">
                        {fmtTime(h.firedAt)}
                        {!active && ` → ${fmtTime(h.resolvedAt)}`}
                      </div>
                      <div className="text-[11px] text-slate-400">{fmtDuration(end - h.firedAt)}</div>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </div>

      <NotifyStatus enabled={d.telegramEnabled} isAdmin={user?.role === "admin"} />

      <p className="text-xs text-muted">
        Qaydalar hər açılışda Prometheus-dan canlı qiymətləndirilir; tarixçə fonda hər 30 saniyədə yazılır.
        {user?.role === "admin" ? " CPU/yaddaş həddlərini yuxarıdan dəyişə bilərsiniz." : " Həddləri yalnız admin dəyişə bilər."}
      </p>
    </div>
  );
}

function NotifyStatus({ enabled, isAdmin }: { enabled: boolean; isAdmin: boolean }) {
  const [state, setState] = useState<"idle" | "sending" | "sent" | "fail">("idle");

  async function test() {
    setState("sending");
    try {
      const r = await api.testNotify();
      setState(r.sent ? "sent" : "fail");
    } catch {
      setState("fail");
    }
    setTimeout(() => setState("idle"), 4000);
  }

  return (
    <div className="card p-4 flex flex-wrap items-center gap-3">
      <span className="w-8 h-8 shrink-0 rounded-lg bg-brand-50 text-brand-600 grid place-items-center ring-1 ring-brand-100">
        <Ico d="M22 2 11 13M22 2l-7 20-4-9-9-4z" />
      </span>
      <div className="min-w-0">
        <div className="text-sm font-medium">Telegram bildirişləri</div>
        <div className="text-xs text-muted">
          {enabled
            ? "Aktiv — yeni alertlər avtomatik Telegram-a göndərilir"
            : "Deaktiv — serverin .env-inə WEB_TELEGRAM_TOKEN + WEB_TELEGRAM_CHAT_ID əlavə edin"}
        </div>
      </div>
      <span className={`ml-auto pill ${enabled ? "bg-green-50 text-green-700" : "bg-slate-100 text-slate-600"}`}>
        {enabled ? "aktiv" : "deaktiv"}
      </span>
      {isAdmin && enabled && (
        <button className="btn" onClick={test} disabled={state === "sending"}>
          {state === "sending" ? "Göndərilir..." : "Test bildirişi"}
        </button>
      )}
      {state === "sent" && <span className="text-xs text-green-600">✓ Göndərildi</span>}
      {state === "fail" && <span className="text-xs text-red-600">Göndərilmədi</span>}
    </div>
  );
}

function ThresholdEditor({ thresholds, isAdmin }: { thresholds: AlertThresholds; isAdmin: boolean }) {
  const { refresh } = useRefresh();
  const [cpu, setCpu] = useState(thresholds.cpuPercent);
  const [mem, setMem] = useState(thresholds.memoryPercent);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState(false);

  // Keep inputs in sync when the server value changes (e.g. another admin saved).
  useEffect(() => {
    setCpu(thresholds.cpuPercent);
    setMem(thresholds.memoryPercent);
  }, [thresholds.cpuPercent, thresholds.memoryPercent]);

  const dirty = cpu !== thresholds.cpuPercent || mem !== thresholds.memoryPercent;

  async function save() {
    setSaving(true);
    setSaved(false);
    setError(false);
    try {
      await api.updateAlertThresholds({ cpuPercent: cpu, memoryPercent: mem });
      setSaved(true);
      refresh(); // re-fetch alerts so rules + active list reflect the new limits
      setTimeout(() => setSaved(false), 2000);
    } catch {
      setError(true);
    } finally {
      setSaving(false);
    }
  }

  if (!isAdmin) return null;

  return (
    <div className="px-4 py-3 border-b border-line bg-page/40 flex flex-wrap items-end gap-4">
      <Field label="CPU həddi (%)" value={cpu} onChange={setCpu} />
      <Field label="Yaddaş həddi (%)" value={mem} onChange={setMem} />
      <button className="btn btn-primary" onClick={save} disabled={saving || !dirty}>
        {saving ? "Saxlanılır..." : "Yadda saxla"}
      </button>
      {saved && <span className="text-xs text-green-600">✓ Saxlanıldı</span>}
      {error && <span className="text-xs text-red-600">Saxlanmadı</span>}
    </div>
  );
}

function Field({ label, value, onChange }: { label: string; value: number; onChange: (v: number) => void }) {
  return (
    <label className="text-xs text-muted">
      <div className="mb-1">{label}</div>
      <input
        type="number"
        min={1}
        max={100}
        className="input w-24"
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
      />
    </label>
  );
}

function Ico({ d }: { d: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" className="w-4 h-4">
      <path d={d} />
    </svg>
  );
}
