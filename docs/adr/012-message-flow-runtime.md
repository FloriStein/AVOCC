# ADR-012: Message Flow Runtime Model & Protobuf Versioning

Status: Partially Accepted (Versioning entschieden, Sync/Async-Modell und Browser-Encoding offen)

## Kontext

Das Hub-and-Spoke-Modell (ADR-007) und Protobuf als Message Format (ADR-008) sind entschieden, aber das Runtime-Verhalten der Kommunikation ist nicht spezifiziert: Wer spricht wann mit wem, welche Kanäle sind synchron und welche asynchron, und wie wird Protobuf im Browser dekodiert. Ohne diese Definitionen entstehen inkonsistente Service-Interaktionen und unkontrollierte Blocking-Effekte.

---

## Teil 1: Protobuf Versioning Strategy

### Optionen

**Option A: Package-Versionierung (v1, v2 als eigene Packages)**
- Saubere Trennung, keine Kompatibilitätszwänge
- Nicht gewählt: hoher Migrationsaufwand, inkonsistente Runtime-Versionen möglich

**Option B: Field-based Versioning**
- Backward Compatibility ohne Service-Migration
- Neue Features über optionale Felder
- Veraltete Felder bleiben (deprecated), werden nie entfernt

### Entscheidung: Option B — Field-based Versioning

### Regeln
- Keine Änderung bestehender Field IDs
- Keine Entfernung bestehender Felder
- Neue Features ausschließlich über optionale Felder
- Veraltete Felder werden mit `deprecated` markiert, bleiben im Schema
- CI erzwingt Schema-Kompatibilitätsprüfung bei jedem Commit

### Konsequenzen
- Schema wächst über Zeit — klare Disziplin im Team erforderlich
- CI muss Schema-Validierung als Pflicht-Gate einführen
- Keine Breaking Changes durch neue Service-Versionen

---

## Teil 2: Sync vs. Async je Kommunikationskanal

Status: **Offen — Folge-Entscheidung erforderlich**

Offene Fragen:

| Kanal | Offen |
|-------|-------|
| Control Server → Safety Event Bus | Synchron (warte auf Bestätigung) oder Asynchron? |
| Control Server → Auth Service (Token Validation) | Synchron (blockiert bis JWT validiert) oder Cached/Async? |
| Frontend → Control Server (Commands) | Wartet Frontend auf ACK oder fire-and-forget? |
| Control Server → MQTT Telemetry | Async (one-way, bereits klar) |

→ Muss als Folge-ADR-012b entschieden werden, bevor Phase 2 beginnt.

---

## Teil 3: Browser-Encoding für Protobuf

Status: **Offen — Folge-Entscheidung erforderlich**

| Option | Beschreibung |
|--------|-------------|
| A: `protobufjs` | JavaScript Library, Runtime-Decoding aus `.proto` Dateien |
| B: Code-Gen | Statische TypeScript-Klassen aus `.proto` generiert |

→ Muss entschieden werden vor Beginn von FE-09 (Frontend Protobuf Adapter).

---

## Konsequenzen (bereits entschieden)

### Positiv:
- Field-based Versioning verhindert Breaking Changes ohne Koordination
- CI Schema-Gate schützt alle Services vor inkompatiblen Updates

### Negativ:
- Sync/Async-Modell ist noch nicht entschieden — Implementierung von BE-04 und FE-02 wird dadurch blockiert
- Browser-Encoding-Entscheidung blockiert FE-09
