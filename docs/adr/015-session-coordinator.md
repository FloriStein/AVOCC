# ADR-015: Session Coordinator — Control Session als primäre Einheit

Status: Accepted

## Kontext

Das System hat mehrere parallele Verbindungen (Vehicle, Operator, SFU, Safety Bus), aber keinen explizit definierten "System of Record" für Session Context. Ohne klare Session Ownership entstehen Zustandsdrift zwischen Control Hub und Video Hub, inkonsistente Operator States und nicht rekonstruierbare Incidents. ADR-015 definiert die Control Session als einzige safety-relevante Einheit und den Control Server als Global Session Authority.

---

## Session-Hierarchie (3 Schichten)

```
1. Vehicle Runtime Context   (Transport Layer)
   └── physische Verbindung: WebRTC / MQTT / WS / 5G
       kommt und geht, hat keine Safety-Relevanz

2. Control Session            (ADR-015 — primär, Safety Layer)
   └── Vehicle + Active Operator + Control Server
       einzige Einheit mit Safety-Relevanz

3. Operator Session           (Identity Layer)
   └── Login / JWT / Permissions
       Basis für Authentifizierung, NICHT für Execution Context
```

**Kernregel:** Nur die Control Session besitzt Safety-Relevanz. Alles andere ist Träger- oder Identity-Schicht.

---

## Optionen

### Option A: Vehicle Session (zu technisch)
## Vorteile:
- Einfach
## Nachteile:
- Ignoriert Operator + Safety Logik
- Führt zu falscher Kopplung: „Fahrzeug = Session"

### Option B: Operator Session (falsch für Safety)
## Vorteile:
- Operator-zentriert
## Nachteile:
- Ein Operator kann mehrere Fahrzeuge haben — widerspricht 1:1 Safety Control
- Login ≠ Execution Context

### Option C: Control Session
## Vorteile:
- Exakt eine Safety-relevante Einheit
- Operator-Handover möglich ohne Session-Wechsel
- Vehicle-Reconnect möglich ohne Session-Wechsel (RECOVERING)
- Single Source of Truth für State Machine, Failure Model, Recording, Correlation ID
## Nachteile:
- Komplexere Koordination zwischen den drei Schichten

## Entscheidung

Wir wählen **Option C: Control Session** als primäre operative Einheit.

## Definition Control Session

Eine Control Session ist die zeitlich begrenzte, sicherheitskritische Verbindung zwischen:
- exakt **1 Vehicle**
- exakt **1 Active Operator** (wechselbar via Handover)
- exakt **1 Control Server Instanz**

### Lebenszyklus

```
Entsteht:   CONNECTING → CONNECTED (Vehicle + Operator + Safety Bus OK)
Besteht:    während CONNECTED, DEGRADED, SAFE_MODE, RECOVERING
Endet bei:  CRITICAL Failure ohne Recovery, explizites Session-Ende, Timeout
```

Operator kann wechseln (HANDOVER) — Session bleibt bestehen.
Vehicle kann reconnecten (RECOVERING) — Session bleibt logisch bestehen.

---

## Session State Persistenz: Ephemeral + Checkpoint

### Entscheidung

Session State ist **in-memory** (ephemeral). Beim Eintritt in SAFE_MODE wird ein **Recovery Checkpoint** gesichert.

### Checkpoint-Inhalt

```
{
  session_id:        ULID (ADR-016)
  vehicle_id:        string
  operator_id:       string
  last_system_state: SYSTEM STATE Enum
  last_control_state: CONTROL STATE Enum
  safety_reason:     CRITICAL Trigger Enum
  checkpoint_ts:     timestamp
  correlation_id:    ULID (Root, für ADR-016 Kontinuität)
}
```

### Verhalten bei SAFE_MODE

```
1. State einfrieren
2. Checkpoint speichern (in-memory, optional: storage-hook für Persistenz)
3. Channel schließen (ADR-010 Channel Close)
```

### Verhalten bei Recovery

```
1. Neues Session-Objekt erstellen
2. Checkpoint laden
3. State: RECOVERING
4. Operator-Ack erforderlich
5. → CONNECTED (neue Execution Branch, gleiche Session-ID als Root)
```

**Designregel:** Recovery ist kein Wiederherstellen eines laufenden Zustands, sondern das bewusste Neu-Aktivieren eines validierten Zustands.

---

## SFU Session Context: Event-driven Push

### Entscheidung

Der Control Server **pusht** Session Events asynchron an den SFU. Der SFU hält einen lokalen Session Cache, der ausschließlich durch Events aktualisiert wird.

### Session Events

```
SESSION_CREATED       → SFU beginnt Stream-Routing vorzubereiten
OPERATOR_ASSIGNED     → SFU weiß, an wen Primary Stream forwarden
OPERATOR_HANDOVER     → SFU aktualisiert Routing sofort
SESSION_DEGRADED      → SFU zeigt Degraded-Status
SESSION_SAFE_MODE     → SFU droppt Streams sofort (Operator-Sicht: schwarz)
SESSION_ENDED         → SFU beendet alle Streams, gibt Ressourcen frei
```

**Designregel:** *SFU darf State konsumieren, aber niemals interpretieren. Der SFU ist ein Dumb Media Router with State Subscription.*

### Warum nicht Pull oder JWT

- **Pull:** Race Conditions bei Handover, SFU könnte falschen Operator zeigen
- **JWT:** Statisch — Handover, SAFE_MODE, Recovery nicht abbildbar; Sicherheitsrisiko bei Operator-Wechsel

---

## Control Server = Global Session Authority (GSA)

Der Control Server ist der einzige Punkt im System, der:
- Sessions erzeugen darf
- Sessions zerstören darf
- Sessions rekonstruieren darf (RECOVERING)
- Session Context an SFU pushen darf
- Correlation IDs generieren darf (ADR-016)

---

## Ownership-Tabelle

| Domäne | Owner |
|--------|-------|
| Session Lifecycle | Control Server (GSA) |
| Identity / JWT | Auth Service |
| Video Routing | SFU (Consumer, nicht Owner) |
| Vehicle Transport | Vehicle Runtime Context |
| State Machine | Control Server (ADR-011) |
| Safety Decisions | Control Server (ADR-009) |
| Correlation IDs | Control Server (ADR-016) |

## Konsequenzen

### Positiv:
- Eindeutiger Single Source of Truth für Session Context
- SFU kann sofort auf SAFE_MODE reagieren (keine Polling-Verzögerung)
- Recovery ist sauber strukturiert — kein Zombie-Session-Risiko
- Control Session = auditierbare, rekonstruierbare Safety-Einheit

### Negativ:
- Control Server trägt mehr Verantwortung — interne Modulstruktur ist essenziell (ADR impl-plan)
- SFU Event Stream muss zuverlässig implementiert werden (Delivery Guarantees)
- Session Checkpoint erfordert bewusste Implementierung der Storage-Hook-Schnittstelle
