import { useEffect, useMemo, useState } from "react";
import { api, LogCategory } from "../lib/api";

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
  const [cats, setCats] = useState<LogCategory[]>([]);
  const [active, setActive] = useState<string>("filter");

  useEffect(() => {
    api.logCategories().then((c) => {
      setCats(c);
      if (c.length && !c.find((x) => x.key === active)) setActive(c[0].key);
    });
  }, []);

  const groups = useMemo(() => {
    const g: Record<string, LogCategory[]> = { unifi: [], kerio: [] };
    cats.forEach((c) => g[c.vendor].push(c));
    return g;
  }, [cats]);

  const cur = cats.find((c) => c.key === active);

  return (
    <div className="flex gap-4 h-full">
      <div className="w-52 shrink-0 card p-2 overflow-auto">
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
          <Stat label="Xəta" value="12" tone="text-red-600" />
          <Stat label="Xəbərdarlıq" value="34" tone="text-amber-600" />
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
            <thead className="sticky top-0 bg-white">
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
    <div className="rounded-xl bg-white border border-line px-4 py-3">
      <div className={`text-xl font-semibold ${tone ?? ""}`}>{value}</div>
      <div className="text-xs text-muted">{label}</div>
    </div>
  );
}
