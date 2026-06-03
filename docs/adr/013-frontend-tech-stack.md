# ADR-013: Frontend Technology Stack

Status: Accepted

## Kontext

Das Frontend war als React (JavaScript) definiert (CONTEXT.MD, architecture.md). ADR-012b ließ die Frage offen, ob TypeScript für Protobuf Code-Gen eingeführt wird. Zusätzlich waren Build-Tool, Styling und Component Library nicht entschieden. Diese Entscheidung legt den vollständigen Frontend-Stack fest und löst die offene TypeScript-Frage aus ADR-012b.

## Optionen

### Option A: React + JavaScript + CRA (bisheriger Stand)

## Vorteile:
- Kein Breaking Change zur bisherigen Vorgabe

## Nachteile:
- Kein Type-Safety für Protobuf Code-Gen
- CRA ist deprecated
- Kein einheitliches Component System

### Option B: React + TypeScript + Vite + Tailwind CSS + Shadcn/ui

## Vorteile:
- Vollständige TypeScript-Integration für Protobuf Code-Gen (protoc-gen-es)
- Vite: deutlich schnellerer Build als CRA, modernes Tooling
- Tailwind CSS: utility-first, konsistentes Styling ohne CSS-Overhead
- Shadcn/ui: zugängliche, anpassbare React-Komponenten auf Tailwind-Basis
- Type-Safety über gesamten Frontend-Code
- Bessere IDE-Unterstützung und Refactoring-Sicherheit

## Nachteile:
- Erfordert TypeScript-Kenntnisse
- Shadcn/ui generiert Code ins Projekt (kein klassisches npm-Package)

## Entscheidung

Wir wählen **Option B: React + TypeScript + Vite + Tailwind CSS + Shadcn/ui**.

## Begründung

TypeScript ist für die Protobuf Code-Gen-Strategie (ADR-012b) notwendig — statische Klassen aus `.proto`-Dateien erfordern TypeScript für volle Type-Safety. Vite ersetzt das veraltete CRA und bietet erheblich schnellere Build-Zeiten. Tailwind CSS und Shadcn/ui bilden ein konsistentes, wartbares UI-System, das sich gut für das modulare Dashboard-Design (Video Panel, Safety Panel, Control Panel) eignet. Die Kombination ist aktuell Best Practice für neue React-Projekte.

## Vollständiger Frontend-Stack

| Bereich | Technologie |
|---------|------------|
| Framework | React 18+ |
| Sprache | TypeScript |
| Build Tool | Vite |
| Styling | Tailwind CSS |
| Component Library | Shadcn/ui |
| Protobuf Code-Gen | protoc-gen-es (TypeScript) |
| Test Framework | Jest + React Testing Library + Playwright (ADR-006) |

## Löst offene Fragen

- ADR-012b: TypeScript im Frontend → **Ja, TypeScript**
- ADR-012b: protoc-Plugin → **protoc-gen-es** (TypeScript-Klassen)

## Konsequenzen

### Positiv:
- Vollständige Type-Safety über Frontend-Code und Protobuf-Messages
- Schnelle Build- und HMR-Zeiten durch Vite
- Konsistentes UI-System durch Shadcn/ui + Tailwind
- Shadcn/ui-Komponenten sind direkt anpassbar (kein Black-Box-Package)

### Negativ:
- CONTEXT.MD muss aktualisiert werden (JavaScript → TypeScript)
- architecture.md muss aktualisiert werden
- FE-01 (React Setup) hat jetzt konkrete Tech-Vorgaben
- CI benötigt protoc-gen-es als Build-Dependency
