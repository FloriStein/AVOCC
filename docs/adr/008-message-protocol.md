# ADR-008: Message Protocol Contract Layer

Status: Accepted

## Kontext

Das System verwendet drei Kommunikationskanäle (WebSocket Control, MQTT Telemetry, Safety Event Bus) ohne bisher ein einheitliches Message-Schema definiert zu haben. Ohne Schema-Contract entstehen inkonsistente Systemzustände, Interpretationsfehler zwischen Services und unkontrollierte Versionierungsprobleme. Das Latenz-Ziel von <100ms und die Safety-Kritikalität erfordern ein performantes, typsicheres und versionierbares Format.

## Optionen

### Option A: JSON

## Vorteile:
- Browser-nativ, kein Code-Gen
- Einfach lesbar und debuggbar

## Nachteile:
- Keine Schema-Sicherheit
- Keine native Versionierung
- Größerer Payload — riskant für <100ms Latenzziel
- Nicht geeignet für Safety-kritische Systeme

### Option B: Protocol Buffers (Protobuf)

## Vorteile:
- Strikt typisiert — verhindert Interpretationsfehler
- Schema-basierte Versionierung (backward/forward compatible)
- Kompakter Payload — besser für Latenz
- Ideal für Multi-Service-Echtzeitsysteme
- Code-Generation für Go und JavaScript verfügbar

## Nachteile:
- Code-Generation erforderlich
- Frontend benötigt Protobuf-Parser/Adapter
- Initialer Setup-Aufwand für `.proto` Repository

## Entscheidung

Wir wählen **Protocol Buffers (Protobuf)** als einheitliches Message Format für alle Kommunikationskanäle.

## Begründung

Protobuf garantiert konsistente Datenstrukturen über alle Services hinweg, verhindert Interpretationsfehler und ermöglicht schema-basierte Evolution ohne Breaking Changes. Der reduzierte Payload unterstützt das <100ms Latenzziel. Für Safety-kritische Systeme ist Typ- und Schema-Sicherheit nicht verhandelbar.

## Message Domains

```
proto/
├── common.proto      → CorrelationHeader (shared across all domains — ADR-016)
├── control.proto     → Control Commands + ControlAck (WebSocket)
├── telemetry.proto   → Telemetry Events (MQTT)
├── safety.proto      → Safety Events (Safety Bus)
└── session.proto     → Session Events (SFU Push) + RecordingEntry
```

`CorrelationHeader` (session_id ULID, event_id ULID, vehicle_id, operator_id, timestamp) ist in `common.proto` definiert und wird von allen anderen Schemas importiert. Er ermöglicht vollständige Session-Rekonstruktion über alle Protokollwelten (ADR-016).

## WebRTC Signaling — bewusst außerhalb Protobuf

WebRTC Signaling (SDP Offer/Answer, ICE Candidates) unterliegt **nicht** dem Protobuf-Schema. Es ist ein standardisiertes Interoperabilitätsprotokoll der Media-Schicht. Protobuf gilt ausschließlich für den systeminternen Application Bus (Control, Safety, Telemetry, Session).

## Prioritätsmodell

`Safety > Control > Telemetry`

Die technische Durchsetzung dieses Prioritätsmodells (getrennte Channels vs. Prioritäts-Flag im Header) ist noch offen — siehe DECISIONS.MD offene Folge-Entscheidungen.

## Konsequenzen

### Positiv:
- Konsistente, typsichere Kommunikation über alle Services
- Schema-Versionierung verhindert Breaking Changes
- CI kann Schema-Kompatibilität automatisch prüfen

### Negativ:
- Zentrales `.proto` Schema-Repository erforderlich
- Code-Generation für Go Backend und Frontend-Adapter nötig
- Versionierungsstrategie für Messages muss als Prozess etabliert werden
