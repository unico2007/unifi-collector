// Client-side report helpers: CSV download + a printable PDF-ready report.
// Everything is generated in the browser from the same /api data the pages use,
// so no extra backend endpoints are needed.

function csvEsc(v: unknown): string {
  const s = v === null || v === undefined ? "" : String(v);
  return /[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

function htmlEsc(v: unknown): string {
  return String(v ?? "").replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c] as string,
  );
}

export function toCSV(headers: string[], rows: (string | number)[][]): string {
  return [headers, ...rows].map((r) => r.map(csvEsc).join(",")).join("\r\n");
}

export function download(filename: string, content: string, mime = "text/csv") {
  // Prepend a UTF-8 BOM so Excel renders Azerbaijani characters correctly.
  const blob = new Blob(["﻿", content], { type: `${mime};charset=utf-8` });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

export function stamp(): string {
  return new Date().toISOString().slice(0, 16).replace("T", "_").replace(/:/g, "-");
}

interface ReportTable {
  title: string;
  headers: string[];
  rows: (string | number)[][];
}

// Builds a clean, print-styled report inside a hidden iframe (via srcdoc — no
// document.write) and triggers the browser print dialog, where the user picks
// "Save as PDF". All interpolated values are HTML-escaped.
export function printReport(title: string, kpis: { label: string; value: string }[], tables: ReportTable[]) {
  const kpiHtml = kpis
    .map((k) => `<div class="kpi"><div class="kl">${htmlEsc(k.label)}</div><div class="kv">${htmlEsc(k.value)}</div></div>`)
    .join("");
  const tableHtml = tables
    .map(
      (t) => `
      <h2>${htmlEsc(t.title)}</h2>
      <table><thead><tr>${t.headers.map((h) => `<th>${htmlEsc(h)}</th>`).join("")}</tr></thead>
      <tbody>${t.rows
        .map((r) => `<tr>${r.map((c) => `<td>${htmlEsc(c)}</td>`).join("")}</tr>`)
        .join("")}</tbody></table>`,
    )
    .join("");

  const doc = `<!doctype html><html lang="az"><head><meta charset="utf-8"><title>${htmlEsc(title)}</title>
    <style>
      body{font-family:-apple-system,Segoe UI,system-ui,sans-serif;color:#0f172a;margin:32px;}
      h1{font-size:22px;margin:0 0 4px;}
      .meta{color:#64748b;font-size:13px;margin-bottom:20px;}
      .kpis{display:flex;gap:14px;flex-wrap:wrap;margin-bottom:24px;}
      .kpi{border:1px solid #e2e8f0;border-radius:10px;padding:12px 16px;min-width:120px;}
      .kl{font-size:12px;color:#64748b;} .kv{font-size:22px;font-weight:700;margin-top:2px;}
      h2{font-size:15px;margin:22px 0 8px;}
      table{border-collapse:collapse;width:100%;font-size:12px;margin-bottom:8px;}
      th,td{border:1px solid #e2e8f0;padding:6px 9px;text-align:left;}
      th{background:#f6f8fc;text-transform:uppercase;font-size:10.5px;letter-spacing:.05em;color:#64748b;}
      @media print{@page{margin:14mm;}}
    </style></head><body>
    <h1>${htmlEsc(title)}</h1>
    <div class="meta">Unico Şəbəkə Monitorinq · ${htmlEsc(new Date().toLocaleString("az"))}</div>
    <div class="kpis">${kpiHtml}</div>
    ${tableHtml}
    </body></html>`;

  const iframe = document.createElement("iframe");
  iframe.setAttribute("aria-hidden", "true");
  iframe.style.cssText = "position:fixed;right:0;bottom:0;width:0;height:0;border:0;";
  iframe.srcdoc = doc;
  iframe.onload = () => {
    const win = iframe.contentWindow;
    if (win) {
      win.focus();
      win.print();
    }
    setTimeout(() => iframe.remove(), 1500);
  };
  document.body.appendChild(iframe);
}
