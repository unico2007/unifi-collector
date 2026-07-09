# Sorğu nümunələri (PromQL / LogQL)

Tez-tez verilən suallar üçün hazır sorğular. Metrik sualı canlı icra olunur; bu
nümunələr həm də "necə sorğulamaq olar" suallarına cavab üçündür.

## Cihaz statusu

- Neçə cihaz online: `count(unifi_device_up == 1)`
- Offline cihazlar: `unifi_device_up == 0`
- Cihaz sayı vendor üzrə: `count by (vendor) (unifi_device_up)`

## CPU / yaddaş

- CPU 85%-dən yuxarı cihazlar: `unifi_device_cpu_percent > 85`
- Ən yüksək yaddaş: `topk(5, unifi_device_memory_percent)`
- 15 dəqiqəlik orta CPU: `avg_over_time(unifi_device_cpu_percent[15m])`

## Klientlər / WiFi

- Qoşulu klient sayı: `sum(unifi_clients_total)`
- Zəif siqnallı klientlər: `unifi_client_rssi < -75`
- AP üzrə klient sayı: `count by (ap) (unifi_client_rssi)`
- Zolaq bölgüsü: `count by (band) (unifi_client_rssi)`

## Trafik

- Ümumi endirmə sürəti (Mbps): `sum(rate(unifi_device_rx_bytes[5m])) * 8 / 1e6`
- AP üzrə trafik: `sum by (name) (rate(unifi_device_rx_bytes{type="uap"}[5m]))`

## Loglar (LogQL)

- UniFi error logları: `{vendor="unifi"} |= "error"`
- Kerio blok sətirləri: `{vendor="kerio"} |= "DENY"`
- Bütün vendorlar, error səviyyə: `{vendor=~"unifi|kerio"} | logfmt | level="error"`

## Sağlamlıq

- Subsystem statusu: `unifi_health_status`
- Problemli subsystem: `unifi_health_status < 1` (bu saytda adətən yalnız wlan real).
