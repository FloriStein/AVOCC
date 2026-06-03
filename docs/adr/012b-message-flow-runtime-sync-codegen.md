# ADR-012b: Message Flow Runtime — Sync/Async-Modell & Protobuf Code Generation

Status: Accepted

## Kontext

ADR-012 hat Field-based Protobuf Versioning entschieden, ließ aber zwei Punkte offen:
1. Welche Kanäle im Hub-and-Spoke-Modell sind synchron, welche asynchron?
2. Wie wird Protobuf im Browser dekodiert und wie wird Code generiert?

Diese Entscheidungen blockieren FE-02 (WebSocket Client), BE-04 (Command Routing) und FE-09 (Protobuf Adapter).

---

## Teil 1: Sync/Async-Modell je Kommunikationskanal

### Entscheidungen

| Kanal | Modell | Begründung |
|-------|--------|-----------|
| **Control Server → Safety Event Bus** | **Async (fire-and-forget)** | Safety darf Control nicht blockieren; Safety Bus agiert als unabhängiger Override Layer |
| **Control Server → Auth Service** | **Async + lokale JWT-Validierung** | Auth Service nur für Token-Issuance und Revocation; lokale Validierung verhindert Auth-Bottleneck im Control Loop |
| **Frontend → Control Server** | **Synchron (ACK-based, non-blocking UI)** | Grundlage für das <100ms Latenz-Messmodell (ADR-010); ermöglicht deterministische Performance Tests |
| **Control Server → MQTT Telemetry** | **Async (one-way)** | Telemetrie ist immer fire-and-forget, kein Blocking |

### Konsequenzen

- Safety Bus kann Control nie blockieren — Safety Override wirkt durch State Transition, nicht durch Blocking (konsistent mit ADR-010 Channel Close)
- Auth Service Ausfall blockiert nicht den laufenden Control Loop — nur Issuance neuer Tokens
- Frontend-ACK ist der MessPunkt für <100ms (ADR-010) — UI darf während Warten nicht blockieren (non-blocking UI pattern)
- Lokale JWT-Validierung erfordert Public Key Distribution an alle Services

---

## Teil 2: Protobuf Code Generation Strategy

### Entscheidung: Build-time Code Generation, nicht im Repository versioniert

### Umsetzung
- `protoc` wird im Build- und CI-Prozess ausgeführt
- Generierte Dateien werden in `.gitignore` ausgeschlossen
- Build Pipeline stellt sicher, dass Code vor Kompilierung generiert wird
- Gilt für Go Backend (protoc-gen-go) und Frontend (protoc Plugin)

### Begründung
- `.proto`-Dateien bleiben Single Source of Truth — kein Drift möglich
- Verhindert Merge-Konflikte durch generierte Code-Diffs
- CI-Konsistenz und Reproduzierbarkeit verbessert
- Schema-Änderungen erzwingen Build — keine veralteten generierten Dateien

### Konsequenzen
- Build-Step erforderlich für Frontend und Backend
- CI benötigt `protoc` + entsprechende Plugins installiert
- Lokales Dev-Setup muss Code-Generation unterstützen (Makefile / Dev-Script)
- Welches protoc-Plugin für das Frontend (TypeScript vs. JavaScript) ist noch offen — siehe offene Folge-Entscheidung

---

## Offene Folge-Entscheidung

**TypeScript im Frontend:** Wenn `protoc-gen-es` (TypeScript-Klassen) gewählt wird, wird TypeScript im Frontend eingeführt. Das erfordert ein eigenes ADR, da JavaScript aktuell die Vorgabe ist (CONTEXT.MD). Muss vor FE-09 entschieden werden.
