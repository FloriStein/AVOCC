# DECISIONS.MD — Live-Übersicht aller Architekturentscheidungen

## Aktive ADRs

| ADR | Titel | Status | Entscheidung |
|-----|-------|--------|--------------|
| [ADR-001](docs/adr/001-backend-language.md) | Backend-Sprache & Framework | Accepted | Go |
| [ADR-002](docs/adr/002-safety-channel.md) | Safety Channel | Accepted | Safety Event Bus (kein DDS), DDS-Interface-kompatibel |
| [ADR-003](docs/adr/003-mqtt-broker.md) | MQTT Broker | Accepted | Eclipse Mosquitto |
| [ADR-004](docs/adr/004-authentication.md) | Authentifizierung | Accepted | Separater Auth Service (Operator + Vehicle, OAuth-erweiterbar) |
| [ADR-005](docs/adr/005-session-recording.md) | Session Recording | Accepted | Abstraktes Recording Interface (Persistenz als Folge-ADR offen) |
| [ADR-006](docs/adr/006-testing-strategy.md) | Testing Strategy | Accepted | testing+testify / Docker / Safety Test Suite / Jest+Playwright / CI Latenz / WebRTC E2E non-blocking |
| [ADR-007](docs/adr/007-system-runtime-topology.md) | System Runtime Topology | Accepted | Hub-and-Spoke (Control Hub ≠ Data Hub); CONTROL HUB > VIDEO HUB; SFU = orthogonaler zweiter Hub |
| [ADR-008](docs/adr/008-message-protocol.md) | Message Protocol Contract Layer | Accepted | Protobuf (Application Bus); WebRTC Signaling bewusst außerhalb |
| [ADR-009](docs/adr/009-failure-model.md) | Failure Model & System Behavior under Faults | Accepted | CRITICAL/DEGRADED/OBSERVATION; Video=DEGRADED; Command ACK Timeout=CRITICAL; 3 formale Invarianten |
| [ADR-010](docs/adr/010-control-loop.md) | Control Loop Definition & Safety Override | Accepted | Event-driven Stream / ACK-Roundtrip / Channel Close |
| [ADR-011](docs/adr/011-system-state-machine.md) | System State Machine | Accepted | 4-Layer: SYSTEM / CONTROL / MEDIA / OPERATOR STATE |
| [ADR-012](docs/adr/012-message-flow-runtime.md) | Message Flow Runtime Model | Accepted | Field-based Protobuf Versioning, CI Schema-Gate |
| [ADR-012b](docs/adr/012b-message-flow-runtime-sync-codegen.md) | Sync/Async-Modell & Code Generation | Accepted | Safety/MQTT async; Auth async+lokale JWT; Frontend sync ACK; Build-time protoc Code-Gen |
| [ADR-013](docs/adr/013-frontend-tech-stack.md) | Frontend Technology Stack | Accepted | React 18+ + TypeScript + Vite + Tailwind CSS + Shadcn/ui + protoc-gen-es |
| [ADR-014](docs/adr/014-video-streaming.md) | Video Streaming Technologie | Accepted | WebRTC SFU (Pion/Go) + coturn; 1 Primary + 1-2 Secondary; server-seitiges Recording; Handover |
| [ADR-015](docs/adr/015-session-coordinator.md) | Session Coordinator | Accepted | Control Session als primäre Einheit; GSA = Control Server; Ephemeral + Checkpoint; SFU Event-Push |
| [ADR-016](docs/adr/016-session-correlation-id.md) | Session Correlation ID | Accepted | ULID; Control Server generiert bei CONNECTING→CONNECTED; Vehicle-ID→Session-ID→Event-ID; JWT = Identity only |
| [ADR-017](docs/adr/017-logging-strategy.md) | Logging Strategy | Accepted | Hybrid: Technical Logs async (slog → Loki); Safety Events sync (AuditWriter.WriteSync); 3 Log-Klassen; Frontend via POST /log; Interface-first (ADR-018) |
| [ADR-018](docs/adr/018-audit-trail-strategy.md) | Audit Trail Strategy | Accepted | SQLite WAL (embedded, kein extra Service) als AuditWriter-Implementierung; garantierte Persistenz Safety Events; fsync vor SAFE_MODE |
| [ADR-019](docs/adr/019-deployment-strategy.md) | Deployment-Strategie | Accepted | Docker Hub private Repos + EC2 Elastic IP + AWS SSM Parameter Store; linux/amd64; kein Quellcode auf EC2; deploy.sh als Deployment-Mechanismus |

## Offene Folge-Entscheidungen

| Thema | Blockiert | Referenz |
|-------|-----------|----------|
| Prioritätsmodell technisch (getrennte Channels vs. Header-Flag) | offen | ADR-008 Folge — BE-04 nutzt Publisher-Pattern; explizite Channel-Trennung noch offen |
| Session Recording Storage (DB/Files/Object Storage) | offen | ADR-005 Folge — MemoryRecorder seit Sprint 4 als Platzhalter aktiv; Storage-ADR ausstehend |
| DDS-Produktivimplementierung | Nicht in diesem Scope | ADR-002 Folge — Safety Event Bus als DDS-Drop-in implementiert |
| Backup-Strategie Audit Store (SQLite Volume → S3) | nach Sprint 8 | ADR-018 Folge — S3-Bucket im CDK vorhanden; ADR-020 möglich |
| Migration zu AWS ECR | nach Sprint 8 | ADR-019 Folge — für Produktivbetrieb sinnvoll (IAM statt Docker Hub Credentials) |
| HTTPS / TLS-Terminierung auf EC2 | nach Sprint 8 | ADR-019 Folge — für Testphase HTTP akzeptabel |
| MQTT-Authentifizierung (Mosquitto Passwort-File) | nach Sprint 8 | Port 1883 aktuell ohne Auth offen — Vehicle-IP-Einschränkung in SG empfohlen |
