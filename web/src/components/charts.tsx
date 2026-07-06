// Lightweight dependency-free SVG charts, styled for the Unico light theme.

const BRAND = "#1466d6";

export function Card({ title, right, children, className = "" }: { title?: string; right?: React.ReactNode; children: React.ReactNode; className?: string }) {
  return (
    <div className={`card p-4 ${className}`}>
      {(title || right) && (
        <div className="flex items-center mb-3">
          {title && <div className="text-sm font-medium">{title}</div>}
          {right && <div className="ml-auto text-xs text-muted">{right}</div>}
        </div>
      )}
      {children}
    </div>
  );
}

export function AreaLine({ data, color = BRAND, height = 140 }: { data: number[]; color?: string; height?: number }) {
  const w = 560, pad = 6;
  const min = Math.min(...data), max = Math.max(...data);
  const nx = (i: number) => pad + (i * (w - pad * 2)) / (data.length - 1);
  const ny = (v: number) => height - pad - ((v - min) / (max - min || 1)) * (height - pad * 2);
  const line = data.map((v, i) => `${i ? "L" : "M"}${nx(i).toFixed(1)},${ny(v).toFixed(1)}`).join(" ");
  return (
    <svg viewBox={`0 0 ${w} ${height}`} className="w-full" style={{ height }}>
      <path d={`${line} L${nx(data.length - 1)},${height} L${nx(0)},${height} Z`} fill={color} fillOpacity="0.08" />
      <path d={line} fill="none" stroke={color} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function DualArea({ a, b, height = 150, labelA = "Endirmə (RX)", labelB = "Yükləmə (TX)" }: { a: number[]; b: number[]; height?: number; labelA?: string; labelB?: string }) {
  return (
    <div className="relative">
      <div className="absolute inset-0"><AreaLine data={b} color="#16a34a" height={height} /></div>
      <AreaLine data={a} color={BRAND} height={height} />
      <div className="flex gap-4 text-xs text-muted mt-1">
        <span className="flex items-center gap-1"><span className="w-2.5 h-2.5 rounded-sm" style={{ background: BRAND }} />{labelA}</span>
        <span className="flex items-center gap-1"><span className="w-2.5 h-2.5 rounded-sm" style={{ background: "#16a34a" }} />{labelB}</span>
      </div>
    </div>
  );
}

export function Gauge({ value, label, tone = BRAND }: { value: number; label: string; tone?: string }) {
  const c = 2 * Math.PI * 42;
  return (
    <div className="relative w-32 h-32">
      <svg viewBox="0 0 100 100" className="w-32 h-32 -rotate-90">
        <circle cx="50" cy="50" r="42" fill="none" stroke="#e9edf3" strokeWidth="10" />
        <circle cx="50" cy="50" r="42" fill="none" stroke={tone} strokeWidth="10" strokeDasharray={`${(value / 100) * c} ${c}`} strokeLinecap="round" />
      </svg>
      <div className="absolute inset-0 grid place-content-center text-center">
        <div className="text-xl font-semibold">{Math.round(value)}%</div>
        <div className="text-[11px] text-muted">{label}</div>
      </div>
    </div>
  );
}

export function TopBars({ rows, unit = "" }: { rows: { label: string; value: number; sub?: string }[]; unit?: string }) {
  const max = Math.max(...rows.map((r) => r.value), 1);
  return (
    <div className="space-y-2">
      {rows.map((r) => (
        <div key={r.label} className="flex items-center gap-3 text-sm">
          <div className="w-28 truncate shrink-0">{r.label}</div>
          <div className="flex-1 h-2 rounded-full bg-line overflow-hidden">
            <div className="h-full rounded-full" style={{ width: `${(r.value / max) * 100}%`, background: BRAND }} />
          </div>
          <div className="w-20 text-right text-muted text-xs shrink-0">
            {r.value.toLocaleString()}
            {unit}
            {r.sub ? ` · ${r.sub}` : ""}
          </div>
        </div>
      ))}
    </div>
  );
}

export function Histogram({ bins, labels }: { bins: number[]; labels: string[] }) {
  const max = Math.max(...bins, 1);
  return (
    <div className="flex items-end gap-1.5 h-36">
      {bins.map((b, i) => (
        <div key={i} className="flex-1 flex flex-col items-center gap-1">
          <div className="w-full rounded-t bg-brand-500/80" style={{ height: `${(b / max) * 100}%`, minHeight: 2 }} />
          <div className="text-[10px] text-muted">{labels[i]}</div>
        </div>
      ))}
    </div>
  );
}

export function Sparkline({ data, color = BRAND }: { data: number[]; color?: string }) {
  const w = 80, h = 22, pad = 2;
  const min = Math.min(...data), max = Math.max(...data);
  const nx = (i: number) => pad + (i * (w - pad * 2)) / (data.length - 1);
  const ny = (v: number) => h - pad - ((v - min) / (max - min || 1)) * (h - pad * 2);
  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="w-20 h-5">
      <path d={data.map((v, i) => `${i ? "L" : "M"}${nx(i).toFixed(1)},${ny(v).toFixed(1)}`).join(" ")} fill="none" stroke={color} strokeWidth="1.5" />
    </svg>
  );
}
