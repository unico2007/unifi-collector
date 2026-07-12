// Branded .xlsx export (Unico design). Built with exceljs, which is loaded via
// a dynamic import so the ~250 KB library lands in its own lazy chunk and is
// only fetched the first time a user actually exports a report.
//
// Layout of every sheet:
//   row 1  — brand band: "UNICO" + report title (brand blue fill, white text)
//   row 2  — meta line: subtitle · generated timestamp (Baku time)
//   row 3  — (spacer)
//   row 4  — column headers (dark blue fill, white bold, autofilter, frozen)
//   row 5+ — data rows with zebra striping and soft borders

// Unico brand palette — keep in sync with tailwind.config.js `colors.brand`.
const BRAND = {
  primary: "FF1466D6", // brand-500
  dark: "FF0E4796", //   brand-700
  band50: "FFEAF1FB", // brand-50 (zebra fill)
  line: "FFD5DFEE", //   soft border between rows
  ink: "FF0F172A", //    body text
  muted: "FF64748B", //  meta text
  white: "FFFFFFFF",
  red: "FFDC2626",
  green: "FF15803D",
  amber: "FFB45309",
};

export interface XlsxColumn {
  header: string;
  width: number;
  /** right-align numeric-looking columns */
  align?: "left" | "right" | "center";
}

export interface XlsxSheet {
  name: string;
  columns: XlsxColumn[];
  rows: (string | number)[][];
  /** optional conditional text color per cell, e.g. status columns */
  cellColor?: (colIndex: number, value: string | number) => string | undefined;
}

export interface XlsxReport {
  /** report title shown in the brand band, e.g. "Cihazlar" */
  title: string;
  /** short description under the band */
  subtitle: string;
  sheets: XlsxSheet[];
}

function bakuStamp(): string {
  return new Intl.DateTimeFormat("az-Latn-AZ", {
    dateStyle: "long",
    timeStyle: "short",
    timeZone: "Asia/Baku",
  }).format(new Date());
}

// buildXlsx renders the report to an .xlsx ArrayBuffer. Pure (no DOM), so it
// can also run under Node for tests/verification.
export async function buildXlsx(report: XlsxReport): Promise<ArrayBuffer> {
  const ExcelJS = (await import("exceljs")).default;
  const wb = new ExcelJS.Workbook();
  wb.creator = "Unico Şəbəkə Monitorinqi";
  wb.created = new Date();

  for (const sheet of report.sheets) {
    const ws = wb.addWorksheet(sheet.name, {
      views: [{ state: "frozen", ySplit: 4 }],
    });
    const colCount = sheet.columns.length;
    ws.columns = sheet.columns.map((c) => ({ width: c.width }));

    // Row 1 — brand band.
    ws.mergeCells(1, 1, 1, colCount);
    const band = ws.getCell(1, 1);
    band.value = `UNICO  ·  ${report.title}`;
    band.fill = { type: "pattern", pattern: "solid", fgColor: { argb: BRAND.primary } };
    band.font = { name: "Calibri", size: 15, bold: true, color: { argb: BRAND.white } };
    band.alignment = { vertical: "middle", horizontal: "left", indent: 1 };
    ws.getRow(1).height = 34;

    // Row 2 — meta line.
    ws.mergeCells(2, 1, 2, colCount);
    const meta = ws.getCell(2, 1);
    meta.value = `${report.subtitle}  ·  ${bakuStamp()}  ·  unico.az`;
    meta.font = { name: "Calibri", size: 10, color: { argb: BRAND.muted } };
    meta.alignment = { vertical: "middle", horizontal: "left", indent: 1 };
    ws.getRow(2).height = 18;

    // Row 4 — column headers.
    const headerRow = ws.getRow(4);
    sheet.columns.forEach((c, i) => {
      const cell = headerRow.getCell(i + 1);
      cell.value = c.header;
      cell.fill = { type: "pattern", pattern: "solid", fgColor: { argb: BRAND.dark } };
      cell.font = { name: "Calibri", size: 11, bold: true, color: { argb: BRAND.white } };
      cell.alignment = { vertical: "middle", horizontal: c.align ?? "left", wrapText: true };
    });
    headerRow.height = 22;
    ws.autoFilter = {
      from: { row: 4, column: 1 },
      to: { row: 4, column: colCount },
    };

    // Data rows.
    sheet.rows.forEach((r, ri) => {
      const row = ws.getRow(5 + ri);
      r.forEach((v, ci) => {
        const cell = row.getCell(ci + 1);
        cell.value = v;
        const color = sheet.cellColor?.(ci, v) ?? BRAND.ink;
        cell.font = { name: "Calibri", size: 10.5, color: { argb: color } };
        cell.alignment = { vertical: "middle", horizontal: sheet.columns[ci]?.align ?? "left" };
        if (ri % 2 === 1) {
          cell.fill = { type: "pattern", pattern: "solid", fgColor: { argb: BRAND.band50 } };
        }
        cell.border = { bottom: { style: "thin", color: { argb: BRAND.line } } };
      });
      row.height = 18;
    });
  }

  return wb.xlsx.writeBuffer() as Promise<ArrayBuffer>;
}

export async function downloadXlsx(filename: string, report: XlsxReport): Promise<void> {
  const buf = await buildXlsx(report);
  const blob = new Blob([buf], {
    type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

/** Text colors for common status-ish values (shared by report builders). */
export function statusColor(v: string | number): string | undefined {
  const s = String(v).toLowerCase();
  if (s === "online" || s === "✓ online") return BRAND.green;
  if (s === "offline" || s === "✗ offline") return BRAND.red;
  if (s === "critical" || s === "kritik") return BRAND.red;
  if (s === "warning" || s === "xəbərdarlıq") return BRAND.amber;
  return undefined;
}
