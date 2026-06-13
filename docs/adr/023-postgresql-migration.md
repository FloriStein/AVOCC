# ADR-023: PostgreSQL als primäre Datenbank (SQLite-Migration)

**Status:** Akzeptiert
**Datum:** 2026-06-14
**Autoren:** Florian Steinmann

---

## Kontext

Das System nutzt seit Sprint 7 (ADR-018) SQLite (`modernc.org/sqlite`, pure Go) als AuditWriter und seit
Sprint 12 (ADR-022) als VehicleStore. Beide Stores teilen eine gemeinsame `*sql.DB`-Connection via
`DB()`-Getter des SQLiteAuditWriters. SQLite ist für den aktuellen Anwendungsfall geeignet, stößt aber
an Grenzen sobald:

- Die Auth-Service-Datenbank (Nutzerverwaltung, ADR-024) in einem separaten Service läuft
- Mehrere Services unabhängig auf die Datenbank zugreifen sollen
- Echter Connection-Pool (nicht `SetMaxOpenConns(1)`) gewünscht wird
- Cloud-Deployment einfache Backup/Restore-Workflows erfordert

## Entscheidung

**PostgreSQL 16 ersetzt SQLite vollständig** als einzige Datenbankinfrastruktur.

Alle Tabellen (`audit_events`, `vehicles`, neu: `users`) liegen in einer gemeinsamen PostgreSQL-Datenbank
(`avoc`). Auth-Service und Control-Server öffnen je eine eigene `*sql.DB`-Verbindung (eigene Connection-Pools,
kein Service-übergreifendes Sharing).

## Durabilität: WriteSync() ohne wal_checkpoint

ADR-018 fordert: SAFE_MODE-Transition erst nach garantierter Persistenz des Safety-Events
(`AuditWriter.WriteSync()`). Bisher: SQLite WAL + `PRAGMA wal_checkpoint(FULL)` + `PRAGMA synchronous=FULL`.

PostgreSQL mit `synchronous_commit = on` (Default) garantiert: WAL-Flush auf Disk **vor dem COMMIT-Return**.
Ein `INSERT` ohne explizite Transaktion läuft im Autocommit-Modus — COMMIT = fsync auf PostgreSQL-WAL.
Dies ist semantisch äquivalent zu `wal_checkpoint(FULL)`. Kein explizites `PRAGMA` nötig.

**Invariante bleibt erhalten:** `WriteSync()` blockiert bis zum PostgreSQL-COMMIT, der WAL-durabel ist.

## SQL-Dialekt-Anpassungen

| SQLite | PostgreSQL |
|--------|-----------|
| `?` Platzhalter | `$1`, `$2`, ... `$N` |
| `INTEGER PRIMARY KEY AUTOINCREMENT` | `SERIAL PRIMARY KEY` |
| `INSERT OR IGNORE` | `INSERT ... ON CONFLICT (col) DO NOTHING` |
| `PRAGMA journal_mode=WAL` | entfällt (PG native WAL) |
| `PRAGMA synchronous=FULL` | entfällt (PG `synchronous_commit=on` Default) |
| `PRAGMA wal_checkpoint(FULL)` | entfällt (PG COMMIT = durabel) |
| `db.SetMaxOpenConns(1)` | entfällt — PG unterstützt concurrent writers |
| Timestamps: TEXT RFC3339 | Timestamps: TEXT RFC3339 (unverändert, kompatibel) |

## Infrastruktur

- **Dev:** `postgres:16-alpine` Container in `docker-compose.yml`, `DB_PASSWORD=avoc_dev_secret` Default
- **Prod:** selbes Image in `docker-compose.prod.yml`, `DB_PASSWORD` aus AWS SSM Parameter Store
- **Healthcheck:** `pg_isready -U avoc -d avoc` — Control-Server + Auth-Service starten erst wenn `healthy`
- **Persistence:** `postgres-data` Docker Volume (ersetzt `audit-data`)

## Go-Dependencies

- Hinzufügen: `github.com/lib/pq` (PostgreSQL-Treiber, `database/sql`-kompatibel)
- Entfernen: `modernc.org/sqlite`
- Neue Hilfsfabrik: `pkg/db/postgres.go` — `Open(databaseURL string) (*sql.DB, error)`

## Konsequenzen

**Positiv:**
- Einheitliche DB-Technologie für alle Services
- Echter Connection-Pool (5 open, 2 idle) statt Single-Writer-Serialisierung
- Standardisierte Backup-Workflows (`pg_dump`)
- Voraussetzung für ADR-024 (User Management) ohne zweites SQLite in Auth-Service

**Negativ / Risiken:**
- PostgreSQL-Container als neue Infrastruktur-Abhängigkeit (Startup-Reihenfolge via `depends_on: condition: service_healthy`)
- Netzwerkhop auf SAFE_MODE-kritischem Pfad (mitigiert durch `synchronous_commit=on` + `connect_timeout=5s`)
- Bestehende SQLite-Daten (`avoc_audit.db`) werden nicht migriert — Safety-Event-History geht bei erstem Deploy verloren (akzeptabel für Testbetrieb)

## Alternativen verworfen

- **SQLite behalten + UserStore-SQLite im Auth-Service:** Zwei SQLite-Dateien in verschiedenen Containern, kein Sharing möglich ohne gemeinsames Volume. Erhöht Komplexität statt zu vereinfachen.
- **SQLite für Audit behalten, PG nur für User:** Split-Infrastruktur ohne Mehrwert. Safety-kritischer Pfad bleibt auf SQLite, kein Gewinn.
