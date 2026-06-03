# ADR-006: Testing Strategy

Status: Accepted

## Kontext

Das Teleoperation-System ist ein verteiltes Echtzeit-System mit Safety-kritischen Komponenten (Emergency Stop, Dead-man Switch, Auto-Stop) und einem harten Latenz-Ziel von <100ms im Control Loop. Klassische Unit-Test-Strategien allein sind nicht ausreichend, da viele Fehler durch Netzwerk- und Timing-Verhalten entstehen. Eine explizite, mehrschichtige Test-Strategie ist notwendig, bevor die Implementierung beginnt.

---

## Entscheidungen

### Teil 1 — Go Backend: Test-Framework

**Gewählt: `testing` (Standard) + `testify`**

#### Vorteile:
- Expressive, lesbare Assertions
- Mocking via `testify/mock` für komplexe Szenarien (Netzwerkfehler, Safety Events)
- Weit verbreitet im Go-Ecosystem
- Guter Kompromiss aus Einfachheit und Funktionalität

#### Nachteile:
- Externe Dependency (`testify`)
- Mocking-Konzept für externe Services muss eingeführt werden

#### Verworfene Alternative:
- Nur Standard `testing`: zu verbose für komplexe Systemtests

---

### Teil 2 — Integrationstests: Testinfrastruktur

**Gewählt: Hybrid — Docker-basierte Integration Tests + In-Memory Mocks für Unit Tests**

#### Unit Tests:
- Verwenden In-Memory Mocks / Fake Implementierungen
- Schnelle, isolierte Logiktests ohne Netzwerk

#### Integration Tests (primär):
- Echte Services via Docker (Mosquitto, WebSocket Server, Safety Mock Service)
- Tests gegen reale Netzwerkbedingungen
- Fokus auf Inter-Service-Kommunikation

#### Vorteile:
- Erkennt echte Integrationsprobleme, die Mocks nicht abbilden können
- Realistische Systemvalidierung für ein Echtzeitsystem

#### Nachteile:
- CI benötigt Docker-Environment
- Höherer Setup-Aufwand

#### Verworfene Alternative:
- In-Memory Mocks only: nicht realitätsnah genug für Teleoperation

---

### Teil 3 — Safety Tests: Dedizierte Safety Test Suite

**Gewählt: Separate Safety Test Suite (`safety_test.go`)**

#### Abgedeckte Szenarien:
- Dead-man Switch Verhalten unter kontinuierlichem Input
- Emergency Stop Priorisierung über alle anderen Commands
- Auto-Stop bei WebSocket / MQTT Disconnect
- Race Conditions zwischen Control Layer und Safety Layer
- Timeout- und Latency-basierte Trigger

#### Vorteile:
- Klare, dokumentierte Safety-Szenarien
- Höhere Reviewbarkeit und Audit-Fähigkeit
- Safety-Verhalten wird als explizite Systemanforderung behandelt

#### Nachteile:
- Einführung eines separaten Testlayers

#### Verworfene Alternative:
- Normale Unit Tests gegen Safety Event Bus Interface: keine explizite Szenario-Struktur

---

### Teil 4 — Frontend Testing: Hybrid-Strategie

**Gewählt: Jest + React Testing Library (Unit/Component) + Playwright (E2E)**

#### Unit & Component Tests (Jest + RTL):
- Isolierte UI-Komponenten
- Button States, Inputs, Rendering
- Schnelle CI-Ausführung

#### E2E Tests (Playwright):
- Vollständige Browser-Sessions
- WebSocket-Kommunikation unter realen Bedingungen
- Emergency Stop / Dead-man Switch Verhalten im Browser
- State-Synchronisation zwischen Backend und UI

#### Vorteile:
- Optimale Balance zwischen Testgeschwindigkeit und Realismus
- Safety-relevante UI-Interaktionen werden unter realen Bedingungen validiert

#### Nachteile:
- CI-Pipeline wird zweistufig (Unit + E2E)
- E2E-Testumgebung erfordert Docker-Setup

#### Verworfene Alternativen:
- Nur Jest + RTL: kein Real-Time Testing möglich
- Nur E2E: zu langsam und teuer für CI

