// Typed data layer. Every call hits the Go BFF at /api/*; if that isn't
// reachable yet (e.g. running the frontend standalone) it falls back to the
// mock data below, so the UI is fully browsable during development.

export type Vendor = "unifi" | "kerio";

export interface Device {
  name: string;
  vendor: Vendor;
  type: string;
  model: string;
  ip: string;
  mac: string;
  state: "online" | "offline";
  cpu: number;
  memory: number;
  uptime: string;
}

export interface Client {
  name: string;
  mac: string;
  ap: string;
  vlan: string;
  rssi: number; // dBm
  rx: string;
  tx: string;
  ip: string;
  since: string;
}

export interface LogRow {
  cols: string[];
}

export interface LogCategory {
  key: string;
  label: string;
  vendor: Vendor;
  count: number;
  columns: string[];
  rows: (string | { text: string; kind: "ok" | "no" | "warn" | "info" })[][];
}

export interface Overview {
  devices: { total: number; online: number; offline: number };
  clients: number;
  health: number;
  alerts: number;
  clientSeries: number[];
  vendorSplit: { vendor: Vendor; devices: number; clients: number }[];
  recentLogs: { time: string; level: "info" | "warn" | "error"; msg: string }[];
}

async function get<T>(path: string, mock: T): Promise<T> {
  try {
    const r = await fetch(`/api${path}`, { credentials: "include" });
    if (!r.ok) throw new Error(String(r.status));
    return (await r.json()) as T;
  } catch {
    return mock;
  }
}

export interface Traffic {
  rx: number[];
  tx: number[];
  totalRx: string;
  totalTx: string;
  topTalkers: { label: string; value: number; sub?: string }[];
  perAp: { label: string; value: number }[];
}

export interface Wifi {
  rssiBins: number[];
  rssiLabels: string[];
  clientsPerAp: { label: string; value: number }[];
  bandSplit: { label: string; value: number }[];
  vlanSplit: { label: string; value: number }[];
  quality: { good: number; fair: number; poor: number };
}

export interface Firewall {
  allow: number[];
  deny: number[];
  blockedToday: number;
  topBlockedIps: { label: string; value: number; sub?: string }[];
  topRules: { label: string; value: number }[];
  webCategories: { label: string; value: number }[];
  attacks: { time: string; type: string; source: string; action: string }[];
}

export interface DeviceDetail {
  device: Device;
  cpu: number[];
  memory: number[];
  rx: number[];
  tx: number[];
  clients: { name: string; mac: string; rssi: number; rx: string; tx: string }[];
}

export interface AiChat {
  answer: string;
  source: "prometheus" | "loki" | "";
  query: string;
  result?: string;
}

export interface Alert {
  level: "critical" | "warning";
  rule: string;
  target: string;
  message: string;
  value: string;
}

export interface AlertsData {
  active: Alert[];
  counts: { critical: number; warning: number };
  rules: { name: string; condition: string; level: string }[];
}

export interface TopoNode {
  name: string;
  type: string;
  vendor: string;
  model: string;
  ip: string;
  state: string;
  clients: number;
}
export interface TopoClient {
  name: string;
  mac: string;
  rssi: number;
}
export interface Topology {
  edge: TopoNode[];
  switches: TopoNode[];
  aps: TopoNode[];
  clientsByAp: Record<string, TopoClient[]>;
  stats: { switches: number; aps: number; clients: number };
}

export interface AiInsight {
  level: "info" | "warn" | "error";
  title: string;
  detail: string;
}

export interface AiInsights {
  insights: AiInsight[];
  summary: string;
  generated_at: number;
}

