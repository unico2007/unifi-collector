# Troubleshooting runbook-ları (UniFi + Kerio)

Bu sənəd Unico şəbəkəsində tez-tez rast gəlinən problemlərin diaqnostika və həll
addımlarını əhatə edir. Cavablar bu addımlara əsaslanmalıdır.

## AP (access point) offline oldu

Simptom: `unifi_device_up == 0`, Alertlər səhifəsində "Cihaz offline" (kritik).

Yoxlama addımları:
1. Elektrik / PoE — switch portunun PoE verdiyini yoxla; kabeli/portu dəyiş.
2. Uplink — AP-nin qoşulduğu switch portu link statusunu itiribmi (`unifi_device_up`
   həmin switch üçün 1-dirmi).
3. Fiziki — AP-də işıq yanırmı; lazım olsa yerində reboot.
4. Uptime — əgər cihaz təzəcə qayıdıbsa (`unifi_device_uptime_seconds` kiçikdir),
   son reboot səbəbini loglardan yoxla.
5. Bu saytda gateway Kerio-dur; UniFi USG yoxdur, ona görə wan/lan/vpn/www
   subsystem-ləri normalda 0 cihaz göstərir — bu offline demək deyil.

## Cihazda CPU yüksəkdir

Simptom: `unifi_device_cpu_percent` həddi (default 85%) aşıb; Alertlər "CPU yüksək".

Yoxlama addımları:
1. Hansı cihaz — Cihazlar səhifəsində CPU sütununu sırala.
2. Yük — həmin AP-yə çox klient qoşulub? `unifi_client_rssi` sayını AP üzrə yoxla.
3. Firmware — köhnə firmware CPU sızması yarada bilər; yeniləməni nəzərdən keçir.
4. Davamlıdırmı — `avg_over_time(unifi_device_cpu_percent[15m])` yüksəkdirsə real
   problemdir; anlıq sıçrayış adətən keçicidir.

## Cihazda yaddaş yüksəkdir

Simptom: `unifi_device_memory_percent` həddi (default 90%) aşıb.

Yoxlama addımları:
1. Trend — `predict_linear` ilə yaddaşın saturasiyaya nə vaxt çatacağını qiymətləndir.
2. Davamlı yüksək — `avg_over_time(unifi_device_memory_percent[15m]) > 88` isə
   planlı reboot həll edir (yaddaş sızması).
3. Model — kiçik yaddaşlı köhnə modellər (U6-Lite) daha tez dolur.

## Klientlərdə zəif siqnal (weak signal)

Simptom: çox klientdə `unifi_client_rssi < -75` dBm; WiFi keyfiyyəti "zəif".

Yoxlama addımları:
1. Əhatə — həmin AP-nin əhatə etdiyi zona böyükdür, ya da divar/maneə var.
2. Zolaq — 2.4 GHz daha uzaq gedir amma yavaşdır; 5 GHz yaxın amma sürətli.
3. AP sıxlığı — zəif zonaya əlavə AP lazım ola bilər.
4. Norma — həmişə ~5-10% klient zəif olur; yalnız pay 30%-i keçəndə problem sayılır.

## WAN gecikməsi / internet problemi

Simptom: loglarda "WAN latency" və ya yüksək gecikmə.

Yoxlama addımları:
1. Gateway Kerio-dur (`https://10.10.0.1:4081`) — WAN interfeysinin link statusunu
   `unifi_device_up{vendor="kerio"}` ilə yoxla.
2. Kerio loglarında (Firewall səhifəsi) uplink/ISP xətaları var?
3. Kerio yalnız read-only — dəyişiklik Kerio admin tərəfindən edilməlidir.

## Firewall çox blok edir / hücum şübhəsi

Simptom: Firewall səhifəsində "Bu gün bloklanan" yüksəkdir, top attacker IP-lər var.

Yoxlama addımları:
1. Top IP-lər — Firewall səhifəsində ən çox bloklanan mənbə IP-ləri (adətən public,
   xarici) yoxla; təkrarlanan IP scan/brute-force ola bilər.
2. Qaydalar — hansı Kerio qaydası işləyir (Block-RDP, Block-SSH-WAN və s.).
3. IPS hadisələri — port scan, brute force, spoofed source cədvəlində.
4. Bunlar adətən normal internet "background noise"-dur; artım kəskin olsa Kerio
   admininə bildir.

## Subsystem sağlamlığı (health) xəbərdarlığı

Simptom: `unifi_health_status < 1`.

Qeyd: bu saytda gateway Kerio olduğu üçün UniFi-nin wan/lan/vpn/www subsystem-ləri
0 idarə olunan cihaz göstərir — collector bunları filtrləyir, yalnız `wlan` qalır.
Yəni wlan xaricində subsystem alertləri gözlənilmir; görünərsə collector loglarını
yoxla.
