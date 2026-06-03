# ADR-003: MQTT Broker — Eclipse Mosquitto

Status: Accepted

## Kontext

Der Telemetrie-Kanal des Systems verwendet MQTT für Fahrzeugstatus und Telemetriedaten (pub/sub). Es wird ein MQTT Broker benötigt, der als eigenständiger Container in Docker Compose läuft und für lokale Entwicklung geeignet ist. Der Broker muss zuverlässig, leichtgewichtig und einfach zu konfigurieren sein.

## Optionen

### Option A: Eclipse Mosquitto

## Vorteile:
- Extrem leichtgewichtig (minimaler RAM/CPU-Verbrauch)
- Einfache Konfiguration über einzelne Config-Datei
- De-facto-Standard für lokale MQTT-Setups
- Kleines Docker-Image
- MQTT 3.1.1 und 5.0 Support

## Nachteile:
- Kein integriertes Web-Dashboard
- Nur minimales built-in Monitoring

### Option B: EMQX

## Vorteile:
- Webbasiertes Management-Dashboard
- MQTT 5.0 vollständig
- Bessere Skalierbarkeit für Produktion

## Nachteile:
- Deutlich größeres Container-Image
- Erheblicher Overhead für lokale Entwicklung
- Overengineering für aktuelle Projektphase

## Entscheidung

Wir wählen **Eclipse Mosquitto** als MQTT Broker.

## Begründung

Das System befindet sich in einer lokalen Entwicklungsphase mit Docker Compose. Mosquitto erfüllt alle Anforderungen (pub/sub, MQTT 5.0, stabile Verbindung) mit minimalem Ressourcenverbrauch. Der Overhead von EMQX ist für diese Phase nicht gerechtfertigt. Ein späterer Austausch gegen einen skalierbaren Broker (EMQX, HiveMQ) ist möglich, da die MQTT-Clients protokollkonform kommunizieren.

## Konsequenzen

### Positiv:
- Minimaler Container-Overhead im Docker-Compose-Stack
- Einfache Konfiguration und Wartung
- Schnell einsatzbereit

### Negativ:
- Kein GUI-Monitoring out-of-the-box (ggf. externe Tools wie MQTT Explorer nötig)
- Bei Skalierungsanforderungen muss Broker-Austausch als neues ADR entschieden werden