export const api = {
  overview: () => get<Overview>("/overview", mockOverview),
  devices: () => get<Device[]>("/devices", mockDevices),
  clients: () => get<Client[]>("/clients", mockClients),
  logCategories: () => get<LogCategory[]>("/logs/categories", mockCategories),
  traffic: () => get<Traffic>("/traffic", mockTraffic),
  wifi: () => get<Wifi>("/wifi", mockWifi),
  firewall: () => get<Firewall>("/firewall", mockFirewall),
  alerts: () => get<AlertsData>("/alerts", mockAlerts),
  topology: () => get<Topology>("/topology", mockTopology),
  device: (name: string) => get<DeviceDetail>(`/devices/${encodeURIComponent(name)}`, mockDeviceDetail(name)),
  login: async (username: string, password: string, role: string) => {
    const r = await fetch("/api/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify({ username, password, role }),
    });
    if (!r.ok) throw new Error("auth");
    return (await r.json()) as { username: string; role: "admin" | "guest" };
  },
  aiChat: async (question: string): Promise<AiChat> => {
    try {
      const r = await fetch("/api/ai/chat", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ question }),
      });
      if (!r.ok) throw new Error(String(r.status));
      return (await r.json()) as AiChat;
    } catch {
      // Offline/dev fallback so the page stays browsable without the AI service.
      return mockAiChat(question);
    }
  },
  aiInsights: async (): Promise<AiInsights> => {
    try {
      const r = await fetch("/api/ai/insights", { credentials: "include" });
      if (!r.ok) throw new Error(String(r.status));
      return (await r.json()) as AiInsights;
    } catch {
      return {
        insights: [
          { level: "info", title: "AI Insights", detail: "AI servisi qoşulanda anomaliya və proqnozlar burada avtomatik görünəcək (demo)." },
        ],
        summary: "",
        generated_at: 0,
      };
    }
  },
  aiSummary: async (): Promise<string> => {
    try {
      const r = await fetch("/api/ai/summary", { credentials: "include" });
      if (!r.ok) throw new Error(String(r.status));
      return ((await r.json()) as { summary: string }).summary;
    } catch {
      return "Son 24 saatda 2 error, 3 xəbərdarlıq qeydə alınıb. Ən çox WAN gecikməsi ilə bağlıdır (demo).";
    }
  },
};

// Canned answers so the AI page is demonstrable without the live service.
function mockAiChat(question: string): AiChat {
  const q = question.toLowerCase();
  if (q.includes("cihaz") || q.includes("device"))
    return { answer: "Hazırda 25 cihaz var — 23 online, 2 offline (demo cavab).", source: "prometheus", query: 'unifi_devices_total', result: "unifi_devices_total = 25" };
  if (q.includes("offline") || q.includes("problem") || q.includes("error"))
    return { answer: "1 UniFi AP offline görünür: AP-Zirzəmi. Elektrik/uplink yoxlanmalıdır (demo cavab).", source: "prometheus", query: 'unifi_device_up == 0', result: 'unifi_device_up{name="AP-Zirzəmi"} = 0' };
  return { answer: "AI servisi qoşulanda buradan real cavab alacaqsınız. (demo rejimi)", source: "", query: "" };
}

// --- mock data (mirrors the BFF response shapes) ---------------------------

const mockOverview: Overview = {
  devices: { total: 25, online: 23, offline: 2 },
  clients: 154,
  health: 98,
  alerts: 2,
  clientSeries: [120, 128, 131, 140, 138, 145, 150, 149, 152, 154, 151, 154],
  vendorSplit: [
    { vendor: "unifi", devices: 22, clients: 154 },
    { vendor: "kerio", devices: 3, clients: 0 },
  ],
  recentLogs: [
    { time: "12:04:30", level: "info", msg: "Client 9c:7b connected on AP-Ofis-1" },
    { time: "12:04:12", level: "warn", msg: "High channel utilization on AP-Zirzəmi" },
    { time: "12:02:33", level: "error", msg: "WAN latency spike 240ms" },
    { time: "12:01:05", level: "info", msg: "Roaming: client moved to AP-Ofis-2" },
  ],
};

const mockDevices: Device[] = [
  { name: "AP-Ofis-1", vendor: "unifi", type: "uap", model: "U6-Pro", ip: "10.10.0.11", mac: "9c:7b:ef:32:3e:8a", state: "online", cpu: 12, memory: 41, uptime: "12g 4s" },
  { name: "AP-Ofis-2", vendor: "unifi", type: "uap", model: "U6-Lite", ip: "10.10.0.12", mac: "9c:7b:ef:aa:11:02", state: "online", cpu: 8, memory: 38, uptime: "12g 4s" },
  { name: "AP-Zirzəmi", vendor: "unifi", type: "uap", model: "U6-Lite", ip: "10.10.0.13", mac: "9c:7b:ef:bb:22:03", state: "offline", cpu: 0, memory: 0, uptime: "-" },
  { name: "internet", vendor: "kerio", type: "interface", model: "-", ip: "89.147.252.244", mac: "9c:7b:ef:32:3e:8a", state: "online", cpu: 0, memory: 0, uptime: "-" },
  { name: "Lan", vendor: "kerio", type: "interface", model: "-", ip: "10.10.0.1", mac: "00:23:54:61:d5:59", state: "online", cpu: 0, memory: 0, uptime: "-" },
  { name: "VpnServer", vendor: "kerio", type: "interface", model: "-", ip: "-", mac: "-", state: "online", cpu: 0, memory: 0, uptime: "-" },
];

