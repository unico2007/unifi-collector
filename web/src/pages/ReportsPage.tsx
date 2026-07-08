import { useState } from "react";
import { api } from "../lib/api";
import { toCSV, download, printReport, stamp } from "../lib/report";

function Icon({ d }: { d: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" className="w-5 h-5">
      <path d={d} />
    </svg>
  );
}

const download_icon = "M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3";

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

  const devicesCsv = () =>
    run("devices", async () => {
      const d = await api.devices();
      const csv = toCSV(
        ["Ad", "Vendor", "Tip", "Model", "IP", "MAC", "Status", "CPU%", "Yaddaş%", "Uptime"],
        d.map((x) => [x.name, x.vendor, x.type, x.model, x.ip, x.mac, x.state, x.cpu, x.memory, x.uptime]),
      );
      download(`cihazlar_${stamp()}.csv`, csv);
    });

  const clientsCsv = () =>
    run("clients", async () => {
      const c = await api.clients();
      const csv = toCSV(
        ["Ad", "MAC", "AP", "VLAN", "RSSI", "Rx", "Tx", "IP", "Müddət"],
        c.map((x) => [x.name, x.mac, x.ap, x.vlan, x.rssi, x.rx, x.tx, x.ip, x.since]),
      );
      download(`klientler_${stamp()}.csv`, csv);
    });

  const alertsCsv = () =>
    run("alerts", async () => {
      const a = await api.alerts();
      const csv = toCSV(
        ["Səviyyə", "Qayda", "Hədəf", "Mesaj", "Dəyər"],
        a.active.map((x) => [x.level, x.rule, x.target, x.message, x.value]),
      );
      download(`alertler_${stamp()}.csv`, csv);
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
    { id: "devices", title: "Cihazlar", desc: "Bütün UniFi/Kerio cihazları — status, IP, CPU, yaddaş.", action: devicesCsv },
    { id: "clients", title: "Klientlər", desc: "Qoşulu klientlər — AP, VLAN, RSSI, sürət.", action: clientsCsv },
    { id: "alerts", title: "Aktiv alertlər", desc: "Cari threshold pozuntuları.", action: alertsCsv },
  ];

  return (
    <div className="space-y-4">
      <div className="card p-5 flex flex-col sm:flex-row sm:items-center gap-4">
        <div className="flex-1">
          <div className="font-semibold">Tam şəbəkə hesabatı</div>
          <div className="text-sm text-muted mt-0.5">KPI-lər, cihazlar və alertlər — bir səhifəlik hesabat. Çap pəncərəsində "PDF kimi saxla" seçin.</div>
        </div>
        <button onClick={fullPdf} disabled={busy === "pdf"} className="btn btn-primary shrink-0">
          {busy === "pdf" ? "Hazırlanır..." : "PDF hesabat"}
        </button>
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        {cards.map((c) => (
          <div key={c.id} className="card p-5 flex flex-col">
            <div className="font-medium">{c.title}</div>
            <div className="text-sm text-muted mt-1 flex-1">{c.desc}</div>
            <button onClick={c.action} disabled={busy === c.id} className="btn mt-4 self-start">
              <Icon d={download_icon} />
              {busy === c.id ? "Yüklənir..." : "CSV yüklə"}
            </button>
          </div>
        ))}
      </div>

      <p className="text-xs text-muted">
        Hesabatlar birbaşa brauzerdə cari canlı datadan yaradılır. CSV faylları Excel/Google Sheets-də açıla bilər.
      </p>
    </div>
  );
}
