# ADR-002: Safety Channel — Simulated Safety Event Bus statt DDS

Status: Accepted

## Kontext

Die ursprüngliche Architektur sah DDS (Data Distribution Service) als Safety-kritischen Kommunikationskanal vor, da DDS deterministische Nachrichtenzustellung und Fault-Tolerance bietet. DDS-Implementierungen sind jedoch komplex in der Einrichtung, erhöhen die Infrastrukturabhängigkeit und verlangsamen die initiale Entwicklung erheblich. Das Projekt befindet sich in einer frühen Phase, in der die Safety-Semantik wichtiger ist als die konkrete Transport-Technologie.

## Optionen

### Option A: DDS-Implementierung (CycloneDDS / FastDDS)

## Vorteile:
- Produktionsreif für Safety-kritische Systeme
- Deterministische Nachrichtenzustellung
- Fault-tolerantes Design

## Nachteile:
- Hohe Setup-Komplexität in Docker
- Lange Einarbeitungszeit
- Blockiert frühe Entwicklungsiterationen

### Option B: Simulated Safety Event Bus mit DDS-kompatiblem Interface

## Vorteile:
- Sofort entwickelbar und testbar
- Klares Interface ermöglicht späteren DDS-Austausch ohne Änderungen an Consumern
- Deterministisches Verhalten simulierbar über interne Message-Queue
- Passt zu Docker-Compose-first-Ansatz

## Nachteile:
- Nicht produktionstauglich für reale Safety-kritische Hardware
- Simulation kann echte DDS-Fehlerszenarien nicht vollständig abbilden

## Entscheidung

Wir wählen **Option B: Simulated Safety Event Bus** mit einem klar definierten, DDS-kompatiblen Interface.

## Begründung

DDS wird in dieser Projektphase nicht implementiert. Stattdessen wird ein dedizierter Go-Service ("Safety Event Bus") eingeführt, der Safety-Events über ein abstraktes Interface verarbeitet. Das Interface ist so gestaltet, dass es später 1:1 durch eine echte DDS-Implementierung ersetzt werden kann (Strangler-Fig-Prinzip), ohne dass Consumer-Services angepasst werden müssen.

## Architektur

Der Safety Event Bus stellt folgendes Interface bereit:

- `PublishSafetyEvent(event SafetyEvent)`
- `SubscribeSafetyEvents(handler SafetyEventHandler)`
- `TriggerEmergencyStop(reason string)`
- `GetSafetyState() SafetyState`

Intern arbeitet die erste Implementierung mit einer in-memory Message-Queue und deterministischer Event-Simulation.

## Konsequenzen

### Positiv:
- Sofortige Entwickelbarkeit ohne DDS-Infrastruktur
- Saubere Interface-Grenze für späteren DDS-Austausch
- Vollständig in Docker Compose integrierbar

### Negativ:
- Kein reales DDS-Verhalten in dieser Phase
- Späterer DDS-Austausch erfordert neues ADR und Integrationstest