const mockClients: Client[] = [
  { name: "a.mammadov-mbp", mac: "3c:22:fb:aa:01", ap: "AP-Ofis-1", vlan: "10", rssi: -48, rx: "22 Mbps", tx: "8 Mbps", ip: "10.10.0.55", since: "2s 14m" },
  { name: "r.aliyeva-iphone", mac: "a4:83:e7:bb:02", ap: "AP-Ofis-2", vlan: "10", rssi: -66, rx: "12 Mbps", tx: "3 Mbps", ip: "10.10.0.61", since: "44m" },
  { name: "k.huseynov-pc", mac: "d8:cb:8a:cc:03", ap: "AP-Ofis-1", vlan: "20", rssi: -72, rx: "6 Mbps", tx: "1 Mbps", ip: "10.10.0.77", since: "3s 2m" },
  { name: "Guest-2f1a", mac: "b8:27:eb:dd:04", ap: "AP-Ofis-2", vlan: "90", rssi: -58, rx: "5 Mbps", tx: "2 Mbps", ip: "10.10.90.4", since: "18m" },
];

function pill(text: string, kind: "ok" | "no" | "warn" | "info") {
  return { text, kind };
}

const mockCategories: LogCategory[] = [
  {
    key: "filter", label: "Filtr / Trafik", vendor: "kerio", count: 8912,
    columns: ["Vaxt", "Əməl", "Mənbə IP:Port", "Təyinat IP:Port", "Protokol", "Qayda"],
    rows: [
      ["12:04:51", pill("Deny", "no"), "89.20.11.4:51044", "10.10.0.3:22", "TCP", "Block-SSH-WAN"],
      ["12:04:50", pill("Allow", "ok"), "10.10.0.55:5123", "142.250.185.4:443", "TCP", "LAN-to-Internet"],
      ["12:04:45", pill("Deny", "no"), "45.148.10.2:44210", "89.147.252.244:3389", "TCP", "Block-RDP"],
      ["12:04:44", pill("Allow", "ok"), "10.10.0.12:5001", "8.8.8.8:53", "UDP", "DNS-Allow"],
      ["12:04:40", pill("Deny", "no"), "193.32.162.9:40122", "89.147.252.244:23", "TCP", "Block-Telnet"],
    ],
  },
  {
    key: "security", label: "Təhlükəsizlik", vendor: "kerio", count: 1240,
    columns: ["Vaxt", "Səviyyə", "Əməl", "Mənbə IP", "Qayda", "Mesaj"],
    rows: [
      ["12:03:10", pill("error", "no"), pill("Deny", "no"), "45.148.10.2", "IPS", "Port scan aşkarlandı"],
      ["12:01:44", pill("warn", "warn"), pill("Deny", "no"), "185.34.9.7", "Anti-spoof", "Saxta mənbə ünvanı"],
    ],
  },
  {
    key: "web", label: "Veb (HTTP)", vendor: "kerio", count: 2231,
    columns: ["Vaxt", "İstifadəçi", "URL", "Kateqoriya", "Əməl"],
    rows: [
      ["12:03:11", "a.mammadov", "youtube.com", "Media", pill("Allow", "ok")],
      ["12:03:02", "r.aliyeva", "facebook.com", "Social", pill("Deny", "no")],
      ["12:02:40", "a.mammadov", "bet365.com", "Gambling", pill("Deny", "no")],
      ["12:02:31", "s.qasimova", "office.com", "Business", pill("Allow", "ok")],
    ],
  },
  {
    key: "conn", label: "Bağlantılar", vendor: "kerio", count: 560,
    columns: ["Vaxt", "Mənbə", "Təyinat", "Protokol", "Müddət", "Baytlar"],
    rows: [
      ["12:04:10", "10.10.0.55", "142.250.185.4", "TCP/443", "2m 14s", "1.2 MB"],
      ["12:03:40", "10.10.0.61", "89.147.252.244", "TCP/443", "12m 03s", "8.9 MB"],
    ],
  },
  {
    key: "vpn", label: "SSL VPN", vendor: "kerio", count: 14,
    columns: ["Vaxt", "İstifadəçi", "IP", "Status", "Müddət"],
    rows: [
      ["11:30:02", "r.aliyeva", "10.8.0.4", pill("Aktiv", "ok"), "1s 12m"],
      ["10:05:41", "k.huseynov", "10.8.0.5", pill("Bağlandı", "info"), "2s 40m"],
    ],
  },
  { key: "kerr", label: "Xətalar", vendor: "kerio", count: 12, columns: gen(), rows: genRows("kerio") },
  { key: "kwarn", label: "Xəbərdarlıqlar", vendor: "kerio", count: 34, columns: gen(), rows: genRows("kerio") },
  {
    key: "config", label: "Konfiqurasiya", vendor: "kerio", count: 6,
    columns: ["Vaxt", "Admin", "Əməliyyat", "IP", "Detal"],
    rows: [
      ["11:52:05", "helpdesk_unico", "Qayda dəyişdi", "10.10.0.21", "Block-RDP əlavə"],
      ["10:22:44", "log", "Giriş", "10.10.1.229", "Read-only"],
    ],
  },
  { key: "alert", label: "Alert", vendor: "kerio", count: 2, columns: gen(), rows: genRows("kerio") },
  { key: "debug", label: "Debug", vendor: "kerio", count: 120, columns: gen(), rows: genRows("kerio") },

  { key: "system", label: "Sistem", vendor: "unifi", count: 42, columns: gen(), rows: genRows("unifi") },
  { key: "updates", label: "Yeniləmələr", vendor: "unifi", count: 3, columns: gen(), rows: genRows("unifi") },
  {
    key: "admins", label: "Adminlər", vendor: "unifi", count: 12,
    columns: ["Vaxt", "Admin", "Əməliyyat", "IP", "Detal"],
    rows: [
      ["11:58:20", "admin", "Giriş", "10.10.0.20", "Uğurlu"],
      ["11:40:11", "admin", "Konfiq saxlandı", "10.10.0.20", "WiFi"],
    ],
  },
  { key: "backups", label: "Ehtiyat nüsxə", vendor: "unifi", count: 1, columns: gen(), rows: genRows("unifi") },
  { key: "users", label: "İstifadəçilər", vendor: "unifi", count: 28, columns: gen(), rows: genRows("unifi") },
  {
    key: "clientev", label: "Klient hadisələri", vendor: "unifi", count: 310,
    columns: ["Vaxt", "Səviyyə", "Klient", "AP", "Hadisə"],
    rows: [
      ["12:04:30", pill("info", "info"), "9c:7b:ef", "AP-Ofis-1", "Qoşuldu"],
      ["12:01:05", pill("info", "info"), "a4:83:e7", "AP-Ofis-2", "Roaming"],
      ["11:58:22", pill("warn", "warn"), "d8:cb:8a", "AP-Ofis-1", "Zəif siqnal (-78 dBm)"],
    ],
  },
];

