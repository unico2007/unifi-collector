# Unico şəbəkəsi — ümumi baxış

Unico bir şirkət şəbəkəsinin (UniFi access point-lər + Kerio Control firewall)
monitorinq platformasıdır. Data axını: UniFi + Kerio → Collector → Prometheus
(metriklər) + Loki (loglar) → BFF (:80) → React panel + yerli AI köməkçi.

## Avadanlıq və vendorlar

- **UniFi** — controller `https://10.10.0.3` (UniFi OS), read-only hesab
  `helpdesk_unico`. Təxminən 26 cihaz, ~110-125 klient. Əsasən access point-lər
  (uap), switch-lər (usw). UniFi Remote Logging (CEF) → collector → Loki işləyir.
- **Kerio Control** — gateway/firewall `https://10.10.0.1:4081`, read-only hesab
  `log`. Bu saytda internet gateway-i Kerio-dur (UniFi USG YOXDUR). Kerio firewall
  syslog-u Loki-yə axır; Firewall səhifəsi canlıdır.

## Vacib fakt: gateway Kerio-dur

UniFi tərəfdə wan/lan/vpn/www subsystem-ləri 0 idarə olunan cihaz göstərir, çünki
routing/firewall Kerio-dadır. Bu offline və ya problem demək DEYİL — normaldır.
Yalnız `wlan` subsystem-i real (WiFi) sağlamlığı əks etdirir.

## Metrik namespace və label-lar

Bütün metriklər `unifi_` prefiksi ilədir. `vendor` label-i "unifi" və ya "kerio"
ola bilər. Cihaz metrikləri: name, model, type, ip, mac, state label-ları daşıyır.
Klient metrikləri: name, mac, ap (AP-nin MAC-i), vlan, band, rssi.

## VLAN-lar

Bu bölmə şablondur — real VLAN planını buraya əlavə edin (məs. VLAN 10 = Ofis,
VLAN 20 = IT, VLAN 90 = Qonaq). Klientlərin VLAN-ı `unifi_client_rssi{vlan=...}`
label-ında görünür; WiFi analitika səhifəsində VLAN bölgüsü var.

## Deploy və məhdudiyyətlər

Server: Windows box, LAN 10.10.1.229, Docker Desktop. Kerio yalnız read-only
(dəyişiklik Kerio admin tərəfindən). Panel LAN-da `http://10.10.1.229/`.
