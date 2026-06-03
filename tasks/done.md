# Done

Lifecycle: backlog → sprint → done

---

## Sprint 1 — Foundation Layer

Abgeschlossen: 2026-06-03

### Tasks

| ID | Task | Typ | Ergebnis |
|----|------|-----|----------|
| INFRA-01 | Proto Schema Repository — `.proto` + CorrelationHeader | M | ✅ Alle 5 Schemas (common, control, telemetry, safety, session) inkl. CorrelationHeader. ULID-Lib konfiguriert. |
| FE-01 | React Projekt Setup — Vite + TypeScript + Tailwind + Shadcn | S | ✅ React 18 + TypeScript + Vite läuft, erreichbar auf Port 3000 |
| BE-01 | Auth Service — JWT Ausstellung (Operator + Vehicle) | M | ✅ JWT-Ausstellung für Operator (role=OBSERVER) und Vehicle (role=VEHICLE) verifiziert |
| BE-11 | STUN/TURN Service — coturn Setup & Config | S | ✅ coturn läuft als Docker-Container auf Port 3479 |
| BE-03 | Safety Event Bus — Interface + In-Memory Implementierung | M | ✅ EmergencyStop auslösbar, State korrekt (SafeMode: true, LastEvent: EMERGENCY_STOP) |
| BE-02 | Control Server — WebSocket Setup + JWT Auth Middleware | M | ✅ WS-Verbindung mit JWT-Auth (101 Switching Protocols), Log: `subject=operator-1 role=OBSERVER` |
| DC-01 | Dockerfile — Frontend (React) | S | ✅ Multi-stage build, nginx serving, Port 3000 |
| DC-02 | Dockerfile — Backend Services (Go) | M | ✅ Alle Go-Services als separate Images gebaut |
| DC-03 | Docker Compose — Multi-Service Orchestrierung | M | ✅ Alle 8 Services starten fehlerfrei via `docker-compose up` |

### Testprotokoll (2026-06-03)

| Test | Erwartung | Ergebnis |
|------|-----------|----------|
| Frontend localhost:3000 | HTML erreichbar | ✅ Vite + React + TS |
| Health /health alle Services | `{"status":"ok"}` | ✅ Alle 5 Services (8080–8084) |
| State Machine Initialzustand | `IDLE / CONTROL_INIT / MEDIA_INIT / NO_OPERATOR` | ✅ exakt |
| Operator JWT `POST /auth/operator/login` | `{"token":"eyJ..."}` mit `role=OBSERVER` | ✅ |
| Vehicle JWT `POST /auth/vehicle/register` | `{"token":"eyJ..."}` mit `role=VEHICLE` | ✅ |
| Safety Initial-State `GET /safety/state` | `SafeMode: false` | ✅ |
| Emergency Stop `POST /safety/emergency-stop` | SafeMode aktiviert | ✅ `SafeMode: true, LastEvent: EMERGENCY_STOP` |
| WebSocket Handshake mit JWT | `101 Switching Protocols`, Server-Log korrekt | ✅ Log: `WebSocket connected: subject=operator-1 role=OBSERVER` |
| WS-Disconnect → SAFE_MODE | System State wechselt | ✅ `SAFE_MODE / CONTROL_BLOCKED` nach Disconnect (ADR-009/010) |

### Beobachtung

WS-Disconnect triggert korrekt `SAFE_MODE → CONTROL_BLOCKED` (nicht im ursprünglichen Testplan, aber validiert). Safety-Verhalten funktioniert bereits auf Transport-Ebene wie in ADR-009/010 definiert.
