# ADR-022: Vehicle Registry

**Status:** Accepted  
**Datum:** 2026-06-12  
**Entscheider:** Florian Steinmann

---

## Kontext

`vehicle-001` war in 5 Frontend-Dateien hardcoded. Das System unterstützte ausschließlich ein fest verdrahtetes Fahrzeug. Für eine realistische Flottensteuerung muss der Operator vor Session-Start aus einer vordefinierten Liste von Fahrzeugen wählen können.

**Randbedingungen:**
- ADR-015 (Single Active Session) bleibt unverändert — immer genau ein Fahrzeug aktiv
- Keine Auto-Registrierung beim WS-Connect; Fahrzeuge werden manuell vorkonfiguriert
- Bestehende `avoc_audit.db` soll weiterverwendet werden (keine zweite Datei)

---

## Entscheidung

### Persistenz

Eine neue `vehicles`-Tabelle in der bestehenden `avoc_audit.db` (shared WAL-Connection via `SQLiteAuditWriter.DB()`):

```sql
CREATE TABLE IF NOT EXISTS vehicles (
    id           TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL
);
```

Der `online`-Status wird **nicht** in der DB gespeichert — er wird immer live aus `vehicleconnection.Registry.Connected()` abgefragt. Eine veraltete Online-Markierung in der DB wäre schlimmer als keinen Status zu haben.

### Auto-Seed

Beim Server-Start wird `vehicle-001` via `INSERT OR IGNORE` angelegt. Das stellt Rückwärtskompatibilität mit dem `vehicle-mock`-Container (`VEHICLE_ID: "vehicle-001"`) sicher.

### Backend-Interface

`VehicleStore` ist ein Interface — ermöglicht `NoopVehicleStore` für Tests und Fallback wenn die DB nicht erreichbar ist:

```go
type VehicleStore interface {
    List() ([]Vehicle, error)
    Add(id, displayName, description string) error
    Delete(id string) error
    Exists(id string) (bool, error)
    SeedDefault() error
}
```

### REST-Endpoints

| Method | Path | Beschreibung |
|--------|------|--------------|
| `GET /vehicles` | — | Liste aller Fahrzeuge mit Live-`online`-Flag |
| `POST /vehicles` | `{id, display_name, description}` | Fahrzeug anlegen (409 wenn ID existiert) |
| `DELETE /vehicles/{id}` | — | Löschen (409 wenn aktuell in aktiver Session) |

### Frontend-Flow

1. App startet → Auto-Connect → WS-Verbindung → System `AUTHENTICATED`
2. Im `ConnectionPanel` erscheint der `VehicleSelector` (Dropdown + "Session starten"-Button)
3. `useVehicles` pollt `GET /api/vehicles` alle 2 Sekunden
4. Operator wählt Fahrzeug → klickt "Session starten" → `POST /session/start` mit gewählter `vehicle_id`
5. System wechselt zu `CONNECTED`, Video + Controls werden aktiv

Der Auto-Start (`startSessionIfNeeded` beim AUTHENTICATED-Eintritt) wurde entfernt — der Operator entscheidet explizit.

---

## Konsequenzen

**Positiv:**
- Mehrere Fahrzeuge verwaltbar ohne Code-Änderung
- Live-Online-Status direkt aus WS-Registry — kein Polling auf Fahrzeugebene nötig
- `NoopVehicleStore` hält Tests stabil ohne DB-Abhängigkeit
- Rückwärtskompatibel: `vehicle-001` immer vorhanden nach Seed

**Negativ / Risiken:**
- Keine Persistenz des Online-Status: Neustart des Control-Servers markiert alle Fahrzeuge als offline bis sie sich reconnecten (akzeptabel — WS-Reconnect passiert in <5s)
- CRUD nur über API (kein UI-Formular): Neue Fahrzeuge müssen via `curl` oder zukünftigem Admin-UI angelegt werden

---

## Abgelehnte Alternativen

**Separate SQLite-Datei:** Vermieden, da shared WAL-Connection einfacher ist und zwei Dateien für `SetMaxOpenConns(1)` problematisch werden.

**Auto-Register beim WS-Connect:** Abgelehnt, weil unbekannte Fahrzeuge automatisch als vertrauenswürdig behandelt würden — sicherheitskritisch für ein Teleoperationssystem.

**Multi-Vehicle gleichzeitig:** Bleibt ausgeschlossen (ADR-015). Erhöht die kognitive Last des Operators und die Systemkomplexität ohne Mehrwert für den aktuellen Use Case.
