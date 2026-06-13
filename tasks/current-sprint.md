# Sprint 13 — Dev-Stack Stabilisierung & Log-Korrelation

Ziel: `make up` startet wieder vollständig. vehicle-mock ist im Build-Workflow. Alle MQTT-Telemetrie-Events tragen session_id.

Datum: 2026-06-13 | **Status: Abgeschlossen ✅**
Vorgänger: Sprint 12 ✅ (Vehicle Registry, ADR-022)

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| DEV-01 | `nginx.dev.conf` — HTTP-only für Dev-Stack; `docker-compose.yml` Volume-Mount | S | ✅ |
| DEV-02 | Makefile: `vehicle-mock` in `build-prod` + `push` (separates Dockerfile) | S | ✅ |
| DEV-03 | `cmd/vehicle-mock/main.go` — `session_id` aus ControlCommand extrahieren + in TelemetryEvent Header | M | ✅ |