---

### Teil 5 — Latenz-Tests: CI-basierte Performance Validierung

**Gewählt: Verpflichtende Latenz-Tests in der CI/CD Pipeline**

#### Messstrategie:
- End-to-End Control Message Roundtrip
- Durchschnittliche Latenz, P95, P99, Worst-case Spikes

#### Tools:
- Go Benchmark Tests für Service-interne Messungen
- k6 für Load- und Latency-Simulation
- Custom Telemetry Measurement Layer

#### Fail-Kriterium:
- <100ms Verletzung = Build fail

#### Vorteile:
- Automatische Validierung der Echtzeitfähigkeit
- Regression Detection bei Code- oder Infrastrukturänderungen
- Notwendig für Safety-kritische Systeme

#### Nachteile:
- Einführung eines Performance-Test Layers in CI
- Latenz-Budgets pro Service müssen definiert werden

#### Verworfene Alternative:
- Manuelle Messungen: nicht reproduzierbar, keine Regressionserkennung

---

## Test-Architektur Übersicht

```
┌─────────────────────────────────────────────────────┐
│                   CI/CD Pipeline                    │
├─────────────┬──────────────┬────────────────────────┤
│  Unit Tests │ Integration  │    E2E / Performance   │
│             │    Tests     │                        │
│  Go testing │ Docker-based │  Playwright (Frontend) │
│  + testify  │ (Mosquitto,  │  k6 / Go Benchmarks    │
│  In-Memory  │  WS, Safety) │  (<100ms enforcement)  │
│  Mocks      │              │                        │
├─────────────┴──────────────┴────────────────────────┤
│          Dedicated Safety Test Suite                │
│   (safety_test.go — szenariobasiert, audit-fähig)  │
└─────────────────────────────────────────────────────┘
```

## WebRTC CI Non-Determinism Policy

WebRTC E2E Tests sind strukturell nicht-deterministisch durch ICE-Aushandlung, NAT-Traversal-Timing und Browser-Rendering-Timing. Diese Tests werden daher explizit anders behandelt als Safety oder Control Tests.

**Regeln für WebRTC E2E Tests:**

1. **Retry erlaubt:** WebRTC E2E Tests dürfen bis zu 3× wiederholt werden, bevor sie als fehlgeschlagen gelten.
2. **Threshold-based:** Erfolg wird über Prozentwerte definiert (z.B. ≥ 80% der ICE-Verbindungen müssen innerhalb von 5s stehen).
3. **Non-blocking:** WebRTC E2E Tests blockieren den Main CI Pipeline-Build nicht — sie laufen als separater CI-Job.
4. **Kein Safety Gate:** WebRTC E2E Tests sind kein Blocking-Gate für Deployments. Safety Tests und Latenz-Tests sind die einzigen harten Gates.
5. **Flakiness-Tracking:** Flaky WebRTC Tests werden getracked (nicht ignoriert), aber nicht als Blocker behandelt.

```
CI Pipeline:
  ├── [BLOCKING]  Unit Tests (Go + Frontend)
  ├── [BLOCKING]  Safety Test Suite
  ├── [BLOCKING]  Latency Tests (<100ms)
  ├── [BLOCKING]  Integration Tests (Docker)
  └── [NON-BLOCKING, RETRY 3×]  WebRTC E2E Tests (Playwright)
```

## Konsequenzen

### Positiv:
- Mehrschichtige Absicherung für ein Safety-kritisches Echtzeitsystem
- Safety-Szenarien explizit dokumentiert und reviewbar
- Latenz-Ziel (<100ms) automatisch verifiziert
- Mock-Drift durch Docker-basierte Integrationstests vermieden
- WebRTC CI-Flakiness ist explizit modelliert und kein unerwarteter Blocker

### Negativ:
- CI-Umgebung benötigt Docker + STUN/TURN (coturn)
- Erhöhter initialer Setupaufwand für Test-Infrastruktur
- Latenz-Budgets pro Service müssen noch definiert werden (Folge-Task)
- WebRTC E2E Tests können ICE-Race-Conditions erzeugen — Retry-Logik ist Pflicht