function gen() {
  return ["Vaxt", "Səviyyə", "Mənbə", "Mesaj"];
}
function genRows(v: Vendor) {
  return [
    ["12:04:30", pill("info", "info"), v === "unifi" ? "controller" : "kerio-ctrl", "Servis normal işləyir"],
    ["12:02:33", pill("error", "no"), "gateway", "WAN gecikməsi 240ms"],
    ["12:00:11", pill("warn", "warn"), "system", "Yaddaş istifadəsi 82%"],
  ];
}

function wave(n: number, base: number, amp: number, seed = 1) {
  return Array.from({ length: n }, (_, i) =>
    Math.max(0, Math.round(base + amp * Math.sin(i / 2 + seed) + amp * 0.4 * Math.sin(i / 5 + seed * 2))),
  );
}

const mockTraffic: Traffic = {
  rx: wave(24, 340, 120, 1),
  tx: wave(24, 120, 60, 3),
  totalRx: "1.9 TB",
  totalTx: "612 GB",
  topTalkers: [
    { label: "k.huseynov-pc", value: 128, sub: "GB" },
    { label: "AP-Ofis-1", value: 96, sub: "GB" },
    { label: "a.mammadov-mbp", value: 74, sub: "GB" },
    { label: "Guest-2f1a", value: 41, sub: "GB" },
    { label: "r.aliyeva-iphone", value: 28, sub: "GB" },
  ],
  perAp: [
    { label: "AP-Ofis-1", value: 620 },
    { label: "AP-Ofis-2", value: 410 },
    { label: "AP-Zirzəmi", value: 0 },
  ],
};

const mockWifi: Wifi = {
  rssiBins: [4, 9, 22, 41, 38, 24, 11, 5],
  rssiLabels: ["-90", "-80", "-75", "-70", "-65", "-60", "-55", "-45"],
  clientsPerAp: [
    { label: "AP-Ofis-1", value: 82 },
    { label: "AP-Ofis-2", value: 61 },
    { label: "AP-Zirzəmi", value: 11 },
  ],
  bandSplit: [
    { label: "5 GHz", value: 108 },
    { label: "2.4 GHz", value: 46 },
  ],
  vlanSplit: [
    { label: "VLAN 10 (Ofis)", value: 96 },
    { label: "VLAN 20 (IT)", value: 34 },
    { label: "VLAN 90 (Qonaq)", value: 24 },
  ],
  quality: { good: 118, fair: 27, poor: 9 },
};

