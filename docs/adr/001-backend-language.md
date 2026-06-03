# ADR-001: Backend-Sprache & Framework

Status: Accepted

## Kontext

Das System benötigt mehrere Backend-Services (Control Server, Telemetry Service, Safety Service). Keine Sprache war bisher festgelegt. Die Wahl beeinflusst Latenz, Ressourcenverbrauch, DDS/MQTT-Bibliotheksverfügbarkeit und langfristige Wartbarkeit aller Backend-Komponenten. Der Control-Loop hat ein Latenz-Ziel von <100ms.

## Optionen

### Option A: Node.js / TypeScript

## Vorteile:
- Native WebSocket-Unterstützung
- Gleiche Sprache wie React-Frontend möglich
- Großes Ökosystem

## Nachteile:
- DDS-Bibliotheken weniger ausgereift
- Single-threaded Event Loop unter hoher Last

### Option B: Python / FastAPI

## Vorteile:
- Schnelles Prototyping
- Gute MQTT-Bibliotheken (paho-mqtt)
- Große Community

## Nachteile:
- Höhere Latenz unter Last
- GIL-Limitation bei echter Parallelverarbeitung

### Option C: Go

## Vorteile:
- Sehr niedrige Latenz
- Geringe Ressourcennutzung
- Robuste native Concurrency (Goroutinen)
- Statisch kompiliert, einfache Container-Images
- Kein GIL, echte Parallelverarbeitung

## Nachteile:
- Kleineres Ökosystem für DDS-Bibliotheken
- Mehr Boilerplate als Python

## Entscheidung

Wir wählen **Go** als Backend-Sprache für alle Backend-Services.

## Begründung

Das <100ms Latenz-Ziel im Control-Loop und die Anforderung an robuste Parallelverarbeitung (gleichzeitig: WebSocket-Verbindungen, Safety-Events, Telemetrie) machen Go zur optimalen Wahl. Die native Concurrency über Goroutinen passt direkt auf das Multi-Channel-Architekturmodell. Schlanke Container-Images vereinfachen das Docker-Compose-Setup.

## Konsequenzen

### Positiv:
- Konsistente Sprache über alle Backend-Services
- Geringe Laufzeit-Ressourcen pro Container
- Gute Testbarkeit durch statisches Typsystem

### Negativ:
- DDS-Bibliotheken für Go weniger ausgereift (irrelevant durch ADR-002)
- Höherer initialer Lernaufwand falls neue Entwickler kein Go kennen
