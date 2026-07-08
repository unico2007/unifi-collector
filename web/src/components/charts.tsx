// Dependency-free SVG chart kit for the Unico dashboard.
// Smooth curves, gradient fills, gridlines, and interactive hover tooltips —
// no external chart library (keeps the Docker build lean).

import { ReactNode, useId, useMemo, useState } from "react";

const BRAND = "#1466d6";
const GREEN = "#16a34a";
const AMBER = "#f59e0b";
const RED = "#ef4444";

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

/** Catmull-Rom → cubic-bezier smoothing so lines read as gentle curves. */
function smoothPath(pts: [number, number][]): string {
  if (pts.length < 2) return pts.map((p, i) => `${i ? "L" : "M"}${p[0]},${p[1]}`).join(" ");
  const t = 0.18; // tension
  let d = `M${pts[0][0].toFixed(1)},${pts[0][1].toFixed(1)}`;
  for (let i = 0; i < pts.length - 1; i++) {
    const p0 = pts[i - 1] ?? pts[i];
    const p1 = pts[i];
    const p2 = pts[i + 1];
    const p3 = pts[i + 2] ?? p2;
    const c1x = p1[0] + (p2[0] - p0[0]) * t;
    const c1y = p1[1] + (p2[1] - p0[1]) * t;
    const c2x = p2[0] - (p3[0] - p1[0]) * t;
    const c2y = p2[1] - (p3[1] - p1[1]) * t;
    d += ` C${c1x.toFixed(1)},${c1y.toFixed(1)} ${c2x.toFixed(1)},${c2y.toFixed(1)} ${p2[0].toFixed(1)},${p2[1].toFixed(1)}`;
  }
  return d;
}

function niceTicks(min: number, max: number, count = 3): number[] {
  if (max <= min) return [min];
  const step = (max - min) / count;
  return Array.from({ length: count + 1 }, (_, i) => min + step * i);
}

const fmtNum = (v: number) =>
  Math.abs(v) >= 1000 ? v.toLocaleString(undefined, { maximumFractionDigits: 0 }) : `${Math.round(v * 10) / 10}`;

// ---------------------------------------------------------------------------
// Card
// ---------------------------------------------------------------------------

export function Card({
  title,
  subtitle,
  right,
  children,
  className = "",
}: {
  title?: string;
  subtitle?: string;
  right?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={`card p-4 ${className}`}>
      {(title || right) && (
        <div className="flex items-start mb-3">
          <div>
            {title && <div className="text-sm font-semibold text-ink">{title}</div>}
            {subtitle && <div className="text-xs text-muted mt-0.5">{subtitle}</div>}
          </div>
          {right && <div className="ml-auto text-xs text-muted">{right}</div>}
        </div>
      )}
      {children}
    </div>
  );
}

// ---------------------------------------------------------------------------
// StatCard — unified KPI tile
// ---------------------------------------------------------------------------

const toneMap: Record<string, { text: string; bg: string; ring: string }> = {
  brand: { text: "text-brand-600", bg: "bg-brand-50", ring: "ring-brand-100" },
  green: { text: "text-green-600", bg: "bg-green-50", ring: "ring-green-100" },
  amber: { text: "text-amber-600", bg: "bg-amber-50", ring: "ring-amber-100" },
  red: { text: "text-red-600", bg: "bg-red-50", ring: "ring-red-100" },
  slate: { text: "text-ink", bg: "bg-slate-100", ring: "ring-slate-100" },
};

