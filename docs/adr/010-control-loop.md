# ADR-010: Control Loop Definition & Safety Override Mechanismus

Status: Accepted

## Kontext

Das System hat ein hartes Latenz-Ziel von <100ms für den Control Loop, aber weder das Loop-Modell noch der Messpunkt noch der Safety Override Mechanismus waren technisch definiert. Ohne diese Definitionen ist das Latenz-Ziel nicht testbar und Safety Enforcement nicht implementierbar.

---

## Teil 1: Control Loop Modell

### Optionen

**Option A: Event-driven Stream**
- Operator-Inputs werden als kontinuierlicher Event-Stream über WebSocket gesendet
- Server verarbeitet Commands sofort bei Eingang

**Option B: Fixed Tick Rate**
- Server fragt in festem Intervall nach Commands
- Deterministisch, aber künstliche Verzögerung

**Option C: Request-Response**
- Operator sendet Command, wartet auf ACK
- Klare Messung, aber hohe Latenz durch Roundtrip-Warten

### Entscheidung: Option A — Event-driven Stream

### Begründung
- Minimale End-to-End Latenz für Teleoperation
- Keine künstlichen Tick-Verzögerungen
- Passt zu WebSocket-basiertem Design

### Konsequenzen
- Server muss Rate Limiting und Backpressure implementieren
- Keine garantierte feste Update-Frequenz
- Verarbeitung erfolgt asynchron im Control Server

---

## Teil 2: Latenz-Messpunkt

### Entscheidung: Client sendet Command → Server verarbeitet → ACK zurück (Roundtrip bis Control Server Processing Completion)

### Begründung
- Unabhängig von Fahrzeug-Hardware-Latenz
- Reproduzierbar in CI-Umgebung
- Misst echte Control-Plane-Latenz
- Ermöglicht automatische Performance Regression Tests

### Nicht enthalten
- Fahrzeug-Aktuation wird separat im Field Test bewertet
- Netzwerk-Latenz zum Fahrzeug ist nicht Teil des Messmodells

---

## Teil 3: Safety Override Mechanismus

### Optionen

**Option A: State Gate**
- Control Server prüft Safety State vor jeder Command-Ausführung
- Nicht gewählt: Race Condition möglich zwischen Check und Ausführung

**Option B: Command Drop**
- Safety Layer verwirft Commands in Queue
- Nicht gewählt: komplexe Queue-Logik, schwer testbar

**Option C: Channel Close**
- Bei CRITICAL Safety Event wird Control WebSocket Verbindung geschlossen oder in SAFE MODE gesperrt
- Neue Commands werden nicht akzeptiert
- Client muss neue Session initiieren

### Entscheidung: Option C — Channel Close

### Begründung
- Eliminiert Race Conditions zwischen Safety Check und Command Execution
- Keine in-flight Commands möglich nach Safety Event
- Safety Enforcement auf Transport Layer — deterministisch und testbar
- Klare Failure Semantics

### Konsequenzen
- Frontend muss Reconnect-Logik nach Channel Close implementieren
- Session State muss extern persistiert werden (beeinflusst BE-06, BE-07)
- SAFE MODE ist nicht nur ein Zustand, sondern eine aktive Kommunikationsblockade
- Safety Test Suite muss Channel-Close-Szenarien explizit abdecken
