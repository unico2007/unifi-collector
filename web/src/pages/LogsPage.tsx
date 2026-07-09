import { useEffect, useMemo, useState } from "react";
import { api, LogCategory } from "../lib/api";
import { usePolling } from "../lib/refresh";

const pillClass: Record<string, string> = {
  ok: "bg-green-50 text-green-700",
  no: "bg-red-50 text-red-700",
  warn: "bg-amber-50 text-amber-700",
  info: "bg-slate-100 text-slate-600",
};

function Cell({ v }: { v: string | { text: string; kind: string } }) {
  if (typeof v === "string") return <span className={/[.:]/.test(v) ? "font-mono text-xs" : ""}>{v}</span>;
  return <span className={`pill ${pillClass[v.kind] ?? pillClass.info}`}>{v.text}</span>;
}

export default function LogsPage() {
  const { data } = usePolling<LogCategory[]>(() => api.logCategories());
  const cats = data ?? [];
  const [active, setActive] = useState<string>("filter");

  // Keep the selected tab valid as categories load / change.
  useEffect(() => {
    if (cats.length && !cats.find((x) => x.key === active)) setActive(cats[0].key);
  }, [cats, active]);

  const groups = useMemo(() => {
    const g: Record<string, LogCategory[]> = { unifi: [], kerio: [] };
    cats.forEach((c) => g[c.vendor].push(c));
    return g;
  }, [cats]);

  const cur = cats.find((c) => c.key === active);

  // Error/warning counts derived from the selected category's rows (level pill
  // sits in column 1). Honest counts instead of the old hardcoded 12 / 34.
  const levels = useMemo(() => {
    let err = 0,
      warn = 0;
    cur?.rows.forEach((row) => {
      const cell = row[1];
      if (typeof cell === "object") {
        if (cell.kind === "no") err++;
        else if (cell.kind === "warn") warn++;
      }
    });
    return { err, warn };
  }, [cur]);

  return (
    <div className="flex flex-col md:flex-row gap-4 md:h-full">
      <div className="w-full md:w-52 shrink-0 card p-2 overflow-auto max-h-56 md:max-h-none">
        {(["unifi", "kerio"] as const).map((v) => (
          <div key={v} className="mb-2">
            <div className="px-2 py-1 text-[11px] uppercase tracking-wide text-muted">{v === "unifi" ? "UniFi" : "Kerio"}</div>
            {groups[v]?.map((c) => (
              <button
                key={c.key}
                onClick={() => setActive(c.key)}
                className={`w-full flex items-center justify-between gap-2 px-2 py-1.5 rounded-lg text-sm text-left transition-colors ${
                  active === c.key ? "bg-brand-50 text-brand-700 font-medium" : "text-muted hover:bg-page"
                }`}
              >
                <span className="truncate">{c.label}</span>
                <span className={`text-xs ${active === c.key ? "text-brand-700" : "text-slate-400"}`}>
                  {c.count.toLocaleString()}
                </span>
              </button>
            ))}
          </div>
        ))}
      </div>

      <div className="flex-1 min-w-0 flex flex-col">
        <div className="grid grid-cols-3 gap-3 mb-3">
          <Stat label="Bu gün" value={cur ? cur.count.toLocaleString() : "-"} />
          <Stat label="Xəta" value={String(levels.err)} tone="text-red-600" />
          <Stat label="Xəbərdarlıq" value={String(levels.warn)} tone="text-amber-600" />
        </div>

        <div className="flex items-center gap-2 mb-3 flex-wrap">
          <span className="btn">Son 1 saat</span>
          <span className="btn">Səviyyə: Hamısı</span>
          <div className="btn flex-1 min-w-[140px] text-muted justify-start">Axtar...</div>
          <span className="btn">İxrac</span>
          <span className="pill bg-green-50 text-green-700">● Canlı</span>
        </div>

        <div className="card flex-1 overflow-auto">
          <table className="w-full text-sm">
            <thead className="sticky top-0 bg-card">
              <tr className="text-left text-muted">
                {cur?.columns.map((c) => (
                  <th key={c} className="font-medium px-3 py-2 border-b border-line whitespace-nowrap">{c}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {cur?.rows.map((row, i) => (
                <tr key={i} className="odd:bg-page/60">
                  {row.map((v, j) => (
                    <td key={j} className="px-3 py-2 border-b border-line whitespace-nowrap"><Cell v={v} /></td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

function Stat({ label, value, tone }: { label: string; value: string; tone?: string }) {
  return (
    <div className="rounded-xl bg-card border border-line px-4 py-3">
      <div className={`text-xl font-semibold ${tone ?? ""}`}>{value}</div>
      <div className="text-xs text-muted">{label}</div>
    </div>
  );
}