export function StatCard({
  label,
  value,
  sub,
  tone = "slate",
  icon,
  spark,
}: {
  label: string;
  value: string | number;
  sub?: string;
  tone?: keyof typeof toneMap;
  icon?: ReactNode;
  spark?: number[];
}) {
  const t = toneMap[tone] ?? toneMap.slate;
  return (
    <div className="card p-4 flex flex-col gap-1.5 transition-shadow hover:shadow-md">
      <div className="flex items-center gap-2">
        {icon && (
          <span className={`w-8 h-8 shrink-0 rounded-lg grid place-items-center ${t.bg} ${t.text} ring-1 ${t.ring}`}>{icon}</span>
        )}
        <div className="text-xs font-medium text-muted truncate min-w-0">{label}</div>
      </div>
      <div className="flex items-end justify-between gap-2">
        <div className={`text-2xl font-bold tabular-nums leading-none ${t.text}`}>{value}</div>
        {spark && (
          <div className="shrink-0">
            <Sparkline data={spark} color={tone === "slate" ? BRAND : undefined} />
          </div>
        )}
      </div>
      {sub && <div className="text-xs text-muted">{sub}</div>}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Time-series (one or two areas) with hover tooltip + gridlines
// ---------------------------------------------------------------------------

type Series = { data: number[]; color: string; label: string };

function TimeSeries({
  series,
  height = 150,
  unit = "",
  showAxis = true,
}: {
  series: Series[];
  height?: number;
  unit?: string;
  showAxis?: boolean;
}) {
  const uid = useId().replace(/:/g, "");
  const [hover, setHover] = useState<number | null>(null);
  const w = 600;
  const padX = 10;
  const padTop = 14;
  const padBottom = 8;
  const n = Math.max(...series.map((s) => s.data.length));

  const { min, max } = useMemo(() => {
    const all = series.flatMap((s) => s.data);
    let lo = Math.min(...all);
    let hi = Math.max(...all);
    if (lo === hi) {
      hi += 1;
      lo -= 1;
    }
    lo = Math.min(lo, 0) === 0 && lo >= 0 ? 0 : lo; // anchor to 0 when data is non-negative
    return { min: lo, max: hi };
  }, [series]);

  const nx = (i: number) => padX + (i * (w - padX * 2)) / (n - 1 || 1);
  const ny = (v: number) => padTop + (1 - (v - min) / (max - min || 1)) * (height - padTop - padBottom);
  const ticks = niceTicks(min, max, 3);

  function onMove(e: React.MouseEvent<SVGSVGElement>) {
    const rect = e.currentTarget.getBoundingClientRect();
    const frac = (e.clientX - rect.left) / rect.width;
    const vbx = frac * w;
    let idx = Math.round((vbx - padX) / ((w - padX * 2) / (n - 1 || 1)));
    idx = Math.max(0, Math.min(n - 1, idx));
    setHover(idx);
  }

  const hoverPct = hover != null ? (100 * nx(hover)) / w : 0;

  return (
    <div className="relative select-none" style={{ height }}>
      <svg
        viewBox={`0 0 ${w} ${height}`}
        preserveAspectRatio="none"
        className="w-full block overflow-visible"
        style={{ height }}
        onMouseMove={onMove}
        onMouseLeave={() => setHover(null)}
      >
        <defs>
          {series.map((s, i) => (
            <linearGradient key={i} id={`${uid}-g${i}`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={s.color} stopOpacity="0.28" />
              <stop offset="100%" stopColor={s.color} stopOpacity="0.02" />
            </linearGradient>
          ))}
        </defs>

        {/* gridlines */}
        {showAxis &&
          ticks.map((t, i) => (
            <line key={i} x1={padX} x2={w - padX} y1={ny(t)} y2={ny(t)} stroke="#eef2f7" strokeWidth="1" vectorEffect="non-scaling-stroke" />
          ))}

        {/* areas + lines */}
        {series.map((s, i) => {
          const pts = s.data.map((v, j) => [nx(j), ny(v)] as [number, number]);
          const line = smoothPath(pts);
          const area = `${line} L${nx(s.data.length - 1)},${height - padBottom} L${nx(0)},${height - padBottom} Z`;
          return (
            <g key={i}>
              <path d={area} fill={`url(#${uid}-g${i})`} />
              <path
                d={line}
                fill="none"
                stroke={s.color}
                strokeWidth="2.25"
                strokeLinecap="round"
                strokeLinejoin="round"
                vectorEffect="non-scaling-stroke"
                className="chart-draw"
              />
            </g>
          );
        })}

        {/* hover crosshair (vertical line; round dots are HTML overlays below) */}
        {hover != null && (
          <line x1={nx(hover)} x2={nx(hover)} y1={padTop} y2={height - padBottom} stroke="#cbd5e1" strokeWidth="1" strokeDasharray="3 3" vectorEffect="non-scaling-stroke" />
        )}
      </svg>

      {/* hover points — HTML overlay so they stay perfectly round despite x-stretch */}
      {hover != null &&
        series.map((s, i) => (
          <div
            key={i}
            className="pointer-events-none absolute w-2.5 h-2.5 rounded-full bg-white"
            style={{
              left: `${hoverPct}%`,
              top: `${(ny(s.data[hover] ?? 0) / height) * 100}%`,
              transform: "translate(-50%, -50%)",
              border: `2.5px solid ${s.color}`,
            }}
          />
        ))}

      {/* y-axis labels (overlay) */}
      {showAxis && (
        <div className="pointer-events-none absolute inset-0">
          {ticks.map((t, i) => (
            <div
              key={i}
              className="absolute left-0 text-[10px] text-slate-400 tabular-nums"
              style={{ top: `${(ny(t) / height) * 100}%`, transform: "translateY(-50%)" }}
            >
              {fmtNum(t)}
            </div>
          ))}
        </div>
      )}

      {/* tooltip */}
      {hover != null && (
        <div
          className="pointer-events-none absolute z-10 -translate-x-1/2 rounded-lg bg-ink/90 px-2.5 py-1.5 text-xs text-white shadow-lg backdrop-blur"
          style={{ left: `${Math.max(12, Math.min(88, hoverPct))}%`, top: 0 }}
        >
          {series.map((s, i) => (
            <div key={i} className="flex items-center gap-1.5 whitespace-nowrap">
              <span className="w-2 h-2 rounded-full" style={{ background: s.color }} />
              <span className="text-white/70">{s.label}</span>
              <span className="ml-auto font-semibold tabular-nums">
                {fmtNum(s.data[hover] ?? 0)}
                {unit && ` ${unit}`}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export function AreaLine({ data, color = BRAND, height = 150, unit = "" }: { data: number[]; color?: string; height?: number; unit?: string }) {
  return <TimeSeries series={[{ data, color, label: "" }]} height={height} unit={unit} />;
}

export function DualArea({
  a,
  b,
  height = 170,
  labelA = "Endirmə (RX)",
  labelB = "Yükləmə (TX)",
  colorA = BRAND,
  colorB = GREEN,
  unit = "",
}: {
  a: number[];
  b: number[];
  height?: number;
  labelA?: string;
  labelB?: string;
  colorA?: string;
  colorB?: string;
  unit?: string;
}) {
  return (
    <div>
      <TimeSeries
        series={[
          { data: a, color: colorA, label: labelA },
          { data: b, color: colorB, label: labelB },
        ]}
        height={height}
        unit={unit}
      />
      <div className="flex gap-4 text-xs text-muted mt-2">
        <Legend color={colorA} label={labelA} />
        <Legend color={colorB} label={labelB} />
      </div>
    </div>
  );
}

function Legend({ color, label }: { color: string; label: string }) {
  return (
    <span className="flex items-center gap-1.5">
      <span className="w-2.5 h-2.5 rounded-full" style={{ background: color }} />
      {label}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Donut
// ---------------------------------------------------------------------------

export function Donut({
  value,
  total,
  label,
  sublabel,
  color = GREEN,
  size = 168,
}: {
  value: number;
  total: number;
  label?: string;
  sublabel?: string;
  color?: string;
  size?: number;
}) {
  const uid = useId().replace(/:/g, "");
  const frac = total > 0 ? value / total : 0;
  const r = 42;
  const c = 2 * Math.PI * r;
  return (
    <div className="relative mx-auto" style={{ width: size, height: size }}>
      <svg viewBox="0 0 100 100" className="w-full h-full -rotate-90">
        <defs>
          <linearGradient id={`${uid}-d`} x1="0" y1="0" x2="1" y2="1">
            <stop offset="0%" stopColor={color} />
            <stop offset="100%" stopColor={color} stopOpacity="0.65" />
          </linearGradient>
        </defs>
        <circle cx="50" cy="50" r={r} fill="none" stroke="#eef2f7" strokeWidth="11" />
        <circle
          cx="50"
          cy="50"
          r={r}
          fill="none"
          stroke={`url(#${uid}-d)`}
          strokeWidth="11"
          strokeDasharray={`${frac * c} ${c}`}
          strokeLinecap="round"
          className="donut-anim"
          style={{ transition: "stroke-dasharray .7s cubic-bezier(.4,0,.2,1)" }}
        />
      </svg>
      <div className="absolute inset-0 grid place-content-center text-center">
        <div className="text-3xl font-bold text-ink tabular-nums">{Math.round(frac * 100)}%</div>
        {label && <div className="text-xs text-muted mt-0.5">{label}</div>}
        {sublabel && <div className="text-[11px] text-slate-400 mt-0.5">{sublabel}</div>}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Gauge (radial, color by value)
// ---------------------------------------------------------------------------

export function Gauge({ value, label, tone }: { value: number; label: string; tone?: string }) {
  const uid = useId().replace(/:/g, "");
  const auto = value >= 85 ? RED : value >= 65 ? AMBER : BRAND;
  const color = tone ?? auto;
  const r = 42;
  const c = 2 * Math.PI * r;
  return (
    <div className="relative w-32 h-32">
      <svg viewBox="0 0 100 100" className="w-32 h-32 -rotate-90">
        <defs>
          <linearGradient id={`${uid}-gg`} x1="0" y1="0" x2="1" y2="1">
            <stop offset="0%" stopColor={color} />
            <stop offset="100%" stopColor={color} stopOpacity="0.6" />
          </linearGradient>
        </defs>
        <circle cx="50" cy="50" r={r} fill="none" stroke="#eef2f7" strokeWidth="10" />
        <circle
          cx="50"
          cy="50"
          r={r}
          fill="none"
          stroke={`url(#${uid}-gg)`}
          strokeWidth="10"
          strokeDasharray={`${(Math.min(100, Math.max(0, value)) / 100) * c} ${c}`}
          strokeLinecap="round"
          style={{ transition: "stroke-dasharray .7s cubic-bezier(.4,0,.2,1)" }}
        />
      </svg>
      <div className="absolute inset-0 grid place-content-center text-center">
        <div className="text-2xl font-bold text-ink tabular-nums">{Math.round(value)}%</div>
        <div className="text-[11px] text-muted">{label}</div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// TopBars — ranked horizontal bars
// ---------------------------------------------------------------------------

export function TopBars({ rows, unit = "" }: { rows: { label: string; value: number; sub?: string }[]; unit?: string }) {
  const max = Math.max(...rows.map((r) => r.value), 1);
  if (rows.length === 0) return <div className="text-sm text-muted py-6 text-center">Məlumat yoxdur</div>;
  return (
    <div className="space-y-2.5">
      {rows.map((r, i) => (
        <div key={r.label} className="group flex items-center gap-3 text-sm">
          <div className="w-5 h-5 shrink-0 grid place-items-center rounded-md bg-slate-100 text-[11px] font-semibold text-slate-500 tabular-nums">
            {i + 1}
          </div>
          <div className="w-24 truncate shrink-0 font-medium" title={r.label}>
            {r.label}
          </div>
          <div className="flex-1 h-2.5 rounded-full bg-slate-100 overflow-hidden">
            <div
              className="h-full rounded-full bar-grow"
              style={{
                width: `${(r.value / max) * 100}%`,
                background: `linear-gradient(90deg, ${BRAND}, #4f92ec)`,
                minWidth: r.value > 0 ? 4 : 0,
              }}
            />
          </div>
          <div className="w-20 text-right text-xs shrink-0 tabular-nums">
            <span className="font-semibold text-ink">{r.value.toLocaleString()}</span>
            {unit && <span className="text-muted">{unit}</span>}
            {r.sub ? <span className="text-muted"> · {r.sub}</span> : ""}
          </div>
        </div>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Histogram — vertical bars with hover value
// ---------------------------------------------------------------------------

export function Histogram({ bins, labels, unit = "" }: { bins: number[]; labels: string[]; unit?: string }) {
  const uid = useId().replace(/:/g, "");
  const [hover, setHover] = useState<number | null>(null);
  const max = Math.max(...bins, 1);
  return (
    <div>
      <div className="flex items-end gap-1.5 h-40">
        {bins.map((b, i) => (
          <div
            key={i}
            className="flex-1 flex flex-col items-center justify-end gap-1 h-full"
            onMouseEnter={() => setHover(i)}
            onMouseLeave={() => setHover(null)}
          >
            <div className={`text-[10px] font-semibold tabular-nums transition-opacity ${hover === i ? "opacity-100 text-ink" : "opacity-0"}`}>
              {b}
              {unit}
            </div>
            <div
              className="w-full rounded-t-md bar-rise transition-all"
              style={{
                height: `${(b / max) * 100}%`,
                minHeight: b > 0 ? 3 : 0,
                background:
                  hover === i
                    ? `linear-gradient(180deg, ${BRAND}, ${BRAND})`
                    : `linear-gradient(180deg, #4f92ec, ${BRAND})`,
                opacity: hover == null || hover === i ? 1 : 0.55,
              }}
              id={`${uid}-${i}`}
            />
            <div className="text-[10px] text-muted">{labels[i]}</div>
          </div>
        ))}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Sparkline
// ---------------------------------------------------------------------------

export function Sparkline({ data, color = BRAND }: { data: number[]; color?: string }) {
  const w = 80,
    h = 24,
    pad = 3;
  const min = Math.min(...data),
    max = Math.max(...data);
  const nx = (i: number) => pad + (i * (w - pad * 2)) / (data.length - 1 || 1);
  const ny = (v: number) => h - pad - ((v - min) / (max - min || 1)) * (h - pad * 2);
  const pts = data.map((v, i) => [nx(i), ny(v)] as [number, number]);
  const last = pts[pts.length - 1];
  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="w-16 h-6">
      <path d={smoothPath(pts)} fill="none" stroke={color} strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" />
      {last && <circle cx={last[0]} cy={last[1]} r="2" fill={color} />}
    </svg>
  );
}
