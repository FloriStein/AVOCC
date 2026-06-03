# ADR-014: Video Streaming Technologie — WebRTC SFU

Status: Accepted

## Kontext

Das System überträgt Video von Fahrzeugen an Operatoren über das offene Internet (Vehicle ↔ Internet ↔ OCC, uncontrolled routing). Bisher war UDP als Video-Kanal vorgesehen. UDP ist im Internet-Szenario ohne eigene NAT-Traversal-Infrastruktur nicht produktionstauglich (CGNAT, Firewalls, mobile Carrier blockieren UDP systematisch). Zusätzliche Anforderungen — server-seitiges Recording, Multi-Operator-Handover, adaptive Bitrate — erfordern eine server-seitige Relay-Architektur.

## Analysierende Anforderungen

| Anforderung | Ausprägung |
|-------------|-----------|
| Netzwerk | Vehicle ↔ Internet ↔ OCC (uncontrolled, NAT) |
| Videolatenz | QoS-Ziel 100–300ms (kein Safety-Hartziel) |
| Multi-Kamera | 1 Primary (always active) + 1–2 Secondary (on-demand) |
| Recording | Server-seitig primär (Audit), client-seitig optional |
| Multi-Operator | Handover Model: Active Operator + Observer/Standby |
| Video-Failure | DEGRADED — kein SAFE_MODE Trigger |

## Optionen

### Option A: UDP Video Streaming

## Vorteile:
- Sehr niedrige Latenz möglich
- Einfach in Docker

## Nachteile:
- NAT Traversal nicht eingebaut — im Internet-Szenario nicht produktionstauglich
- CGNAT, Firewalls blockieren UDP systematisch
- Kein browser-nativer Player — eigener MSE-Player nötig
- Adaptive Bitrate manuell implementieren
- Server-seitiges Recording komplex

### Option B: WebRTC P2P (ohne Media Server)

## Vorteile:
- Niedrigste Latenz

## Nachteile:
- Server-seitiges Recording nicht möglich
- Multi-Operator nur durch mehrere P2P-Verbindungen (nicht skalierbar)
- P2P ohne Media Server inkompatibel mit Server-seitigem Monitoring

### Option C: WebRTC Media Server / SFU

## Vorteile:
- NAT Traversal built-in (ICE/STUN/TURN)
- Server-seitiges Recording native
- Multi-Operator: SFU forwardet denselben Stream an n Empfänger
- Adaptive Bitrate via RTCP Feedback built-in
- Browser-nativ, kein Plugin
- Encryption (DTLS-SRTP) mandatory
- Orthogonal zum Control Hub (ADR-007): eigener Video Hub
- Go-native Implementierung via Pion (ADR-001-konsistent)

## Nachteile:
- Zusätzliche Services in Docker Compose (SFU + coturn)
- Testing-Komplexität erhöht (STUN/TURN in CI)
- Höherer initialer Implementierungsaufwand

## Entscheidung

Wir wählen **Option C: WebRTC SFU** mit Pion (Go) als Media Server.

## Begründung

Das Vehicle ↔ Internet ↔ OCC Szenario macht NAT Traversal zwingend. UDP ohne ICE/STUN/TURN ist in diesem Szenario nicht produktionstauglich. Server-seitiges Recording und Multi-Operator-Handover erfordern einen server-seitigen Relay-Punkt — ein SFU erfüllt alle drei Anforderungen gleichzeitig. Pion ist die führende Go-native WebRTC-Bibliothek und konsistent mit ADR-001 (Go Backend).

## Architektur

```
Vehicle (Kamera) → WebRTC SFU (Pion/Go)
                         ├── Primary Stream → Operator 1 (Active)
                         ├── Primary Stream → Operator 2 (Observer)
                         ├── Secondary Stream → on-demand
                         └── Recording Service (server-seitig)
```

## Kamera-Modell

- **Primary Stream:** immer aktiv, immer an alle Operatoren forwarded
- **Secondary Streams (1–2):** on-demand, Operator subscribed bei Bedarf
- **Adaptive Bitrate:** WebRTC RTCP Feedback automatisch

## Signaling

WebRTC Signaling (SDP Offer/Answer, ICE Candidates) läuft über den bestehenden WebSocket-Kanal. **Signaling-Nachrichten sind bewusst außerhalb des Protobuf-Schemas** — WebRTC Signaling ist ein standardisiertes Interoperabilitätsprotokoll der Media-Schicht, kein systeminterner Application-Bus-Payload (ADR-008 Scope-Abgrenzung).

## ADR-008 Schichtenklarheit

| Schicht | Protokoll | Scope |
|---------|-----------|-------|
| Application Layer | Protobuf | Control, Safety, Telemetry |
| Media Layer | WebRTC Standard (SDP/ICE/DTLS-SRTP) | Video — bewusst außerhalb Protobuf |

## Docker Compose — Neue Services

- `webrtc-sfu` — Pion-basierter Go SFU Service
- `stun-turn` — coturn (STUN/TURN Server für NAT Traversal)

## Failure Model (Mapping in SYSTEM STATE)

| Media State | System Impact |
|-------------|--------------|
| MEDIA_CONNECTED | Normal |
| MEDIA_DEGRADED | DEGRADED |
| MEDIA_FAILED | DEGRADED |

**Kein SAFE_MODE Trigger durch Video.** Video = Awareness only.

## Konsequenzen

### Positiv:
- Produktionstauglich im Internet-Szenario (NAT Traversal, CGNAT)
- Server-seitiges Recording ohne zusätzliche Streaming-Infrastruktur
- Multi-Operator-Handover nativ unterstützt
- Adaptive Bitrate und Verschlüsselung built-in
- ADR-001 konsistent (Go/Pion)

### Negativ:
- coturn als zusätzlicher Docker-Compose-Service
- CI-Test-Setup erfordert STUN/TURN-Container
- Playwright-Tests brauchen WebRTC-Browser-Flags
- Pion-SFU muss implementiert werden (kein Off-the-Shelf-Service)
