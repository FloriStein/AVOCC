# ADR-004: Authentifizierung — Separater Auth Service

Status: Accepted

## Kontext

Das System erfordert JWT-basierte Authentifizierung für WebSocket-Verbindungen (Control Channel). Es müssen zwei Arten von Identitäten authentifiziert werden: Operatoren (menschliche Nutzer) und Fahrzeuge (Clients des Systems). Eine spätere Erweiterung auf OAuth 2.0, IAM oder Fleet Management muss architektonisch möglich sein. Die Frage ist, ob JWT-Ausstellung im Control Server integriert wird oder in einen dedizierten Service ausgelagert wird.

## Optionen

### Option A: JWT-Ausstellung im Control Server integriert

## Vorteile:
- Weniger Services, einfacherer initialer Aufbau
- Keine zusätzliche Netzwerkkommunikation für Token-Validierung

## Nachteile:
- Schlechte Separation of Concerns (Control Server = Steuerung + Auth)
- Schwer erweiterbar für Fahrzeug-Auth und OAuth
- Security Boundary unklar
- Koppelt Auth-Logik an Control-Logik

### Option B: Separater Auth Service

## Vorteile:
- Klare Trennung von Verantwortlichkeiten
- Unterstützt Operator Auth und Vehicle Auth unabhängig
- Erweiterbar auf OAuth 2.0 / IAM / Fleet Management
- Saubere Security Boundary
- Ermöglicht zentrales Token-Management über alle Services

## Nachteile:
- Zusätzlicher Container im Docker-Compose-Stack
- Höherer initialer Implementierungsaufwand

## Entscheidung

Wir wählen **Option B: Separater Auth Service** als eigenständiger Go-Service.

## Begründung

Das System authentifiziert zwei Arten von Identitäten (Operatoren und Fahrzeuge), die unterschiedliche Berechtigungen und Lebensdauern haben. Eine spätere Integration von OAuth 2.0 oder Fleet Management ist realistisch. Die Zusammenführung von Auth-Logik und Control-Logik würde beide Domänen koppeln und die Security-Grenze verwischen. Ein dedizierter Auth Service schafft eine klare, erweiterbare Grundlage.

## Architektur

Der Auth Service stellt bereit:

- `POST /auth/operator/login` → JWT für Operator (mit Rolle: Active/Observer/Standby)
- `POST /auth/vehicle/register` → JWT für Fahrzeug
- `POST /auth/token/validate` → Token-Validierung (intern, via shared Public Key)
- `POST /auth/token/refresh` → Token-Erneuerung
- `POST /auth/handover/token` → Handover-Token für Operator-Übergabe (ADR-015)

Alle anderen Services validieren JWTs **lokal** gegen den shared Public Key (async — ADR-012b). Der Auth Service wird nur für Issuance und Revocation kontaktiert.

## Operator-Rollen (ADR-011/015)

| Rolle | Berechtigung |
|-------|-------------|
| `ACTIVE_OPERATOR` | Vollständige Steuerungsberechtigung — exklusiv pro Control Session |
| `OBSERVER` | Lese-/Video-Zugriff, keine Steuerung |
| `STANDBY` | Wartend, kann via Handover zu ACTIVE werden |

Rollen werden im JWT-Payload übertragen. Rollenwechsel erfordert neues JWT (Handover-Token).

## JWT vs. Session-ID (ADR-016)

**JWT = Identity:** Wer bist du? (Operator-ID, Rolle, Expiry)
**Session-ID (ULID) = Execution Context:** In welcher aktiven Control Session bist du?

JWTs enthalten **keine Session-ID**. Session-IDs werden ausschließlich vom Control Server (GSA) generiert.

## Konsequenzen

### Positiv:
- Klare Security Boundary zwischen Auth und Business Logic
- Operator und Fahrzeug unabhängig authentifiziert und autorisiert
- Lokale JWT-Validierung vermeidet Auth-Bottleneck im Control Loop
- OAuth 2.0 / IAM-Anbindung ohne Änderung an anderen Services möglich

### Negativ:
- Auth Service ist Single Point of Failure für neue Sessions (laufende Sessions funktionieren via lokalem Public Key weiter)
- Handover-Token-Logik erhöht Auth-Service-Komplexität leicht
