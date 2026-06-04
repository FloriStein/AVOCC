# Architecture Decision Records (ADRs)

Alle Architekturentscheidungen des AVOC-Systems. Jede Entscheidung ist unveränderlich — neue Erkenntnisse führen zu einem neuen ADR, niemals zur stillen Überschreibung eines bestehenden.

Vollständige Live-Übersicht: [DECISIONS.MD](../../DECISIONS.MD)

---

## ADR-Index (18 ADRs)

| ADR | Titel | Kernentscheidung |
|-----|-------|-----------------|
| [ADR-001](001-backend-language.md) | Backend-Sprache | Go |
| [ADR-002](002-safety-channel.md) | Safety Channel | Safety Event Bus (Go, In-Memory, DDS-ready) |
| [ADR-003](003-mqtt-broker.md) | MQTT Broker | Eclipse Mosquitto |
| [ADR-004](004-authentication.md) | Authentifizierung | Separater Auth Service; Operator-Rollen; JWT = Identity only |
| [ADR-005](005-session-recording.md) | Session Recording | Abstraktes Interface; ULID als Session Root Key |
| [ADR-006](006-testing-strategy.md) | Testing Strategy | testing+testify / Docker / Safety Suite / Jest+Playwright / CI Latenz; WebRTC E2E non-blocking |
| [ADR-007](007-system-runtime-topology.md) | System Topologie | Hub-and-Spoke; CONTROL HUB > VIDEO HUB; GSA = Control Server |
| [ADR-008](008-message-protocol.md) | Message Protocol | Protobuf (Application Bus); common.proto mit CorrelationHeader; WebRTC außerhalb |
| [ADR-009](009-failure-model.md) | Failure Model | CRITICAL/DEGRADED/OBSERVATION; Video = DEGRADED; 3 formale Invarianten |
| [ADR-010](010-control-loop.md) | Control Loop | Event-driven Stream; ACK-Roundtrip <100ms; Channel Close als Safety Override |
| [ADR-011](011-system-state-machine.md) | State Machine | 4-Layer: SYSTEM / CONTROL / MEDIA / OPERATOR |
| [ADR-012](012-message-flow-runtime.md) | Message Flow | Field-based Protobuf Versioning; CI Schema-Gate |
| [ADR-012b](012b-message-flow-runtime-sync-codegen.md) | Sync/Async & Code-Gen | Safety/MQTT async; Auth async+lokal; Frontend sync ACK; Build-time protoc |
| [ADR-013](013-frontend-tech-stack.md) | Frontend Stack | React 18 + TypeScript + Vite + Tailwind + Shadcn/ui + protoc-gen-es |
| [ADR-014](014-video-streaming.md) | Video Streaming | WebRTC SFU (Pion/Go) + coturn; 1 Primary + 1-2 Secondary; Handover; Recording |
| [ADR-015](015-session-coordinator.md) | Session Coordinator | Control Session als primäre Einheit; GSA; Ephemeral + Checkpoint; SFU Event-Push |
| [ADR-016](016-session-correlation-id.md) | Correlation ID | ULID; Vehicle-ID → Session-ID → Event-ID; JWT = Identity only |
| [ADR-017](017-logging-strategy.md) | Logging Strategy | Hybrid: Technical async (slog → Loki); Safety sync (AuditWriter.WriteSync); 3 Log-Klassen; Interface-first |
| [ADR-018](018-audit-trail-strategy.md) | Audit Trail Strategy | SQLite WAL als AuditWriter; fsync vor SAFE_MODE; garantierte Safety-Event-Persistenz; kein extra Service |

---

## Offene Folge-Entscheidungen

| Thema | Blockiert | Referenz |
|-------|-----------|----------|
| Prioritätsmodell technisch (Channels vs. Header-Flag) | nach Sprint 7 | ADR-008 Folge |
| Session Recording Storage (DB/Files/Object Storage) | nach Sprint 7 | ADR-005 Folge — MemoryRecorder aktiv seit Sprint 4 |
| DDS-Produktivimplementierung | Nicht in diesem Scope | ADR-002 Folge |
| Backup-Strategie Audit Store | nach LOG-10 | ADR-019 möglich — SQLite-Volume-Sicherung für Produktivbetrieb |

---

## Template

Neue ADRs verwenden [000-template.md](000-template.md) als Basis.
