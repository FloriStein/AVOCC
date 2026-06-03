# ADR-005: Session Recording — Abstraktes Recording Interface

Status: Accepted

## Kontext

Session Recording ist eine Kernanforderung für Auditierbarkeit und Nachvollziehbarkeit von Teleoperations-Sessions. Die konkreten Storage-Anforderungen (Video, Steuerereignisse, Safety-Events, Audit-Logs) sind noch nicht final entschieden. Eine frühe Bindung an eine konkrete Speichertechnologie (Datenbank, File-System, Object Storage) würde die Architektur einschränken und spätere Erweiterungen erschweren.

## Optionen

### Option A: Direkte Implementierung (File-System oder Datenbank)

## Vorteile:
- Sofort funktionsfähig
- Kein Interface-Overhead

## Nachteile:
- Frühe Bindung an konkrete Technologie
- Schwer austauschbar ohne Änderungen an allen Aufrufer-Services
- Video- und Event-Aufzeichnung haben unterschiedliche Storage-Anforderungen

### Option B: Abstraktes Recording Interface mit entkoppelter Persistenz

## Vorteile:
- Keine direkte Abhängigkeit von DB oder File-System
- Spätere Implementierungen als Adapter möglich (File, DB, Object Storage, Cloud)
- Ermöglicht schrittweise Erweiterung auf Video-/Audit-/Telemetry-Recording
- Testbar ohne reale Speicher-Infrastruktur (Mock-Adapter)

## Nachteile:
- Erhöhter initialer Design-Aufwand für Interface-Definition
- Konkrete Persistenz muss separat entschieden und als neues ADR dokumentiert werden

## Entscheidung

Wir wählen **Option B: Abstraktes Session Recording Interface** mit bewusst entkoppelter Persistenz.

## Begründung

Teleoperation erfordert flexible und erweiterbare Logging-Mechanismen. Storage-Anforderungen für Video, Events und Audit-Logs sind noch nicht final entschieden. Eine frühe Bindung an eine konkrete Speichertechnologie würde die Architektur einschränken. Das Interface-Muster ermöglicht, die Recording-Logik unabhängig vom Storage-Backend zu entwickeln und zu testen.

## Architektur

Das System arbeitet ausschließlich gegen ein `SessionRecorder`-Interface:

```go
StartSession(sessionID string, vehicleID string, operatorID string)
EndSession(sessionID string)
RecordControlEvent(header CorrelationHeader, event ControlEvent)
RecordStateSnapshot(header CorrelationHeader, state VehicleState)
RecordSafetyEvent(header CorrelationHeader, event SafetyEvent)
```

`sessionID` ist immer ein **ULID** (ADR-016) — generiert vom Control Server (GSA, ADR-015) beim Übergang `CONNECTING → CONNECTED`. Er dient als Root Key für alle Recording-Daten dieser Control Session und überlebt SAFE_MODE als Root-Anchor.

Die `CorrelationHeader`-Struktur ist in `proto/common.proto` definiert und enthält `session_id`, `event_id`, `vehicle_id`, `operator_id`, `timestamp`. Recording-Einträge können auch als `RecordingEntry` (definiert in `proto/session.proto`) serialisiert werden.

Die konkrete Implementierung (File-Adapter, DB-Adapter, Object-Storage-Adapter) wird als separater ADR entschieden, sobald die Storage-Anforderungen geklärt sind.

## Konsequenzen

### Positiv:
- Keine direkte Abhängigkeit von DB oder File-System in der Kernlogik
- Spätere Implementierungen als Adapter möglich ohne Änderung an Consumer-Services
- Erhöhte Flexibilität für Video-, Audit- und Telemetry-Erweiterung
- Mock-Adapter für Tests ohne Infrastruktur

### Negativ:
- Konkrete Persistenzentscheidung ist noch offen — muss als Folge-ADR getroffen werden
- Kein lauffähiges Storage-Backend bis Folge-ADR entschieden
