# vision.md

## 1. Projektvision

Das Ziel dieses Projekts ist die Entwicklung eines sicheren, modularen und skalierbaren Teleoperationssystems zur Fernsteuerung und Überwachung von Fahrzeugen in Echtzeit.

Das System soll es einem Operator ermöglichen, ein Fahrzeug über eine Netzwerkverbindung zuverlässig zu steuern, zu überwachen und in kritischen Situationen sofort zu kontrollieren oder zu stoppen.

---

## 2. Problemstellung

Autonome oder halbautonome Fahrzeuge benötigen in vielen Szenarien eine menschliche Fernüberwachung oder manuelle Intervention.

Dabei entstehen zentrale Herausforderungen:

- hohe Latenz in Netzwerkverbindungen
- unzuverlässige Verbindungen im mobilen Umfeld
- Sicherheitsrisiken bei Steuerungsverlust
- komplexe Echtzeit-Kommunikation (Video + Steuerung + Telemetrie)

Dieses Projekt adressiert genau diese Probleme durch eine strukturierte, mehrschichtige Systemarchitektur.

---

## 3. Ziel des Systems

Das System soll folgende Kernziele erfüllen:

- Echtzeit-Steuerung von Fahrzeugen über eine Remote-Verbindung
- sichere und stabile Verbindung auch bei schwankender Netzqualität
- gleichzeitige Übertragung von Steuerdaten, Video und Telemetrie
- garantierte Sicherheitsmechanismen zur Vermeidung gefährlicher Zustände
- nachvollziehbare Session- und Entscheidungsprotokollierung

---

## 4. Zielgruppen

- Operatoren von ferngesteuerten Fahrzeugen
- Entwickler von autonomen / semi-autonomen Systemen
- Forschungs- und Testumgebungen für Robotik und Mobilität
- industrielle Fernwartungs- und Steuerungssysteme

---

## 5. Kernfunktionen (High-Level)

- Echtzeit-Teleoperation eines Fahrzeugs
- Video-Streaming aus mehreren Kameraquellen
- Manuelle Steuerung über UI (Joystick, Keyboard, Gamepad)
- Sicherheitsmechanismen (Dead-man switch, Emergency Stop)
- Verbindungsmanagement und Latenzüberwachung
- Session Recording zur Nachvollziehbarkeit

---

## 6. Nicht-Ziele (Explicit Non-Goals)

Dieses System ist NICHT:

- ein vollautonomes Fahrsystem
- ein Consumer-Entertainment-Produkt
- ein Offline-Steuerungssystem ohne Netzwerkabhängigkeit
- ein rein lokales Embedded-Control-System ohne Cloud/Netzwerk

---

## 7. Grundprinzipien

Die Architektur basiert auf folgenden Leitprinzipien:

- Safety First (Sicherheitsmechanismen haben höchste Priorität)
- Real-Time Communication (minimale Latenz ist kritisch)
- Multi-Channel Architecture (Trennung von Video, Control, Telemetry, Safety)
- Failure Awareness (System muss Ausfälle erkennen und reagieren können)
- Explicit Design Decisions (alle wichtigen Entscheidungen werden dokumentiert)

---

## 8. Qualitätsziele

- stabile Kommunikation unter variierenden Netzwerkbedingungen
- Reaktionszeit im Kontrollpfad unter 100ms
- robuste Fehler- und Disconnect-Behandlung
- klare Trennung von Sicherheits- und Steuerungssystemen
- nachvollziehbare Systemzustände zu jedem Zeitpunkt

---

## 9. Architekturleitlinie

Die endgültige Architektur wird nicht vorausgesetzt, sondern entsteht iterativ durch:

- Anforderungen
- ADR-basierte Entscheidungen
- kontinuierliche Validierung durch Grill-Me Sessions

Technologieentscheidungen werden nicht vorab fixiert, sondern begründet getroffen.

---

## 10. Erfolgskriterien

Das Projekt gilt als erfolgreich, wenn:

- ein Operator ein Fahrzeug zuverlässig in Echtzeit steuern kann
- Sicherheitsmechanismen zuverlässig eingreifen
- Verbindungsabbrüche keine unkontrollierten Zustände erzeugen
- das System modular erweiterbar bleibt
- alle Architekturentscheidungen nachvollziehbar dokumentiert sind
