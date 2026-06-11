# Sprint 11 — Vehicle Connectivity & Feedback ✅

Ziel: Steuerbefehle kommen tatsächlich beim Fahrzeug an. Operator sieht Aktuator-Ist-Werte.

Datum: 2026-06-11 | **Status: Abgeschlossen ✅**
Vorgänger: Sprint 10 ✅ (Browser WebRTC ICE Migration + HTTPS + Browser WHIP Sender)

Vollständige Dokumentation: [`docs/sprints/sprint-11-vehicle-connectivity.md`](../docs/sprints/sprint-11-vehicle-connectivity.md)
Verification Report: [`docs/sprints/sprint-11-verification.md`](../docs/sprints/sprint-11-verification.md)

---

## Root Cause: Critical Gap

Vor diesem Sprint wurden Steuerbefehle **nie** zum Fahrzeug weitergeleitet.
`readLoop` in `internal/vehicleconnection/handler.go` hat alle eingehenden Nachrichten still verworfen.

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| VEH-01 | ADR-021 — Vehicle Connectivity & Feedback Architecture | L | ✅ |
| VEH-02 | `proto/vehicle.proto` — VehicleCommandAck | S | ✅ |
| VEH-03 | `proto/telemetry.proto` — Actuation Fields 7–11 | S | ✅ |
| VEH-04 | `internal/vehicleconnection/registry.go` — Registry + ForwardCommand | M | ✅ |
| VEH-05 | `internal/vehicleconnection/ackstore.go` — AckStore | S | ✅ |
| VEH-06 | `internal/controlserver/command/engine.go` — VehicleForwarder Interface | M | ✅ |
| VEH-07 | `cmd/control-server/main.go` — Verdrahtung + `GET /vehicle/ack/latest/` | M | ✅ |
| VEH-08 | `cmd/vehicle-mock/main.go` — JWT self-gen, WS, Protobuf, MQTT, Lerp | L | ✅ |
| VEH-09 | `vehicle-mock.Dockerfile` + Compose + nginx `/vehicle/` Proxy | M | ✅ |
| VEH-10 | `useVehicleAck.ts` — Hook 500ms-Polling `/vehicle/ack/latest/` | S | ✅ |
| VEH-11 | `InputIndicatorPanel.tsx` — Lenkrad-SVG + ActuationBars + AckBadge | M | ✅ |
| VEH-12 | Tests: `vehicleconnection_test.go` (7) + `InputIndicatorPanel.test.tsx` (7) | M | ✅ |

---

## Verification (E2E bestätigt)

- ✅ Go Build EXIT 0
- ✅ 26/26 Unit Tests (19 Safety + 7 vehicleconnection)
- ✅ 41/41 Frontend Tests
- ✅ STEER=0.75 → ACK <500ms → steer_actual=0.6375 (15% Lag, Lerp korrekt)
- ✅ SAFE_MODE bei WS-Disconnect (Safety-Invarianten intakt)
- ⚠️ Finding: `make up` startet Frontend lokal nicht ohne SSL-Cert (Sprint-10-Regression)

---

## Definition of Done

- [x] ControlCommand per WebSocket an Fahrzeug weitergeleitet (VehicleForwarder)
- [x] vehicle-mock: JWT self-gen, WS connect, Protobuf decode, ACK senden
- [x] VehicleCommandAck gespeichert, per `GET /vehicle/ack/latest/` abrufbar
- [x] TelemetryEvent mit Aktuator-Ist-Werten (steer/throttle, 15% Lag via Lerp)
- [x] InputIndicatorPanel: Lenkrad-SVG + ActuationBars + AckBadge im Footer
- [x] ADR-021 dokumentiert
- [x] Verification Report erstellt
- [x] Alle Tests grün (Go Build + 26 Unit + 41 Frontend)

---

## Nächste Schritte (Sprint 12 — Backlog)

Kein aktiver Sprint. Offene Punkte aus Backlog:
- Dev-Stack SSL-Fix: nginx.conf dev/prod trennen (Sprint-10-Regression)
- E2E Smoke Test auf EC2 mit Fahrzeug-Feedback
- vehicle-mock zu Makefile GO_SERVICES hinzufügen
- session_id in MQTT-TelemetryEvent (vehicle-mock Session-Kontext)
