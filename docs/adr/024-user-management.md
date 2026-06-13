# ADR-024: Nutzerverwaltung mit ADMIN-Rolle und bcrypt-Authentifizierung

**Status:** Akzeptiert
**Datum:** 2026-06-14
**Autoren:** Florian Steinmann

---

## Kontext

Der Auth-Service (ADR-004) gibt seit Sprint 1 Tokens aus ohne Credentials zu prüfen
("accept any credentials, return OBSERVER role"). Operator-ID und Passwort sind im Frontend hardcoded
(`operator-1`, `test`). Es gibt kein Konzept echter Benutzerkonten, keine Passwortverwaltung,
keine Möglichkeit Operatoren zu sperren oder Rollen zu steuern.

Mit der PostgreSQL-Migration (ADR-023) steht eine gemeinsame Datenbank zur Verfügung, in der
Benutzerkonten persistent gespeichert werden können.

## Entscheidung

### 1. UserStore in PostgreSQL

Neue Tabelle `users` im Auth-Service (erstellt beim Start via `CREATE TABLE IF NOT EXISTS`):

```sql
CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,          -- Operator-ID (z.B. "admin", "op-florian")
    display_name  TEXT NOT NULL,
    password_hash TEXT NOT NULL,             -- bcrypt, cost=12
    role          TEXT NOT NULL DEFAULT 'OBSERVER',
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_auth_at  TIMESTAMPTZ
);
```

### 2. Neue ADMIN-Rolle

```go
const (
    RoleAdmin          OperatorRole = "ADMIN"            // Neu
    RoleActiveOperator OperatorRole = "ACTIVE_OPERATOR"
    RoleObserver       OperatorRole = "OBSERVER"
    RoleStandby        OperatorRole = "STANDBY"
    RoleVehicle        OperatorRole = "VEHICLE"
)
```

ADMIN hat dieselben Steuerrechte wie ACTIVE_OPERATOR, zusätzlich Zugriff auf `GET/POST/DELETE/PATCH /auth/users`.
Die ADMIN-Rolle wird im JWT-Claim `role` übertragen — Control-Server und Frontend lesen sie aus dem Token.

### 3. bcrypt mit cost=12

Passwörter werden ausschließlich als bcrypt-Hash gespeichert (cost=12, ~300ms auf moderner Hardware).
Klartext-Passwörter verlassen niemals die Applikation. `golang.org/x/crypto/bcrypt` wird als Dependency
hinzugefügt.

### 4. Auto-Seed Admin

Beim Start des Auth-Service wird idempotent ein Admin-User angelegt:
```go
userStore.SeedAdmin(ctx, "admin", os.Getenv("ADMIN_PASSWORD"))
// → INSERT INTO users ... ON CONFLICT (id) DO NOTHING
```
- **Dev:** `ADMIN_PASSWORD=admin_dev_secret` in `docker-compose.yml` (env-Default)
- **Prod:** `ADMIN_PASSWORD` aus AWS SSM Parameter Store (`/avoc/prod/admin-password`)

### 5. Echte Authentifizierung

`POST /auth/operator/login` prüft ab sofort:
1. User mit angegebener ID in DB vorhanden und `is_active = TRUE`?
2. `bcrypt.CompareHashAndPassword(hash, password)` erfolgreich?
3. `UPDATE users SET last_auth_at = NOW() WHERE id = $1`
4. JWT mit tatsächlicher Rolle aus DB ausstellen (nicht hardcoded OBSERVER)

### 6. User-Management-Endpoints (nur ADMIN)

| Method | Path | Beschreibung |
|--------|------|-------------|
| `GET /auth/users` | — | Alle Nutzer (ohne password_hash) |
| `POST /auth/users` | `{id, display_name, password, role}` | Nutzer anlegen |
| `DELETE /auth/users/{id}` | — | Nutzer löschen (nicht eigenen Account) |
| `PATCH /auth/users/{id}` | `{role}` | Rolle ändern |

Alle Endpoints geschützt durch `requireAdmin` Middleware (prüft JWT-Claim `role == "ADMIN"`).

### 7. Frontend-Änderungen

- **LoginPanel** ersetzt Auto-Connect: Vollbild-Overlay mit ID + Passwort-Feldern
- **UserManagementPanel**: Tabelle + Formular (nur sichtbar bei ADMIN-Token)
- `parseTokenRole(token)`: Base64-Dekodierung des JWT-Payloads, kein externes Package
- `useSession.connect(id, password)` statt `connect()` ohne Parameter

## Konsequenzen

**Positiv:**
- Echte Zugriffskontrolle — Operator ohne Account kann das System nicht nutzen
- Passwörter kryptografisch sicher gespeichert (bcrypt, nicht reversibel)
- ADMIN-Rolle ermöglicht zentrale Nutzerverwaltung im laufenden Betrieb
- `last_auth_at` ermöglicht Audit ("wer hat sich wann eingeloggt")
- Vorbereitung für spätere Funktionen: Account-Sperre, Passwort-Reset, MFA

**Negativ / Risiken:**
- Login-Overhead: bcrypt cost=12 ~300ms pro Login (akzeptabel — selten, kein Hot-Path)
- `ADMIN_PASSWORD` muss bei erstem Deployment gesetzt sein (sonst kein Login möglich)
- Bestehende hardcodierte `operator-1`-Sessions werden ungültig — einmaliger Break bei Migration

## Abgrenzung zu ADR-004

ADR-004 entschied: "Separater Auth-Service für JWT-Ausstellung". Diese Entscheidung bleibt.
ADR-024 konkretisiert die **Implementierung** des Auth-Service mit echten Credentials,
ohne die Architektur (separater Service, JWT-Basis) zu ändern.

## Alternativen verworfen

- **Argon2id statt bcrypt:** Sicherer gegen GPU-Angriffe, aber weniger portabel und für diesen Anwendungsfall (kein Public-facing Auth) overkill.
- **Externe Auth (OAuth/OIDC):** Zu komplex für die aktuelle Betriebsumgebung; single-operator Teleoperation-System ohne SSO-Anforderung.
- **Passwörter in .env statt DB:** Kein Runtime-Management möglich, kein CRUD, keine Rollentrennung.
