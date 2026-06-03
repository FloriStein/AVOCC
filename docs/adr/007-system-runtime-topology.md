# ADR-007: System Runtime Topology

Status: Accepted

## Kontext

Das System besteht aus mehreren Services (Control Server, Auth Service, Safety Event Bus, MQTT Telemetry, Video Stream, Frontend). Nicht definiert war, wie diese Services miteinander kommunizieren und wer für Routing, Safety Enforcement und Entscheidungen zuständig ist. Ohne klare Topologie entstehen Race Conditions, unkontrollierte Systemzustände und schwer debuggbare Fehler in einem Safety-kritischen Echtzeitsystem.

## Optionen

### Option A: Hub-and-Spoke (Control Server als zentraler Orchestrator)

## Vorteile:
- Deterministischer Kontrollfluss
- Klares Single Source of Truth für Safety-Entscheidungen
- Einfacheres Debugging und System-Replay
- Safety Enforcement an einer einzigen Stelle

## Nachteile:
- Control Server ist Single Point of Failure
- Potenzieller Bottleneck unter hoher Last

### Option B: Peer-to-Peer Mesh

## Vorteile:
- Kein Single Point of Failure
- Theoretisch geringere Latenz

## Nachteile:
- Hohe Komplexität bei Race Conditions
- Safety Enforcement schwer kontrollierbar
- Inkonsistente Systemzustände wahrscheinlich

## Präzisierung: Control Hub ≠ Data Hub

ADR-007 beschreibt einen **Control Hub** — nicht einen zentralen Datendurchlaufpunkt für alle Bytes. Der Control Server ist Single Source of Truth für Steuerungsflüsse, Safety-Entscheidungen und State Transitions. Video-Daten (Media Layer) müssen nicht durch den Control Server fließen.

| Kanal | Durch Control Server? | Begründung |
|-------|----------------------|-----------|
| Control Commands | ✅ Ja | Safety-kritisch |
| Safety Events | ✅ Ja | Hub-and-Spoke |
| MQTT Telemetry | ✅ Ja | Status-Routing |
| **Video (Media)** | ❌ Nein | Media Layer ist orthogonal |

Der **WebRTC SFU** (ADR-014) ist ein zweiter, orthogonaler Hub ausschließlich für den Video-Kanal — kein Konflikt mit dem Control Hub.

## Entscheidung

Wir wählen **Option A: Hub-and-Spoke** mit dem Control Server als zentralem Orchestrator für Control, Safety und Telemetry. Der Video-Kanal hat seinen eigenen Hub (WebRTC SFU, ADR-014).

## Begründung

Deterministische Kontrolle und klares Safety Enforcement sind für ein Teleoperation-System wichtiger als theoretische Latenzvorteile eines Mesh. Video ist Awareness-Kanal, kein Safety-Kanal — er benötigt einen eigenen, spezialisierten Hub (SFU) statt den Control Hub zu belasten.

## Architektur

```
                    ┌─────────────────────┐
                    │    Control Server   │
                    │  (Central Hub)      │
                    │                     │
                    │ · WebSocket Plane   │
                    │ · Command Routing   │
                    │ · Safety Enforce.   │
                    └──────────┬──────────┘
                               │
          ┌────────────────────┼────────────────────┐
          │                    │                    │
   ┌──────▼──────┐    ┌────────▼──────┐    ┌───────▼───────┐
   │ Auth Service│    │ Safety Event  │    │MQTT Telemetry │
   │             │    │ Bus (Mock DDS)│    │ (Mosquitto)   │
   └─────────────┘    └───────────────┘    └───────────────┘
          │
   ┌──────▼──────┐    ┌───────────────┐
   │Video Stream │    │   Frontend    │
   │  Service    │    │  (React SPA)  │
   └─────────────┘    └───────────────┘
```

## Hub-Hierarchie (CONTROL HUB > VIDEO HUB)

Die zwei Hubs sind nicht gleichwertig — es gilt eine strikte Hierarchie:

```
CONTROL HUB (Rang 1 — Safety Truth)
  · System State Machine
  · Safety Decision Engine
  · Session Management (Single Source of Truth)
  · Operator Handover

VIDEO HUB (Rang 2 — Awareness only)
  · Media Relay (Primary + Secondary Streams)
  · Server-seitiges Recording
  · Multi-Operator Forwarding
```

**Hierarchie-Regeln:**

1. Der Video Hub darf den System State **niemals** beeinflussen — außer durch eine DEGRADED-Annotation, die der Control Hub eigenständig auswertet.
2. Bei Ressourcenkonflikten oder Zustandsinkonsistenzen hat der Control Hub immer Vorrang.
3. Der Control Hub ist der **Single Source of Truth für Session Context** — der Video Hub fragt Sessionzustand beim Control Hub ab, nicht umgekehrt.
4. Wenn Control Hub und Video Hub widersprüchliche Zustände anzeigen (z.B. Control CONNECTED, SFU DISCONNECTED), gilt immer der Control Hub-Zustand als autoritativ.

**System Invariante:**

```
INVARIANT: Media Layer SHALL NOT influence SAFE_MODE transitions
           except via DEGRADED annotation evaluated by the Control Hub.
```

## Konsequenzen

### Positiv:
- Klare Kontrolle über alle Systemflüsse
- Safety Enforcement deterministisch an einem Punkt
- Control Hub als expliziter Single Source of Truth für Session Context
- Kein Zustandsdrift zwischen Control Hub und Video Hub möglich

### Negativ:
- Control Server ist kritische Komponente — hohe Reliability erforderlich
- Single Point of Failure muss durch spätere Deployment-Strategie mitigiert werden (eigenes ADR)
- Video Hub muss Session Context aktiv vom Control Hub beziehen
