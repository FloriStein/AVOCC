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

## Offene Folge-Entscheidungen

| Thema | Blockiert | Referenz |
|-------|-----------|----------|
| Prioritätsmodell technisch (getrennte Channels vs. Header-Flag) | Phase 4 | ADR-008 Folge |
| Session Recording Storage (DB/Files/Object Storage) | BE-07, vor Phase 4 | ADR-005 Folge |
| DDS-Produktivimplementierung | Nicht in diesem Scope | ADR-002 Folge |
