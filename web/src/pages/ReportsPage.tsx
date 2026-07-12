import { useState } from "react";
import { api } from "../lib/api";
import { printReport, stamp } from "../lib/report";
import { downloadXlsx, statusColor } from "../lib/xlsx";

function Icon({ d }: { d: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" className="w-5 h-5">
      <path d={d} />
    </svg>
  );
}

const download_icon = "M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3";

const levelAz = (l: string) => (l === "critical" ? "Kritik" : "Xəbərdarlıq");

export default function ReportsPage() {
  const [busy, setBusy] = useState<string | null>(null);

  async function run(id: string, fn: () => Promise<void>) {
    setBusy(id);
    try {
      await fn();
    } finally {
      setBusy(null);
    }
  }

  const devicesXlsx = () =>
    run("devices", async () => {
      const d = await api.devices();
      await downloadXlsx(`unico_cihazlar_${stamp()}.xlsx`, {
        title: "Cihaz hesabatı",
        subtitle: `${d.length} cihaz — UniFi + Kerio inventarı`,
        sheets: [
          {
            name: "Cihazlar",
            columns: [
              { header: "Ad", width: 24 },
              { header: "Vendor", width: 10 },
              { header: "Tip", width: 10 },
              { header: "Model", width: 16 },
              { header: "IP", width: 15 },
              { header: "MAC", width: 19 },
              { header: "Status", width: 10, align: "center" },
              { header: "CPU %", width: 9, align: "right" },
              { header: "Yaddaş %", width: 10, align: "right" },
              { header: "İşləmə müddəti", width: 16, align: "right" },
            ],
            rows: d.map((x) => [x.name, x.vendor, x.type, x.model, x.ip, x.mac, x.state, x.cpu, x.memory, x.uptime]),
            cellColor: (ci, v) => (ci === 6 ? statusColor(v) : undefined),
          },
        ],
      });
    });

  const clientsXlsx = () =>
    run("clients", async () => {
      const c = await api.clients();
      await downloadXlsx(`unico_klientler_${stamp()}.xlsx`, {
        title: "Klient hesabatı",
        subtitle: `${c.length} qoşulu klient`,
        sheets: [
          {
            name: "Klientlər",
            columns: [
              { header: "Ad", width: 24 },
              { header: "MAC", width: 19 },
              { header: "IP", width: 15 },
              { header: "AP", width: 20 },
              { header: "VLAN", width: 8, align: "center" },
              { header: "RSSI (dBm)", width: 11, align: "right" },
              { header: "Rx", width: 12, align: "right" },
              { header: "Tx", width: 12, align: "right" },
              { header: "Data (sessiya)", width: 14, align: "right" },
              { header: "Müddət", width: 12, align: "right" },
            ],
            rows: c.map((x) => [x.name, x.mac, x.ip, x.ap, x.vlan, x.rssi, x.rx, x.tx, x.data, x.since]),
          },
        ],
      });
    });

  const alertsXlsx = () =>
    run("alerts", async () => {
      const a = await api.alerts();
      await downloadXlsx(`unico_alertler_${stamp()}.xlsx`, {
        title: "Alert hesabatı",
        subtitle: `${a.active.length} aktiv alert — kritik: ${a.counts.critical}, xəbərdarlıq: ${a.counts.warning}`,
        sheets: [
          {
            name: "Aktiv alertlər",
            columns: [
              { header: "Səviyyə", width: 13, align: "center" },
              { header: "Qayda", width: 20 },
              { header: "Hədəf", width: 20 },
              { header: "Mesaj", width: 44 },
              { header: "Dəyər", width: 11, align: "right" },
            ],
            rows: a.active.length
              ? a.active.map((x) => [levelAz(x.level), x.rule, x.target, x.message, x.value])
              : [["—", "Aktiv alert yoxdur", "", "", ""]],
            cellColor: (ci, v) => (ci === 0 ? statusColor(v) : undefined),
          },
        ],
      });
    });

  const fullXlsx = () =>
    run("full", async () => {
      const [o, d, c, a] = await Promise.all([api.overview(), api.devices(), api.clients(), api.alerts()]);
      await downloadXlsx(`unico_hesabat_${stamp()}.xlsx`, {
        title: "Tam şəbəkə hesabatı",
        subtitle: `${o.devices.online}/${o.devices.total} cihaz online · ${o.clients} klient · sağlamlıq ${o.health}%`,
        sheets: [
          {
            name: "Cihazlar",
            columns: [
              { header: "Ad", width: 24 },
              { header: "Tip", width: 10 },
              { header: "Model", width: 16 },
              { header: "IP", width: 15 },
              { header: "Status", width: 10, align: "center" },
              { header: "CPU %", width: 9, align: "right" },
              { header: "Yaddaş %", width: 10, align: "right" },
            ],
            rows: d.map((x) => [x.name, x.type, x.model, x.ip, x.state, x.cpu, x.memory]),
            cellColor: (ci, v) => (ci === 4 ? statusColor(v) : undefined),
          },
          {
            name: "Klientlər",
            columns: [
              { header: "Ad", width: 24 },
              { header: "IP", width: 15 },
              { header: "AP", width: 20 },
              { header: "RSSI (dBm)", width: 11, align: "right" },
              { header: "Data (sessiya)", width: 14, align: "right" },
            ],
            rows: c.map((x) => [x.name, x.ip, x.ap, x.rssi, x.data]),
          },
          {
            name: "Aktiv alertlər",
            columns: [
              { header: "Səviyyə", width: 13, align: "center" },
              { header: "Qayda", width: 20 },
              { header: "Hədəf", width: 20 },
              { header: "Mesaj", width: 44 },
            ],
            rows: a.active.length
              ? a.active.map((x) => [levelAz(x.level), x.rule, x.target, x.message])
              : [["—", "Aktiv alert yoxdur", "", ""]],
            cellColor: (ci, v) => (ci === 0 ? statusColor(v) : undefined),
          },
        ],
      });
    });

  const fullPdf = () =>
    run("pdf", async () => {
      const [o, d, a] = await Promise.all([api.overview(), api.devices(), api.alerts()]);
      printReport(
        "Şəbəkə Hesabatı",
        [
          { label: "Cihazlar", value: `${o.devices.online}/${o.devices.total} online` },
          { label: "Klientlər", value: String(o.clients) },
          { label: "Sağlamlıq", value: `${o.health}%` },
          { label: "Aktiv alertlər", value: String(a.active.length) },
        ],
        [
          {
            title: "Cihazlar",
            headers: ["Ad", "Tip", "Model", "IP", "Status", "CPU%", "Yaddaş%"],
            rows: d.map((x) => [x.name, x.type, x.model, x.ip, x.state, x.cpu, x.memory]),
          },
          {
            title: "Aktiv alertlər",
            headers: ["Səviyyə", "Qayda", "Hədəf", "Mesaj"],
            rows: a.active.length ? a.active.map((x) => [x.level, x.rule, x.target, x.message]) : [["—", "Aktiv alert yoxdur", "", ""]],
          },
        ],
      );
    });

  const cards = [
    { id: "devices", title: "Cihazlar", desc: "Bütün UniFi/Kerio cihazları — status, IP, CPU, yaddaş.", action: devicesXlsx },
    { id: "clients", title: "Klientlər", desc: "Qoşulu klientlər — AP, VLAN, RSSI, sürət, data.", action: clientsXlsx },
    { id: "alerts", title: "Aktiv alertlər", desc: "Cari threshold pozuntuları.", action: alertsXlsx },
  ];

  return (
    <div className="space-y-4">
      <div className="card p-5 flex flex-col sm:flex-row sm:items-center gap-4">
        <div className="flex-1">
          <div className="font-semibold">Tam şəbəkə hesabatı</div>
          <div className="text-sm text-muted mt-0.5">
            Cihazlar, klientlər və alertlər — Unico dizaynlı, çoxvərəqli Excel faylı; PDF isə çap pəncərəsindən.
          </div>
        </div>
        <div className="flex gap-2 shrink-0">
          <button onClick={fullXlsx} disabled={busy === "full"} className="btn btn-primary">
            <Icon d={download_icon} />
            {busy === "full" ? "Hazırlanır..." : "Excel hesabat"}
          </button>
          <button onClick={fullPdf} disabled={busy === "pdf"} className="btn">
            {busy === "pdf" ? "Hazırlanır..." : "PDF"}
          </button>
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        {cards.map((c) => (
          <div key={c.id} className="card p-5 flex flex-col">
            <div className="font-medium">{c.title}</div>
            <div className="text-sm text-muted mt-1 flex-1">{c.desc}</div>
            <button onClick={c.action} disabled={busy === c.id} className="btn mt-4 self-start">
              <Icon d={download_icon} />
              {busy === c.id ? "Yüklənir..." : "Excel yüklə"}
            </button>
          </div>
        ))}
      </div>

      <p className="text-xs text-muted">
        Hesabatlar birbaşa brauzerdə cari canlı datadan yaradılır — brend başlıqlı, filtrli və donmuş sərlövhəli .xlsx faylları Excel/Google Sheets/Numbers-də açılır.
      </p>
    </div>
  );
}
