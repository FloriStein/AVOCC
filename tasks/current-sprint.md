# Sprint 6 — Testing & Quality Gates

Ziel: Vollständige Test-Infrastruktur. CI läuft durch. Latenz-Ziel <100ms automatisch verifiziert. README produktionsreif.

Datum: 2026-06-04
Vorgänger: Sprint 5 ✅ (Feature Completion Frontend)

---

## Ausgangslage (aus Sprint 5)

| Was existiert | Stand |
|---------------|-------|
| Safety Test Suite | 19/19 grün (Sprint 2) — Basis für Regression |
| Protobuf ControlAck | Echter Binary-ACK (BE-04) — k6-Latenz jetzt messbar |
| `docker-compose.yml` | Alle 8 Services (inkl. SFU, coturn, MQTT) — Basis für TEST-03 |
| `frontend/src/gen/*.ts` | protoc-gen-es v2 generiert — Vitest-Tests compilieren lokal |
| `vite.config.ts` | `/sfu/` + `/telemetry/` Proxy ergänzt (Sprint 5) |
| ADR-017/018 | Logging-Strategie entschieden — **LOG-Tasks sind Phase 7, nicht Sprint 6** |
| Deadman | Armed-Pattern + 10s Timeout (Sprint 5 Bugfix) — Safety-Tests aktualisiert |

**Wichtig:** Sprint 6 ist rein Testing & Quality Gates. Phase 7 (Logging, LOG-01..11) folgt als Sprint 7.

---

## Tasks

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|----------------|
| TEST-03 | Integration Test Docker Environment | M | 🔲 Todo | DC-03 ✅ |
| TEST-04 | Frontend Tests — Vitest + RTL + Playwright E2E | M | 🔲 Todo | FE-01 ✅, gen/ ✅ |
| TEST-05 | Latenz-Tests CI — k6 + Go Benchmark (<100ms Build-Fail) | M | 🔲 Todo | BE-02 ✅, BE-04 ✅, TEST-03 |
| DC-04 | README — Troubleshooting + Contributor Guide | S | 🔲 Todo | DC-03 ✅ |

---

## Abhängigkeitspfad

```
TEST-03 (Docker Test Env) ──────────────────────┐
TEST-04 (Frontend Tests)  — parallel zu TEST-03 ─┼→ TEST-05 (CI Latenz) ✓
DC-04   (README)          — unabhängig           ┘
```

---

## Implementierungsdetails je Task

### TEST-03 — Integration Test Docker Environment

**Neue Datei:** `tests/docker-compose.test.yml`

Minimaler Stack für Integration Tests (ohne WebRTC/coturn — zu flaky in CI per ADR-006):

```yaml
services:
  control-server:   # Port 8080
  auth-service:     # Port 8081
  safety-service:   # Port 8082
  mosquitto:        # Port 1883
```

**Health-Check-Script:** `tests/integration/health_test.sh`
- Alle Services antworten auf `/health` → HTTP 200
- `docker compose -f tests/docker-compose.test.yml up -d && ./health_test.sh`

**`Makefile`-Target:**
```makefile
test-integration:
    docker compose -f tests/docker-compose.test.yml up -d
    sleep 3
    go test ./tests/integration/... -v -timeout 60s
    docker compose -f tests/docker-compose.test.yml down
```

**Playwright-Flags** (für spätere E2E mit WebRTC):
```
--allow-insecure-localhost
--use-fake-ui-for-media-stream
--use-fake-device-for-media-stream
```
→ In `tests/playwright.config.ts` als `launchOptions` hinterlegen.

---

### TEST-04 — Frontend Tests: Vitest + RTL + Playwright

**Packages installieren:**
```bash
cd frontend
npm install --save-dev vitest @vitest/coverage-v8 \
  @testing-library/react @testing-library/user-event \
  @testing-library/jest-dom jsdom \
  @playwright/test
```

**`frontend/vitest.config.ts`:**
```typescript
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
  },
  resolve: { alias: { '@': path.resolve(__dirname, './src') } },
})
```

**`frontend/src/test/setup.ts`:**
```typescript
import '@testing-library/jest-dom'
```

**`package.json` Scripts ergänzen:**
```json
"test": "vitest run",
"test:watch": "vitest",
"test:coverage": "vitest run --coverage",
"test:e2e": "playwright test"
```

**Component-Tests (Priorität):**

| Datei | Tests |
|-------|-------|
| `SafetyPanel.test.tsx` | E-Stop-Button disabled wenn SAFE_MODE; Deadman-Button visible wenn CONNECTED |
| `ConnectionPanel.test.tsx` | Latenz-Farbe: grün <50ms, gelb <100ms, rot ≥100ms; Session-ID Truncation |
| `SafeModeOverlay.test.tsx` | Overlay rendered wenn SAFE_MODE; Resume-Button ruft onResume auf |
| `ControlPanel.test.tsx` | Panel disabled wenn !enabled (opacity-40 + pointer-events-none) |
| `VideoPanel.test.tsx` | MEDIA_FAILED Overlay zeigt Retry-Button; MEDIA_CONNECTED zeigt kein Overlay |

**Playwright E2E — Baseline:**
```typescript
// tests/e2e/dashboard.spec.ts
test('Dashboard lädt und zeigt SYSTEM STATE', async ({ page }) => {
  await page.goto('http://localhost:3000')
  await expect(page.locator('text=AVOC')).toBeVisible()
  await expect(page.locator('text=IDLE')).toBeVisible()
})
```

---

