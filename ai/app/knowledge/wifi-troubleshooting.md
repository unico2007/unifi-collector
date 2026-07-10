# WiFi və klient problemləri (runbook)

Bu sənəd klient tərəfli WiFi problemlərinin diaqnostika və həll addımlarını əhatə
edir. Cavablar bu addımlara əsaslanmalıdır. Əlaqəli metriklər: `unifi_client_rssi`
(dBm, mənfi), `unifi_client_rx_rate`/`unifi_client_tx_rate` (bits/s), band
(2.4 GHz / 5 GHz / 6 GHz), `unifi_client_rx_bytes`/`unifi_client_tx_bytes`.

Siqnal keyfiyyəti həddləri (dBm): **-60-dan yaxşı = yaxşı**, **-60..-72 = orta**,
**-72-dən aşağı = zəif**.

## Zəif WiFi siqnalı

Simptom: klientin `unifi_client_rssi` dəyəri -72 dBm-dən aşağıdır; Klientlər
səhifəsində "Siqnal" qırmızıdır. AI Insights "Zəif WiFi siqnalı" verə bilər (WiFi
klientlərin ≥30%-i -75 dBm-dən aşağı olanda).

Yoxlama addımları:
1. Əhatə — klient AP-dən çox uzaqdır və ya arada divar/maneə var; klienti AP-yə
   yaxınlaşdır və ya əlavə AP yerləşdir.
2. Band — 2.4 GHz uzağa çatır amma yavaşdır; 5 GHz sürətlidir amma əhatəsi qısadır.
   Klient uzaqdırsa 2.4 GHz-də qalması normaldır.
3. AP seçimi — klient yaxın AP əvəzinə uzaq AP-yə "yapışıb" (sticky client);
   klienti yenidən qoşdur (roaming).
4. Kanal qarışıqlığı — ətrafda çox 2.4 GHz şəbəkə varsa interferens olur.

## Klient yavaş internetdədir

Simptom: klientin `unifi_client_rx_rate`/`tx_rate` dəyəri gözləniləndən aşağıdır.

Yoxlama addımları:
1. Siqnal — əvvəlcə RSSI-ni yoxla; zəif siqnal (yuxarıya bax) sürəti birbaşa aşağı salır.
2. Band — klient 2.4 GHz-dədirsə və 5 GHz dəstəkləyirsə, 5 GHz-ə keçmək sürəti artırır.
3. AP yükü — həmin AP-yə çox klient qoşulub (aşağıya bax); yük bölünməlidir.
4. Data həcmi — klient böyük yükləmə edir? Klientlər səhifəsində "Data" sütununu yoxla.

## Klient tez-tez qopur / yenidən qoşulur

Simptom: `unifi_client_connected_seconds` daim kiçik qalır (qısa sessiyalar).

Yoxlama addımları:
1. Sərhəd siqnal — RSSI -70..-75 arasında dəyişirsə klient qopub-qoşulur; əhatəni yaxşılaşdır.
2. Roaming — iki AP arasında klient gedib-gəlir; AP güclərini/yerləşməsini nəzərdən keçir.
3. Enerji qənaəti — bəzi cihazlar yuxu rejimində qopur; bu normaldır.

## AP-də çox klient (yük)

Simptom: bir AP-yə digərlərindən qat-qat çox klient qoşulub (WiFi analitika →
"AP-yə görə klientlər"); həmin AP-də CPU da yüksələ bilər.

Yoxlama addımları:
1. Say — WiFi səhifəsində AP üzrə klient sayına bax; bir AP çox yüklüdürsə congestion olur.
2. Bölüşdür — yaxınlıqda əlavə AP əlavə et və ya band steering ilə yükü paylaşdır.
3. CPU — yüklü AP-nin `unifi_device_cpu_percent` dəyərini yoxla; davamlı yüksəkdirsə real problemdir.

## Klient çox data işlədir (ağır istifadəçi)

Simptom: bir klientin "Data" (session GB, `unifi_client_rx_bytes + tx_bytes`) dəyəri
digərlərindən çox yüksəkdir.

Yoxlama addımları:
1. Kim — Klientlər səhifəsində "Data" sütununu azalan sırala; ən üstdəki ağır istifadəçidir.
2. Nə — Kerio Firewall loglarında həmin klientin trafikinə bax (məs. P2P/torrent bloklanıb?).
3. Qayda — lazım olsa Kerio-da həmin klient üçün trafik qaydası tətbiq edilməlidir (Kerio admin işi).