const mockFirewall: Firewall = {
  allow: wave(24, 820, 260, 2),
  deny: wave(24, 140, 90, 5),
  blockedToday: 3412,
  topBlockedIps: [
    { label: "45.148.10.2", value: 842, sub: "RU" },
    { label: "193.32.162.9", value: 511, sub: "NL" },
    { label: "185.34.9.7", value: 388, sub: "DE" },
    { label: "89.20.11.4", value: 240, sub: "RU" },
  ],
  topRules: [
    { label: "Block-RDP", value: 1204 },
    { label: "Block-SSH-WAN", value: 902 },
    { label: "Drop-Ping-WAN", value: 640 },
    { label: "Block-Telnet", value: 311 },
  ],
  webCategories: [
    { label: "Social", value: 420 },
    { label: "Media", value: 388 },
    { label: "Gambling", value: 96 },
    { label: "Malware", value: 22 },
  ],
  attacks: [
    { time: "12:03:10", type: "Port scan", source: "45.148.10.2", action: "Deny" },
    { time: "11:51:40", type: "Brute force (SSH)", source: "193.32.162.9", action: "Deny" },
    { time: "11:22:05", type: "Spoofed source", source: "185.34.9.7", action: "Deny" },
  ],
};

const mockAlerts: AlertsData = {
  active: [
    { level: "critical", rule: "Cihaz offline", target: "AP-Zirzəmi", message: "AP-Zirzəmi (uap) offline-dır", value: "offline" },
    { level: "warning", rule: "CPU yüksək", target: "AP-Ofis-1", message: "AP-Ofis-1: CPU 88%", value: "88%" },
  ],
  counts: { critical: 1, warning: 1 },
  rules: [
    { name: "Cihaz offline", condition: "unifi_device_up == 0", level: "critical" },
    { name: "CPU yüksək", condition: "unifi_device_cpu_percent > 85", level: "warning" },
    { name: "Yaddaş yüksək", condition: "unifi_device_memory_percent > 90", level: "warning" },
    { name: "Subsystem problemi", condition: "unifi_health_status < 1", level: "warning" },
  ],
};

const mockTopology: Topology = {
  edge: [
    { name: "Kerio-Firewall", type: "interface", vendor: "kerio", model: "-", ip: "89.147.252.244", state: "online", clients: 0 },
    { name: "USG-Gateway", type: "ugw", vendor: "unifi", model: "USG-Pro", ip: "10.10.0.1", state: "online", clients: 0 },
  ],
  switches: [
    { name: "USW-Core", type: "usw", vendor: "unifi", model: "US-24", ip: "10.10.0.2", state: "online", clients: 0 },
  ],
  aps: [
    { name: "AP-Ofis-1", type: "uap", vendor: "unifi", model: "U6-Pro", ip: "10.10.0.11", state: "online", clients: 8 },
    { name: "AP-Ofis-2", type: "uap", vendor: "unifi", model: "U6-Lite", ip: "10.10.0.12", state: "online", clients: 6 },
    { name: "AP-Zirzəmi", type: "uap", vendor: "unifi", model: "U6-Lite", ip: "10.10.0.13", state: "offline", clients: 0 },
  ],
  clientsByAp: {
    "AP-Ofis-1": [
      { name: "a.mammadov-mbp", mac: "3c:22:fb:aa:01", rssi: -48 },
      { name: "k.huseynov-pc", mac: "d8:cb:8a:cc:03", rssi: -72 },
    ],
    "AP-Ofis-2": [{ name: "r.aliyeva-iphone", mac: "a4:83:e7:bb:02", rssi: -66 }],
  },
  stats: { switches: 1, aps: 3, clients: 14 },
};

function mockDeviceDetail(name: string): DeviceDetail {
  const d = mockDevices.find((x) => x.name === name) ?? mockDevices[0];
  return {
    device: d,
    cpu: wave(24, d.cpu || 6, 8, 1),
    memory: wave(24, d.memory || 30, 10, 2),
    rx: wave(24, 40, 25, 3),
    tx: wave(24, 14, 10, 4),
    clients: [
      { name: "a.mammadov-mbp", mac: "3c:22:fb:aa:01", rssi: -48, rx: "22 Mbps", tx: "8 Mbps" },
      { name: "k.huseynov-pc", mac: "d8:cb:8a:cc:03", rssi: -72, rx: "6 Mbps", tx: "1 Mbps" },
    ],
  };
}