### TEST-05 — Latenz-Tests CI (<100ms Build-Fail)

#### Go Benchmark

**`tests/performance/latency_test.go`:**
```go
// BenchmarkControlACKRoundtrip misst den ACK-Roundtrip für einen Protobuf ControlCommand.
// CI Build-Fail wenn p99 > 100ms (ADR-006/010).
func BenchmarkControlACKRoundtrip(b *testing.B) {
    // Verbindet zu laufendem Control Server (TEST-03 Stack vorausgesetzt)
    // Sendet DEADMAN_HOLD → misst Zeit bis ACK
    // b.ReportMetric(p99Ms, "p99_ms")
}
```

**`Makefile`-Target:**
```makefile
test-latency: ## Latenz-Tests gegen laufenden Stack (Build-Fail bei >100ms p99)
    docker compose -f tests/docker-compose.test.yml up -d
    sleep 3
    go test ./tests/performance/... -bench=. -benchtime=10s -run=^$$ | \
      tee /tmp/bench_out.txt
    @grep p99_ms /tmp/bench_out.txt | \
      awk '{ if ($$NF > 100) { print "FAIL: p99=" $$NF "ms > 100ms Budget"; exit 1 } \
             else { print "PASS: p99=" $$NF "ms" } }'
```

#### k6 Load Test

**`tests/performance/latency.js`:**
```javascript
// k6-Script: 100 VU, 30s, ACK-Roundtrip via WebSocket
// Threshold: p(99) < 100ms
export const options = {
  vus: 100,
  duration: '30s',
  thresholds: { 'ws_session_duration': ['p(99)<100'] },
}
```

**`Makefile`-Target:**
```makefile
test-k6:
    docker compose -f tests/docker-compose.test.yml up -d
    sleep 3
    k6 run tests/performance/latency.js
```

**Hinweis:** k6 muss lokal installiert sein oder via Docker Image laufen:
```bash
docker run --rm -i grafana/k6 run - < tests/performance/latency.js
```

---

### DC-04 — README: Troubleshooting + Contributor Guide

**Abschnitte die fehlen:**

#### Troubleshooting

```markdown
## Troubleshooting

### 502 Bad Gateway nach Container-Rebuild
nginx cached Docker-IPs beim Start. Fix:
  docker exec avoc-frontend-1 nginx -s reload

### npm run dev: cannot find @/gen/control_pb.js
Proto-Dateien fehlen. Fix (einmalig nach git clone):
  make proto-gen-ts
Ursache: src/gen/ ist gitignored — wird build-time generiert.

### npm run dev: @rollup/rollup-linux-x64-gnu fehlt
node_modules wurde in Docker (Alpine/musl) installiert. Fix:
  # Mit Docker löschen (root-owned):
  docker run --rm -v $(PWD)/frontend:/app -w /app node:22-alpine sh -c 'rm -rf node_modules package-lock.json'
  cd frontend && npm install

### Port-Konflikte
  lsof -i :3000   # Frontend
  lsof -i :8080   # Control Server
  lsof -i :8084   # WebRTC SFU

### WSL2: Services nicht erreichbar
Hostname in vite.config.ts und .env auf WSL2-IP setzen:
  hostname -I | awk '{print $1}'
```

#### Contributor Guide

```markdown
## Contributor Guide

### Neue ADR erstellen
1. Kopiere docs/adr/000-template.md → docs/adr/0XX-titel.md
2. Fülle alle Pflichtfelder aus (Kontext, Optionen, Entscheidung, Konsequenzen)
3. Trage ADR in DECISIONS.MD + docs/adr/README.md ein
4. Aktualisiere implementation-plan.md ADR-Index

### Neuen Go-Service hinzufügen
1. Erstelle cmd/<service-name>/main.go
2. Erstelle infrastructure/docker/go-service.Dockerfile (wiederverwendbar)
3. Ergänze Service in docker-compose.yml
4. Füge /health Endpoint hinzu

### Proto-Schema ändern
→ Field-based Versioning (ADR-012): keine Field-IDs ändern, keine Felder entfernen
1. Ändere proto/*.proto
2. make proto-gen      # Go
3. make proto-gen-ts   # TypeScript
4. gen/ ist gitignored — nie committen
```

---

## Sprint-Ziel / Definition of Done

- [ ] `tests/docker-compose.test.yml` startet minimal Stack (control-server, auth, safety, mosquitto)
- [ ] `make test-integration` — alle Services healthy, Go Integration Tests grün
- [ ] `npm run test` — alle Vitest Component-Tests grün (≥5 Test-Files, SafetyPanel, ConnectionPanel, SafeModeOverlay, ControlPanel, VideoPanel)
- [ ] Playwright E2E: Dashboard öffnet, AVOC-Header sichtbar, IDLE-State angezeigt
- [ ] Go Benchmark (`tests/performance/`) misst ACK-Roundtrip
- [ ] k6 Latenz-Test dokumentiert: 100 VU, 30s, p99 < 100ms gegen laufenden Stack
- [ ] `make test-latency` schlägt fehl wenn p99 > 100ms (Build-Fail dokumentiert)
- [ ] README Troubleshooting: 502-Fix, proto-gen-ts, rollup-Fix, Port-Konflikte, WSL2
- [ ] README Contributor Guide: ADR-Prozess, neuer Service, Proto-Änderungen
- [ ] Safety Regression: weiterhin 19/19 grün
- [ ] `docker-compose up --build` — alle Services healthy nach Sprint-6-Änderungen
