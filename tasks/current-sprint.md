# Sprint 1 — Foundation Layer

Ziel: Lauffähiges Grundgerüst — Proto-Schema mit CorrelationHeader, alle Core-Services als Skeleton, Docker Compose läuft lokal durch.

Datum: 2026-06-03
Status: ✅ Abgeschlossen

---

## Tasks

| ID | Task | Typ | Status | Notizen |
|----|------|-----|--------|---------|
| INFRA-01 | Proto Schema Repository — `.proto` + CorrelationHeader | M | ✅ Done | ADR-008/016; control/telemetry/safety/session.proto; CorrelationHeader (session_id, event_id, vehicle_id, operator_id, timestamp) in allen Schemas; ULID-Lib für Go + TS konfiguriert |
| FE-01 | React Projekt Setup — Vite + TypeScript + Tailwind + Shadcn | S | ✅ Done | ADR-013; keine Abhängigkeiten |
| BE-01 | Auth Service — JWT Ausstellung (Operator + Vehicle) | M | ✅ Done | ADR-004; Go; Basis für alle Backend-Services |
| BE-11 | STUN/TURN Service — coturn Setup & Config | S | ✅ Done | ADR-014; NAT Traversal für Vehicle ↔ Internet ↔ OCC; Docker-Container |
| BE-03 | Safety Event Bus — Interface + In-Memory Implementierung | M | ✅ Done | ADR-002; abhängig von INFRA-01 |
| BE-02 | Control Server — WebSocket Setup + JWT Auth Middleware | M | ✅ Done | ADR-007/008/010/011; abhängig von BE-01, INFRA-01; implementiert CONNECTING→AUTHENTICATED Transition |
| DC-01 | Dockerfile — Frontend (React) | S | ✅ Done | Abhängig von FE-01 |
| DC-02 | Dockerfile — Backend Services (Go) | M | ✅ Done | Abhängig von BE-01, BE-02, BE-03 |
| DC-03 | Docker Compose — Multi-Service Orchestrierung | M | ✅ Done | Abhängig von DC-01, DC-02, BE-11; alle Services inkl. coturn + Mosquitto; Sprint-Abschluss |

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

- [x] Proto-Schemas definiert (control, telemetry, safety, session) inkl. `CorrelationHeader` in allen Schemas
- [x] ULID-Bibliothek für Go (`oklog/ulid`) und TypeScript konfiguriert
- [x] Code-Gen (protoc-gen-go + protoc-gen-es) läuft im Build-Schritt
- [x] Auth Service stellt JWTs aus (Operator + Vehicle)
- [x] Control Server nimmt WSS-Verbindungen mit JWT-Auth und Protobuf an
- [x] Control Server implementiert `CONNECTING → AUTHENTICATED` State Transition
- [x] Safety Event Bus antwortet auf `EmergencyStop`
- [x] coturn läuft als Docker-Container, STUN erreichbar
- [x] React App im Browser erreichbar (leere Shell genügt)
- [x] `docker-compose up` startet alle Services fehlerfrei

→ Alle Kriterien erfüllt. Testprotokoll in `tasks/done.md`. Nächste Phase: Sprint 2 (Safety & Failure Model).
