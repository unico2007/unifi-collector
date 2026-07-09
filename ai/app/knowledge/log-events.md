# Log hadisə kataloqu (UniFi CEF + Kerio)

Loglarda rast gəlinən əsas hadisə tiplərinin mənası və tövsiyə olunan tədbir. Xam
loglar embed edilmir — bu kataloq distillə edilmiş biliкdir; konkret/cari loglar
lazım olanda Loki-dən canlı sorğulanır.

## UniFi hadisələri (CEF)

UniFi log sətri CEF formatındadır: `CEF:0|Ubiquiti|UniFi OS|ver|sig|<EventName>|...`.
Əsas hadisə adları:

- **EVT_AP_Connected / EVT_AP_Adopted** — AP controller-ə qoşuldu/adopt olundu. Normal.
- **EVT_AP_Lost_Contact / EVT_AP_Disconnected** — AP əlaqəni itirdi → offline runbook-a
  bax (elektrik/uplink).
- **EVT_AP_RestartedUnknown / reboot** — AP yenidən başladı; uptime kiçikdirsə səbəbi
  araşdır (elektrik kəsintisi, firmware).
- **EVT_WU_Connected / EVT_WU_Disconnected** — klient qoşuldu/ayrıldı. Normal fon.
- **EVT_WU_Roam / EVT_WU_RoamRadio** — klient AP-lər arası keçdi (roaming). Normal.
- **EVT_AP_ChannelChanged** — kanal dəyişdi (avtomatik RF optimizasiya). Adətən normal.
- **admin accessed / EVT_AD_Login** — admin panelə giriş. Təhlükəsizlik auditi üçün izlə.

## Kerio hadisələri

Kerio filter sətirləri: `DENY`/`ALLOW` + qayda adı + mənbə→təyinat. Kateqoriyalar:

- **DENY (block)** — qayda trafiki blokladı. Top bloklanan public IP-lər adətən
  scan/brute-force fonudur.
- **Block-RDP / Block-SSH-WAN / Block-Telnet** — WAN-dan idarəetmə portlarına
  cəhdlər bloklandı. Normal müdafiə.
- **Suspected P2P / Peer to Peer traffic** — P2P trafik aşkarlandı/bloklandı.
- **Anti-spoof / Spoofed source** — saxta mənbə ünvanı bloklandı. Şübhəli, izlə.
- **IPS / Port scan** — hücum aşkarlama sistemi hadisəsi. Təkrarlanırsa Kerio
  admininə bildir.
- **ALLOW / permit** — icazə verilən trafik. Fon.

## Ümumi qayda

Tək-tük DENY/scan hadisələri normal internet fonudur. Yalnız kəskin artım, eyni
IP-dən davamlı cəhdlər, ya da anti-spoof/IPS təkrarı diqqət tələb edir. Kerio
read-only olduğu üçün qayda dəyişikliyi Kerio admin tərəfindən edilir.
