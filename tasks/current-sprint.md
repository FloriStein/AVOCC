# Sprint 1 — Foundation Layer

Ziel: Lauffähiges Grundgerüst — Proto-Schema mit CorrelationHeader, alle Core-Services als Skeleton, Docker Compose läuft lokal durch.

Datum: 2026-06-03

---

## Tasks

| ID | Task | Typ | Status | Notizen |
|----|------|-----|--------|---------|
| INFRA-01 | Proto Schema Repository — `.proto` + CorrelationHeader | M | 🔲 Todo | ADR-008/016; control/telemetry/safety/session.proto; CorrelationHeader (session_id, event_id, vehicle_id, operator_id, timestamp) in allen Schemas; ULID-Lib für Go + TS konfigurieren |
| FE-01 | React Projekt Setup — Vite + TypeScript + Tailwind + Shadcn | S | 🔲 Todo | ADR-013; keine Abhängigkeiten |
| BE-01 | Auth Service — JWT Ausstellung (Operator + Vehicle) | M | 🔲 Todo | ADR-004; Go; Basis für alle Backend-Services |
| BE-11 | STUN/TURN Service — coturn Setup & Config | S | 🔲 Todo | ADR-014; NAT Traversal für Vehicle ↔ Internet ↔ OCC; Docker-Container |
| BE-03 | Safety Event Bus — Interface + In-Memory Implementierung | M | 🔲 Todo | ADR-002; abhängig von INFRA-01 |
| BE-02 | Control Server — WebSocket Setup + JWT Auth Middleware | M | 🔲 Todo | ADR-007/008/010/011; abhängig von BE-01, INFRA-01; implementiert CONNECTING→AUTHENTICATED Transition |
| DC-01 | Dockerfile — Frontend (React) | S | 🔲 Todo | Abhängig von FE-01 |
| DC-02 | Dockerfile — Backend Services (Go) | M | 🔲 Todo | Abhängig von BE-01, BE-02, BE-03 |
| DC-03 | Docker Compose — Multi-Service Orchestrierung | M | 🔲 Todo | Abhängig von DC-01, DC-02, BE-11; alle Services inkl. coturn + Mosquitto; Sprint-Abschluss |

---

## Reihenfolge (Abhängigkeitspfad)

```
INFRA-01 ──────────────────────────┐
FE-01    → DC-01 ──────────────────┤
BE-01    → BE-02 → DC-02 → DC-03 ✓ │
BE-03 (braucht INFRA-01) ──────────┤
BE-11 (coturn) ────────────────────┘
```

---

## Sprint-Ziel / Definition of Done

- Proto-Schemas definiert (control, telemetry, safety, session) inkl. `CorrelationHeader` in allen Schemas
- ULID-Bibliothek für Go (`oklog/ulid`) und TypeScript konfiguriert
- Code-Gen (protoc-gen-go + protoc-gen-es) läuft im Build-Schritt
- Auth Service stellt JWTs aus (Operator + Vehicle)
- Control Server nimmt WSS-Verbindungen mit JWT-Auth und Protobuf an
- Control Server implementiert `CONNECTING → AUTHENTICATED` State Transition
- Safety Event Bus antwortet auf `EmergencyStop`
- coturn läuft als Docker-Container, STUN erreichbar
- React App im Browser erreichbar (leere Shell genügt)
- `docker-compose up` startet alle Services fehlerfrei
