# Sprint 12 — Vehicle Registry (ADR-022) ✅

Ziel: Mehrere Fahrzeuge können in einer SQLite-Datenbank verwaltet werden. Operator wählt vor Session-Start explizit ein Fahrzeug aus.

Datum: 2026-06-12 | **Status: Abgeschlossen ✅**
Vorgänger: Sprint 11 ✅ (Vehicle Connectivity & Feedback, ADR-021)

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| VEH-REG-01 | ADR-022 — Vehicle Registry Architecture | L | ✅ |
| VEH-REG-02 | `pkg/audit/sqlite_writer.go` — `DB() *sql.DB` getter | S | ✅ |
| VEH-REG-03 | `internal/vehicleregistry/` — VehicleStore Interface + SQLiteVehicleStore + NoopVehicleStore | M | ✅ |
| VEH-REG-04 | `cmd/control-server/main.go` — Store-Init, SeedDefault, `GET/POST/DELETE /vehicles` | M | ✅ |
| VEH-REG-05 | `frontend/src/lib/api-client.ts` — `VehicleInfo` + `listVehicles()` | S | ✅ |
| VEH-REG-06 | `frontend/src/hooks/useVehicles.ts` — 2s-Polling Hook | S | ✅ |
| VEH-REG-07 | `frontend/src/hooks/useSession.ts` — `startSession(vehicleId)` statt hardcoded VEHICLE_ID | M | ✅ |
| VEH-REG-08 | `frontend/src/components/VehicleSelector.tsx` — Dropdown + "Session starten"-Button | M | ✅ |
| VEH-REG-09 | `SafetyPanel.tsx` + `ControlPanel.tsx` — `vehicleId: string \| null` Prop | S | ✅ |
| VEH-REG-10 | `frontend/src/components/ConnectionPanel.tsx` — VehicleSelector bei AUTHENTICATED | S | ✅ |
| VEH-REG-11 | `frontend/src/App.tsx` — Auto-Start entfernt, vehicleId-Prop-Chain | M | ✅ |
| VEH-REG-12 | Bugfix: `DELETE /vehicles/{id}` → 404 wenn nicht gefunden (ErrNotFound-Sentinel) | S | ✅ |
| VEH-REG-13 | Bugfix: `POST /vehicles` — malformed JSON → `"invalid JSON"` (getrennt von Feldvalidierung) | S | ✅ |

---

## Verification (E2E bestätigt)

- ✅ Docker build EXIT 0 (Go + TypeScript)
- ✅ `vehicle-001` auto-geseedet beim Start, `online: true` (vehicle-mock verbunden)
- ✅ `POST /vehicles` → 201; Duplikat → 409; fehlende Felder → 400; kaputtes JSON → 400 "invalid JSON"
- ✅ `DELETE /vehicles/{id}` → 204 (existiert), 404 (nicht gefunden), 409 (aktive Session)
- ✅ Persistenz: vehicle-002 überlebt control-server Neustart; SeedDefault kein Duplikat
- ✅ SQL-Injection-Probe: Parameterized Queries schützen korrekt
- ✅ Vite-Proxy liefert `/api/vehicles` korrekt
- ✅ `VEHICLE_ID`-Hardcoding vollständig entfernt aus allen Frontend-Komponenten

---

## Definition of Done

- [x] ADR-022 dokumentiert
- [x] `vehicles`-Tabelle in bestehender audit.db (shared WAL-Connection)
- [x] `GET/POST/DELETE /vehicles` Endpoints mit korrekten Status-Codes
- [x] VehicleSelector erscheint im ConnectionPanel bei AUTHENTICATED
- [x] `vehicle-001` auto-geseedet (INSERT OR IGNORE), backward-kompatibel mit vehicle-mock
- [x] Online-Status live aus WS-Registry (nicht persistiert)
- [x] ADR-015-Invariante (1 aktive Session) bleibt erhalten — DELETE aktives Fahrzeug → 409

---

## Nächste Schritte (Backlog)

- Dev-Stack SSL-Fix: nginx.conf dev/prod trennen (Sprint-10-Regression)
- E2E Smoke Test auf EC2 mit Fahrzeug-Feedback
- vehicle-mock zu Makefile GO_SERVICES hinzufügen
- session_id in MQTT-TelemetryEvent (vehicle-mock Session-Kontext)
- Admin-UI für Vehicle CRUD (aktuell nur via curl/API)
