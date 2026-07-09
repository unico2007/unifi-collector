import { useMemo, useState } from "react";

// A per-column value extractor. Return a number for numeric columns (sorted
// numerically) or a string for text columns (sorted with locale compare).
export type Accessor<T> = (row: T) => string | number;

export type Sort = {
  idx: number | null;
  dir: "asc" | "desc";
  toggle: (i: number) => void;
};

// useSort sorts `rows` by the column the user clicked. Accessors must be stable
// (define them at module scope) so sorting is memoised.
export function useSort<T>(rows: T[], accessors: Accessor<T>[]): { sorted: T[]; sort: Sort } {
  const [idx, setIdx] = useState<number | null>(null);
  const [dir, setDir] = useState<"asc" | "desc">("asc");

  const sorted = useMemo(() => {
    if (idx === null || !accessors[idx]) return rows;
    const acc = accessors[idx];
    const out = [...rows].sort((a, b) => {
      const va = acc(a);
      const vb = acc(b);
      let cmp: number;
      if (typeof va === "number" && typeof vb === "number") cmp = va - vb;
      else cmp = String(va).localeCompare(String(vb), "az");
      return dir === "asc" ? cmp : -cmp;
    });
    return out;
  }, [rows, idx, dir, accessors]);

  function toggle(i: number) {
    if (idx === i) setDir((d) => (d === "asc" ? "desc" : "asc"));
    else {
      setIdx(i);
      setDir("asc");
    }
  }

  return { sorted, sort: { idx, dir, toggle } };
}

// SortTh is a clickable table header cell showing the active sort direction.
export function SortTh({ label, i, sort }: { label: string; i: number; sort: Sort }) {
  const active = sort.idx === i;
  return (
    <th
      onClick={() => sort.toggle(i)}
      className="font-medium px-3 py-2 border-b border-line whitespace-nowrap cursor-pointer select-none hover:text-ink"
    >
      <span className="inline-flex items-center gap-1">
        {label}
        <span className={`text-[10px] ${active ? "text-brand-600" : "text-slate-300"}`}>
          {active ? (sort.dir === "asc" ? "▲" : "▼") : "↕"}
        </span>
      </span>
    </th>
  );
}

// rateToNum normalises a formatted rate ("108 Mbps", "500 Kbps", "0") to bits/s
// so rate columns sort correctly regardless of unit.
export function rateToNum(s: string): number {
  const m = s.match(/([\d.]+)\s*(Kbps|Mbps|Gbps)?/i);
  if (!m) return 0;
  let n = parseFloat(m[1]);
  switch ((m[2] || "").toLowerCase()) {
    case "kbps":
      n *= 1e3;
      break;
    case "mbps":
      n *= 1e6;
      break;
    case "gbps":
      n *= 1e9;
      break;
  }
  return n;
}
