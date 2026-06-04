# Sprint 6 — Testing & Quality Gates

Ziel: Vollständige Test-Infrastruktur. CI läuft durch. Latenz-Ziel <100ms verifiziert.

Datum: 2026-06-03
Vorgänger: Sprint 5 ✅ (Feature Completion Frontend — Control Panel, Video Panel, Dashboard, Telemetrie)

---

## Ausgangslage (aus Sprint 5)

| Was existiert | Stand |
|---------------|-------|
| `frontend/src/hooks/useControls.ts` | 20 Hz Keyboard/Joystick/Gamepad → Protobuf STEER/THROTTLE/BRAKE Commands |
| `frontend/src/components/ControlPanel.tsx` | Virtual Joystick SVG, Speed Slider, Steer/Throttle Bars |
| `frontend/src/hooks/useWebRTC.ts` | RTCPeerConnection, SDP Signaling via `/sfu/subscribe/`, MEDIA STATE |
| `frontend/src/components/VideoPanel.tsx` | Video Element, MEDIA STATE Badge, Overlays |
| `frontend/src/hooks/useTelemetry.ts` | 1 Hz Polling `/telemetry/latest/{vehicleId}` |
| `frontend/src/App.tsx` | Vollständiges Dashboard — VideoPanel, ControlPanel, Telemetrie, Operator-Rolle |
| `infrastructure/docker/nginx.conf` | Docker DNS Resolver fix (kein 502 mehr nach Rebuild) |
| Safety Tests | 19/19 grün ✅ |

---

## Tasks

| ID | Task | Typ | Status | Notizen |
|----|------|-----|--------|---------|
| TEST-03 | Integration Test Infrastructure — Docker Test Environment | M | 🔲 Todo | ADR-006; Docker Compose für Tests; Playwright WebRTC-Flags (`--allow-insecure-localhost`); Basis für TEST-05 |
| TEST-04 | Frontend Test Infrastructure — Vitest + RTL + Playwright | M | 🔲 Todo | ADR-006; **Vitest statt Jest** (ESM-kompatibel mit Vite); Component-Tests für `SafetyPanel`, `ConnectionPanel`, `SafeModeOverlay`, `ControlPanel`, `VideoPanel`; Playwright E2E-Basis |
| TEST-05 | Performance / Latency Tests — CI Integration (<100ms) | M | 🔲 Todo | ADR-006; k6 + Go Benchmarks; ACK-Roundtrip <100ms mit echtem Protobuf ControlAck; Build-Fail bei Verletzung |
| DC-04 | Local Dev Environment — README finalisieren | S | 🔲 Todo | README.md Grundstruktur vorhanden; fehlt: Troubleshooting (WSL, Docker-Socket, Port-Konflikte, nginx DNS), Contributor-Guide |

---

## Abhängigkeitspfad

```
TEST-03 (Docker Test Env) ──────────────────┐
TEST-04 (Frontend Tests)  — parallel möglich ├→ TEST-05 (CI Latenz) ✓
DC-04   (README)          — unabhängig       ┘
```

---

## Sprint-Ziel / Definition of Done

- [ ] Docker Compose Test-Environment startet (`docker compose -f tests/docker-compose.test.yml up`)
- [ ] Vitest läuft: `npm run test` — Component-Tests für SafetyPanel, ConnectionPanel, SafeModeOverlay
- [ ] Playwright E2E: Browser öffnet Dashboard, Safety-Panel sichtbar
- [ ] Go Benchmark: ACK-Roundtrip-Messung implementiert
- [ ] k6 Latenz-Test: 100 VU, <100ms p99 ACK-Roundtrip (gegen laufenden Control Server)
- [ ] CI Build-Fail bei >100ms Latenz-Verletzung dokumentiert
- [ ] README: `docker-compose up`, Proto-Gen, Troubleshooting, Contributor-Guide
- [ ] Safety Regression: weiterhin 19/19 grün
