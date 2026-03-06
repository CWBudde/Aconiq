# Vollständiger Implementierungsplan für eine SoundPLAN-ähnliche Umgebungslärm-Software mit Go-Backend und React-Frontend

## Zielbild, Abgrenzung und Leitprinzipien

Du willst eine Software „ähnlich SoundPLAN“, aber nicht alles sofort implementieren. Gleichzeitig soll der Plan das **Gesamtziel** (volle Produktbreite) abdecken und dann in **viele Phasen** zerlegt werden: Backend (Go, zunächst CLI via Cobra, später ggf. Wails), Frontend (Browser-GUI mit React/TypeScript, Vite, Bun) plus eine leistungsfähige Karten-/GIS-Engine.

Das Zielprodukt sollte – wie kommerzielle Systeme – **modular** sein: SoundPLAN positioniert sich explizit als modular aufgebaut und betont, dass Berechnungsstandards (z. B. CNOSSOS, RLS‑19) fortlaufend implementiert werden. citeturn5search0turn5search3turn5search1 Daraus folgt für dich als Architekturprinzip: **Standards/Normen sind Plug-ins** (fachliche Module) – nicht „Hartkodierung im Kern“.

Leitprinzipien für den Gesamtplan:

**Trennung in Kern + Normen + Workflow:** Ein generischer Akustik-/Geometrie-Kern (Datenmodell, Geometrie, Ausbreitungshilfen, Summation, Raster/Receiver-Management) plus normative Module, die Emission, Ausbreitungskorrekturen, Indikatoren und Berichtstabellen definieren. Das ist die einzige skalierbare Strategie, wenn du später mehrere Standards (EU/DE/weitere Länder) unterstützen willst. citeturn5search3turn2search4

**Zwei Betriebsmodi von Anfang an denken:**  
CLI-first ist gut (Automatisierung, Batch-Computing, CI), aber ein Browser-UI braucht typischerweise einen Job-Server/HTTP-API. Ein Cobra-CLI kann daher früh zwei Modi anbieten:  
- **Batch/Offline:** `noise run ...` erzeugt Ergebnisse (Raster, Tabellen, Reports) in einem Projektordner.  
- **Server/GUI:** `noise serve` startet lokale API + Websocket/SSE für Fortschritt, das React-UI verbindet sich darauf.

**Qualitätssicherung ist nicht „später“:** Für Lärmsoftware existieren explizite Testaufgaben/QA-Konzepte (DIN 45687 / ISO-Ansatz) mit Testaufgaben und Konformitätserklärungen. Das ist prädestiniert für „Tests früh“ und Acceptance-Tests als Teil deiner CI. citeturn4search15turn2search31turn4search16turn4search7

---

## Normen- und Methodenrahmen, den dein Design abdecken muss

### EU-Rahmen: END + CNOSSOS-EU als Kern (Strategische Lärmkartierung)

Die **EU-Umgebungslärmrichtlinie 2002/49/EG (END)** setzt den Rahmen für Bewertung/Management von Umgebungslärm und nutzt als zentrale Indikatoren **Lden** (Tag-Abend-Nacht) und **Lnight** (Nacht). citeturn2search0turn2search15turn2search7  
Die **Gemeinsamen EU-Bewertungsmethoden** werden über den CNOSSOS-EU-Rahmen geregelt: zunächst durch **Richtlinie (EU) 2015/996**, später technisch aktualisiert durch **Delegierte Richtlinie (EU) 2021/1226** (Anpassungen/Updates in Anhang II, u. a. Klarstellungen von Formeln/Tabellen/Schritten). citeturn0search0turn0search1turn0search25turn2search4

Für eine „vollständige“ Produktvision heißt das:  
- CNOSSOS-Module für **Straße, Schiene, Industrie, Flugverkehr** (mindestens als langfristige Roadmap; CNOSSOS wird genau für diese Quellklassen genutzt). citeturn0search8turn0search0  
- Ausgabe-Workflows für strategische Karten + Expositionsauswertungen: Die EEA sammelt z. B. Expositionszahlen in **5‑dB-Bändern** für **Lden** und **Lnight** (Reporting-Kontext). citeturn2search1turn2search30

### Deutschland: zwei Welten – Kartierung vs. Projekt/Immissionsschutz

Für Deutschland ist wichtig, dass es **unterschiedliche Regelwerke je Zweck** gibt.

Für **Lärmkartierung (34. BImSchV / Umgebungslärm)** existiert die **BUB** („Berechnungsmethode für den Umgebungslärm von bodennahen Quellen: Straßen, Schienenwege, Industrie/Gewerbe“) inklusive Datenbank-Bezug (BUB‑D) und die Möglichkeit, **Lden** und **Lnight** für Kartierungszwecke zu berechnen. citeturn4search0turn4search12turn4search4  
Die BUB-Fassung betont zugleich explizit, dass sie **nicht** für Schallberechnungen nach **16. BImSchV** und **TA Lärm** gilt. citeturn4search0  

Für **Projekt/Planung/Schutzmaßnahmen** relevanter sind u. a.:  
- **RLS‑19** (Straßenverkehrslärm) als modernes Emissions-/Berechnungsregelwerk; es existieren sogar offizielle **TEST‑20**-Testaufgaben zur Überprüfung von Rechenprogrammen inkl. Konformitätserklärung (perfekt als Acceptance-Test-Suite). citeturn2search2turn2search31turn0search3  
- **Schall 03** (Schienenverkehr) ist in Deutschland eng mit der 16. BImSchV verknüpft (Anlage 2) und arbeitet u. a. mit Oktavbändern 63 Hz bis 8 kHz. citeturn4search5turn4search1  

Zusätzlich gibt es QA-/Testaufgaben-Landschaften auch jenseits von RLS‑19: DIN selbst stellt „Testaufgaben zur Berechnung von Schallausbreitung im Freien“ als QS-Material bereit. citeturn4search7  
Und das UBA beschreibt QS im Sinne DIN 45687 als kontinuierlichen Prozess, der u. a. auf Testaufgaben/Konformitätserklärungen/Austauschformate setzt. citeturn4search15

### Industrielärm: internationale Normen als Teil des Vollausbaus

Ein Vollausbau wird fast immer zumindest eine international etablierte Methode für Industrielärm enthalten. ISO beschreibt **ISO 9613‑2** als Engineering-Methode zur Berechnung der Dämpfung bei Außenausbreitung zur Vorhersage von Umweltschallpegeln. citeturn0search2turn0search18  
Wichtig für die Roadmap ist nicht, dass du ISO‑Texte frei zitierst (oft paywalled), sondern dass du die Methodik als **separates Normmodul** planst und mit öffentlich verfügbaren Testaufgaben/Referenzen validierst.

---

## Zielarchitektur als „System von Modulen“

### Laufzeit- und Integrationsmodell

**Backend (Go)**
- Bibliotheken:  
  - Akustik-/Geometrie-Kern (reine Go-Libs, ohne IO/UI)  
  - Normmodule (CNOSSOS-EU, BUB, RLS‑19, Schall 03, ISO 9613‑2 …)  
  - IO/Import/Export  
  - Job-Engine (Parallelisierung, Caching, deterministische Ausführung)  
- „Hülle“: Cobra-CLI (und später optional Wails als Desktop-Packaging)

**Frontend (Browser)**
- React + TypeScript, gebaut mit Vite; Bun als Runtime/Package-Manager/Test-Runner (Bun beschreibt sich als All-in-One Toolkit inkl. Package-Manager, Test-Runner und Bundler). citeturn3search18turn3search26  
- Kartenengine: MapLibre GL JS – eine TypeScript-Library, die WebGL nutzt, um interaktive Karten aus Vector Tiles im Browser zu rendern. citeturn3search1turn3search33

**Kopplung**
- Kurzfristig: REST/JSON + WebSocket/SSE für Fortschritt  
- Mittelfristig: gRPC/Connect oder OpenAPI, plus “schema-first” DTOs (TypeScript-Typen generieren)  
- Langfristig (Wails): UI und Go in einem Desktop-Binary; Wails beschreibt explizit das Bündeln von Go-Code und Web-Frontend in ein Single Binary als Alternative zum „Go-Webserver + Browser“-Ansatz. citeturn3search12turn3search4

### Datenhaltung: projektorientiert + skalierbar

Ein realistischer Ansatz ist zweistufig:

**Projektformat v1 (lokal, CLI-freundlich):**  
- Ein Projektordner mit Versionierung/Manifest  
- SQLite (oder reine Dateien) für Metadaten  
- Ergebnisse als GeoTIFF/Cloud-optimierte Raster oder einfache Grid-Container + JSON-Metadaten  
- Vektor-Geometrien (Sources/Barriers/Buildings/Receivers) in GeoJSON/GeoPackage/FlatGeobuf (du kannst später erweitern)

**Projektformat v2 (Multiuser/Server):**  
- PostGIS + Objektstorage (Tiles, Raster, Reports)  
- Versionierte „Szenarien“ und „Runs“  
(Deployment ist nicht Kernfokus, aber die Architektur sollte es erlauben.)

### Karten-/Tile-Strategie

Für eine leistungsfähige Web-GUI brauchst du Tiles. Eine sehr praktikable Option ist **PMTiles**: ein Single-File Archivformat für „Pyramiden“ von Kacheldaten (Z/X/Y), geeignet für kostengünstiges Hosting (S3/Static Storage) und „serverless“ Karten. citeturn3search3turn3search7  
Das ist attraktiv, wenn du Basemaps/Referenzdaten offline oder ohne eigenen Tile-Server ausliefern willst.

---

## Backend-Plan in Go: CLI jetzt, API/GUI später, Wails optional

### Kern-Tooling: Cobra als CLI-Grundgerüst

Cobra ist eine etablierte Go-Bibliothek für moderne Subcommand-CLIs („git/go“-Stil). citeturn1search3turn1search19  
Du solltest die gesamte Funktionalität als Cobra-Kommandos modellieren, da das später eine saubere Brücke zu „GUI startet intern dieselben Use-Cases“ bildet.

**Vorgeschlagene CLI-Topologie (stabil ab Phase „Foundation“):**
- `noise init` – Projekt anlegen (Manifest, CRS, Defaults)
- `noise import` – GIS/CSV/Traffic/Source-Daten importieren (validieren)
- `noise validate` – Geometrie, Attribute, Norm-Kompatibilität prüfen
- `noise run` – Berechnung starten (Szenario + Standard + Receiver-Set)
- `noise status` – Run-Status/Logs
- `noise export` – GeoTIFF/Contours/CSV/JSON/Report Bundles
- `noise serve` – Lokaler API-Server für Browser-GUI (auth optional)
- `noise bench` – Performance-Benchmarks, deterministische Repro-Runs

### Verwendung deiner algo-* Bibliotheken im Backend

Du hast explizit genannt: `github.com/algo-dsp`, `github.com/algo-pde`, `github.com/algo-fft`.

- **algo-dsp**: „Production-quality DSP algorithms for Go“; algorithmisch und transport-agnostisch. citeturn1search0  
  Sinnvolle Rollen im Lärmkontext: Spektral-/Bandverarbeitung, A‑Bewertungsvorgänge, Filter/Glättung als Postprocessing (z. B. optionales „map smoothing“, Detektion von Hotspots, spektrale Kennwerte).

- **algo-fft**: High-Performance FFT-Library für Go. citeturn1search2turn1search6  
  Rollen: FFT-basierte Faltungen (rasterspezifische Filter), schnelle 2D/3D-Operationen bei Grid-Daten, ggf. Vorverarbeitung/Accelerators für numerische Teilprobleme.

- **algo-pde**: Spektrale Poisson/Helmholtz Solver basierend auf algo-fft; plan-basiert wie FFTW, um Eigenwerte/Transform-Pläne wiederzuverwenden. citeturn1search1turn1search9  
  Rollen: nicht als „normativer Default“ (weil CNOSSOS/RLS/BUB formelbasiert sind), sondern als **F&E-/Advanced-Modul**: Wave-Propagation-Demos, Low-Frequency-Szenarien, Sensitivitätsanalysen oder Forschung zur Approximation/Validierung. Der Repo beschreibt sogar eine interaktive Wave-Propagation-Demo via WebAssembly als Machbarkeitsbeleg für spätere „Interaktiv“-Features. citeturn1search1

Die zentrale Produktregel: **Normative Outputs dürfen nur aus normativen Modulen kommen** (CNOSSOS/BUB/RLS/Schall03/ISO). PDE/DSP/FFT-basierte „Enhancements“ laufen als optionales Postprocessing oder als getrennte „Research“-Pipelines, damit Compliance nicht verwässert wird.

### Backendschichten und Pakete

Für ein großes Projekt ist eine klare Package-Struktur essentiell. Eine praxistaugliche Struktur (ohne Dogma) wäre:

- `cmd/noise/` – Cobra Commands + wiring
- `internal/app/` – Use-Cases (Application Services): Import, Validate, Run, Export, Serve …  
- `internal/domain/` – Entities & Value Objects: Project, Scenario, Source, Receiver, Terrain, Building, Barrier, StandardRef, Run, ResultRef …
- `internal/standards/` – Normmodule, jeweils unabhängig testbar:
  - `cnossos/` (EU)
  - `bub/` (DE Kartierung; bodennahe Quellen) citeturn4search0
  - `rls19/` (DE Straße) citeturn2search2turn2search31
  - `schall03/` (DE Schiene) citeturn4search5
  - `iso9613/` (Industrie, International) citeturn0search2
- `internal/geo/` – CRS, Geometrie-Operationen, Spatial Indexing, Raster/Grid
- `internal/engine/` – Compute Engine: Contribution-Loop, Parallelisierung, Caching, Determinismus
- `internal/io/` – Import/Export Adapter (GIS, Tabellen, Tiles)
- `internal/report/` – tabellarische Ausgaben, PDF/Markdown/Docx-Pipelines (Docx später)
- `internal/api/` – HTTP API + WS/SSE (nur für `serve`)
- `internal/qa/` – Testkataloge, Goldenfiles, Norm-Testaufgaben Loader

### Compute Engine: deterministisch, parallel, „job-ified“

SoundPLAN wirbt stark über „High computing power“ und „Multicore/multithreading“. citeturn5search1turn5search2turn5search35  
Du solltest das als Erwartungshaltung betrachten: große Receiver-Grids/City-Scale Kartierung müssen batchfähig sein.

Planvorgaben für den Engine-Kern:
- **Deterministische Ausführung**: gleiche Eingaben → gleiche Outputs, unabhängig von Parallelisierungsgrad (wichtig für QS, Regression, „Konformitätserklärung“).
- **Job-Modell**: Ein Run wird in Tiles/Receiver-Chunks aufgeteilt; jeder Chunk berechnet contributions, aggregiert und persistiert.
- **Caches/Precomputation**:  
  - Geometrische Vorberechnung (Spatial Index, Sichtlinien, Candidate Source Sets)  
  - Normspezifische Tabellen/Parameter (Road Surface Corrections etc., je Standard-Version) citeturn0search1turn0search0
- **Progress & Cancel**: Muss für GUI und CLI funktionieren.

---

## Frontend-Plan: React/TypeScript, MapLibre-Map-Engine, professionelle Workflows

### Toolchain: Vite + Bun + React/TS

Vite positioniert sich als „Next Generation Frontend Tooling“ (schneller Dev-Server, HMR, TypeScript/JSX/WASM etc. out of the box). citeturn3search26turn3search6  
Bun dokumentiert explizit, dass Vite „out of the box“ mit Bun funktioniert; außerdem liefert Bun Paketmanager und Tooling. citeturn3search2turn3search18

Damit kannst du ein Setup planen, das von Tag 1 CI-fähig ist (Bun für Install/Test/Build).

### Kartenengine: MapLibre GL JS als zentrales UI-Element

MapLibre GL JS ist eine TypeScript-Library und rendert interaktive Karten via WebGL aus Vector Tiles. citeturn3search1turn3search33  
Das ist für dich wichtig, weil du:
- sehr viele Objekte (Straßenachsen, Gebäude, Barrieren, Receiver-Punkte) performant darstellen musst,
- Raster/Heatmaps/Contours als Layer brauchst,
- Styling (Legenden/Color Ramps) kontrollieren musst.

### UI-Module und Workflows (Endzustand)

Für die „Full Implementation“-Vision planst du die GUI als eine Folge von „Workflows“, nicht als lose Buttons:

**Projekt & Szenarien**
- Projekt-Explorer (Szenarien, Standards, Runs, Artefakte)
- Szenario-Diff (A/B Vergleich, „Change sets“)

**Modellierung auf der Karte**
- Quellen-Editor: Line/Point/Area (Attribute je Norm/Modul)
- Import-Assistenten für GIS + Traffic/Timetables
- „Validation“-Overlay: fehlerhafte Geometrien/Attribute als Map-Hints (rotes Badge)

**Receiver-Definition**
- Punkt-Receiver, Fassaden-Receiver, Grid-Definition (Bounding boxes, Auflösung, Höhen)
- „Calculation Area“-Konzept (wie viele Profi-Tools es verwenden) als eigenständiges Map-Objekt

**Run-Steuerung**
- Run-Dialog: Standard auswählen (z. B. CNOSSOS EU 2021/1226 vs ältere), Zeitraum (Tag/Abend/Nacht), Receiver-Set, Performance-Optionen
- Run-Monitor (Status, Logs, Fortschritt, Cancel)

**Ergebnisvisualisierung**
- Raster/Heatmap-Layer (Lden/Lnight/LAeq etc.)
- Konturlinien + Labeling
- Punktresultate (Popup-Table), Spektren als Chart (falls norm erlaubt)
- Summenpegel vs. Teilbeiträge (Contribution Breakdown)
- Export-Panel (GeoTIFF/PNG/SVG/CSV/Report packet)

**Reporting**
- Template-basierte Reports (Abschnitte: Eingangsdaten, Normversion, Parameter, Karten, Tabellen, QA-Nachweise)
- Hier lohnt sich, SoundPLAN als Benchmark zu sehen: es betont vordefinierte Layouts und Tabellen-/Grafiktemplates. citeturn5search1turn5search17

### Karten-/Daten-Rendering-Strategie

Du solltest drei Datenkategorien unterscheiden:

- **Basemap**: Vector Tiles (MapLibre) + Styles
- **Modelldaten**: editierbare GeoJSON/Feature-Sets (Sources, Buildings, Barriers)
- **Ergebnisse**:  
  - Raster (serverseitig als Tiles oder als GeoTIFF + Client-Tiling)  
  - Konturen als Vektor-Tiles/GeoJSON  
  - Tabellen als API-JSON

Für Offline/Low-Ops kannst du PMTiles in Betracht ziehen: Single-File Tileset, geeignet für „serverless“ Hosting. citeturn3search3turn3search7

---

## Teststrategie und QS: TDD (Red-Green) + normbasierte Acceptance Suites

### Warum QS hier früh und hart sein muss

Für Lärmsoftware ist QS nicht optional, sondern Teil der Glaubwürdigkeit.  
Belegbar ist etwa, dass es offizielle Testaufgaben zur Überprüfung der korrekten Nachbildung von Regelwerken gibt (z. B. RLS‑19 TEST‑20). citeturn2search2turn2search31  
Auch für BUB-Schiene existieren Testaufgaben, und die EBA beschreibt die normgerechte Umsetzung in Anlehnung an DIN 45687 durch Testaufgaben. citeturn4search16turn4search11  
UBA betont, dass QS im Sinne DIN 45687 auf Werkzeuge wie Testaufgaben, Konformitätserklärungen und Austauschformate setzt und ein kontinuierlicher Prozess ist. citeturn4search15

### Konkretes Test-Pyramidenmodell

**Unit Tests (TDD, Red-Green-Refactor)**
- Domain: Einheiten/Typen, dB-Logik, Band-Summation, Indikator-Aggregation
- Geo: Intersection/Distance, CRS-Handling, Raster indexing
- Standards: Tabellen-Lookups, piecewise Formeln, Grenzfälle

**Property-/Fuzz Tests**
- Geometrie-Algorithmen (robust gegen degenerierte Polygone/NaNs)
- Numerik (Monotonie: mehr Verkehr → nicht weniger Emission, etc.)
- Determinismus bei Parallel Runs

**Golden Tests**
- Kleine Referenzprojekte mit fixierten Inputs → Snapshot von Result-Tabellen/Rastern

**Acceptance Tests (Normtestaufgaben)**
- RLS‑19: TEST‑20 Aufgaben inkl. „Referenzeinstellung“/Konformitätsprozess. citeturn2search31turn2search2  
- BUB-Schiene: EBA-Testaufgaben. citeturn4search16  
- DIN-NALS: „Testaufgaben zur Berechnung von Schallausbreitung im Freien“ als zusätzliche QS-Basis. citeturn4search7

**Frontend-Tests**
- Component Tests (z. B. für Editor-Formulare)
- E2E (Playwright) für Kernflows: import → validate → run → visualize → export  
- Bun kann als Test-Runner genutzt werden (Bun ist explizit Toolkit inkl. Test Runner). citeturn3search18

---

## Phasenplan für die vollständige Umsetzung

Die Phasen sind bewusst fein granuliert, damit du jederzeit „ein nützliches Stück Software“ hast. Jede Phase liefert lauffähige Artefakte und erweitert das System in stabilen, testbaren Schichten.

### Phase Foundation: Repo, CI, Architekturrahmen, TDD-Setup

**Backend**
- Go Monorepo initialisieren, Modulstruktur (Domain/App/Engine/Geo/Standards)
- Cobra-CLI Skeleton (Commands + Config) – Cobra ist dafür Standard. citeturn1search19turn1search3
- Logging/Tracing, Konfig (YAML/JSON), deterministische RNG-Policy (falls nötig)

**Frontend**
- Vite React TS Scaffold; Bun als package manager (Bun + Vite funktionieren out-of-the-box). citeturn3search2turn3search26
- MapLibre Minimal Map View (Hello Map)

**Tests**
- TDD-Workflow definieren (red-green), Coverage-Gates
- Erste Golden-Test Infrastruktur (Snapshot Folder)

**Definition of Done**
- `noise --help` zeigt sinnvolle Kommandostruktur
- `bun test` + `go test ./...` laufen in CI

---

### Phase Project Format: Projektdatei, Szenarien, Versionierung, Import-Grundlagen

**Backend**
- Projektmanifest (Version, CRS, Standards, Szenarien, Artefakte)
- Szenario-Entity + „Run“-Entity (Job-Definition, Parameter, Normversion)
- Einfacher Import: GeoJSON für Quellen/Barrieren/Buildings

**Frontend**
- Projekt-Explorer UI
- Editieren/Anzeigen von Quellen auf der Karte (noch keine Berechnung)

**Tests**
- Roundtrip Serialize/Deserialize
- Validation Tests für Pflichtfelder

---

### Phase Geo Core: Geometrie, Spatial Index, Receiver-Grids, Raster-Container

**Backend**
- CRS-Handling, Distanz/Abstandsgleichungen, Sichtlinien-Helper
- Receiver-Grid Generator (BBox + Auflösung + Höhe)
- Raster-Containerformat + Metadaten (NoData, units, bands)

**Frontend**
- Receiver-Grid Editor (BBox zeichnen, Auflösung wählen)
- Darstellung großer Punktmengen (WebGL layer/tiling)

**Tests**
- Geo-Unit + Fuzz Tests (degenerate cases)
- Raster indexing Golden tests

---

### Phase Engine Skeleton: generische Beitragssummation, Parallelisierung, deterministische Jobs

**Backend**
- Engine-Pipeline:
  1) Lade Modell + Normmodule  
  2) Erzeuge Receiver-Chunks  
  3) Sammle Kandidatenquellen via Spatial Index  
  4) Berechne Contributions  
  5) Aggregiere in Raster/Receiver-Tabellen  
- Parallelisierung (Worker Pool), deterministische Reduktion
- `noise run --standard dummy-freefield` implementieren: Freifeldabnahme als Baseline (nicht normativ, nur technischer Test)

**Frontend**
- Run-Dialog + Run-Monitor (Progress via `noise serve`)

**Tests**
- Determinismus-Test: 1 Worker vs. N Worker → identisches Result
- Performance smoke tests

---

### Phase CNOSSOS EU: Road-Support als erste „echte“ Normschiene

Diese Phase ist oft der MVP-Kern für EU-Perspektive, weil CNOSSOS EU das gemeinsame methodische Dach ist (2015/996, Update 2021/1226). citeturn0search0turn0search1

**Backend**
- Normmodul `cnossos/road`:
  - Emission nach CNOSSOS (Speed, surface corrections etc. – versioniert nach 2021/1226)
  - Ausbreitungskette gemäß Anhang II (für Road-Use Case)
- Indikatoren: Lday/Levening/Lnight und Aggregation zu Lden/Lnight (END-Relevanz). citeturn2search15turn2search7
- Export: Raster (Lden, Lnight), Receiver-Punktlisten

**Frontend**
- Road-Source Editor (Attribute, Templates)
- Ergebnislayer Lden/Lnight (Heatmap) + Legend

**Tests**
- Unit Tests für CNOSSOS-Tabellen/Edge Cases
- Golden-Projekte: kleine synthetische Road-Scenes
- Cross-check mit externen Referenzen (wo vorhanden, ohne IP-Probleme)

---

### Phase CNOSSOS EU Erweiterung: Rail + Industry + (später) Aircraft als Komplettheitspfad

CNOSSOS deckt in der Praxis mehrere Quellklassen ab und wird in kommerzieller Software entsprechend geführt. citeturn0search8turn5search3

**Backend**
- `cnossos/rail` und `cnossos/industry` (Aircraft zunächst als Platzhalter-API + Roadmap)
- Gemeinsame Ausbreitungs-Tools wiederverwenden (diffraction/reflection Komponenten normkonform kapseln)
- Multi-Source Summation: Straße + Schiene + Industrie als Summenpegel (wie es professionelle Tools anbieten) citeturn5search2turn5search35

**Frontend**
- Multi-Noise-Type Layer toggles
- Contribution Breakdown pro Receiver

**Tests**
- Golden-Suites je Quellklasse
- Regression suites bei Standard-Updates (2015/996 → 2021/1226 „Profiles“)

---

### Phase Reporting & Exposition: strategische Karten, Tabellen, EEA-nahe Auswertungen

**Backend**
- Report Generator v1: Markdown/PDF via Template (oder HTML→PDF)
- Exposition: Aggregation von Population/Buildings in dB-Bändern für Lden/Lnight (Report Output). Der EEA-Datenhub beschreibt genau solche 5‑dB-Bänder für Lden/Lnight. citeturn2search1turn2search30

**Frontend**
- Report Preview (HTML/PDF)
- Export Center

**Tests**
- Snapshot-Tests von Report-Templates
- „Known totals“ Tests für Expositionssummen

---

### Phase Germany Mapping Track: BUB/BUF/BEB als DE-Kartierungsmodus

Deutschland nutzt für Kartierung spezifische Methoden/Anlagen: Bekanntmachungen/Anhänge umfassen BUB, BUF und BEB (inkl. Datenbankbezug). citeturn4search12turn4search0  
BUB berechnet für Kartierung u. a. Lden/Lnight und ist explizit für 34. BImSchV relevant, aber nicht für 16. BImSchV/TA Lärm. citeturn4search0

**Backend**
- `bub/` Normmodule (Road/Rail/Industry innerhalb BUB-Logik)
- `beb/` (Belastetenzahlen) als eigener Schritt/Modul, weil es methodisch/berichtlich bedeutsam ist (Roadmap: erst vereinfachtes Exposure, dann echtes BEB)
- Importer für BUB-D Datensätze (versioniert, auditierbar)

**Frontend**
- „Deutschland Kartierung“ Standardauswahl
- Eingabemasken entsprechend BUB Parametern

**Tests**
- EBA-/BUB-Schiene Testaufgaben als Acceptance Tests. citeturn4search16turn4search11

---

### Phase Germany Project Track: RLS‑19 + Schall 03 + Industrielärm-Schiene (Vollausbau für Genehmigung/Planung)

RLS‑19 hat offizielle **TEST‑20** Aufgaben zur Programmprüfung; dieser Pfad ist ideal, um „Konformität“ als Feature im Produkt zu verankern. citeturn2search31turn2search2  
Schall 03 ist verordnungsnah (16. BImSchV Anlage 2) und arbeitet mit Oktavbändern 63 Hz bis 8 kHz. citeturn4search5turn4search1

**Backend**
- `rls19/` Normmodul (Road, Emission+Ausbreitung nach RLS‑19)
- `schall03/` Normmodul (Rail)
- Industrielärm: Schnittstelle zu ISO 9613‑2 Modul als internationaler Standardbaustein (Engineering-Methode zur Außenpropagation). citeturn0search2turn0search18
- „Standard Switch“ pro Szenario: Kartierung (BUB/CNOSSOS) vs Projekt (RLS‑19/Schall 03)

**Frontend**
- Standard-/Rechtskontext bewusst sichtbar (Label: „Kartierung“ vs „Planung“)
- Migrationsassistenten (z. B. Parameter-Änderungen zwischen Standards; als UX-Feature)

**Tests**
- RLS‑19 TEST‑20 vollständig in CI; Toleranzregeln pro Testfall. citeturn2search31
- DIN-/UBA-QS-Checkliste in Repo als Dokumentation (Policy + Tooling). citeturn4search15turn4search7

---

### Phase Performance & Skalierung: City-Scale Runs, Tile-basierte Ergebnisse, Benchmarks

**Backend**
- Tiled Compute Pipeline, disk-backed caches
- Optionale „GPU/FFT“-beschleunigte Rasterpostprozesse (algo-fft)
- „calc lists“ / Batch Runs (Scenario Sweep) – wichtig für Beratungspraxis

**Frontend**
- Progressive Rendering: erst grob (low-res), dann refine
- Compare-Mode (Szenario A vs B, ΔL)

**Tests**
- Benchmark-Suite + Regression-Tracking (nicht nur speed, auch numeric drift)

---

### Phase Desktop Packaging: Wails als „Closer Coupling“-Option

Wails beschreibt genau deinen Wunsch: Go + Web-Frontend zu einem Desktop-Binary bündeln, statt einen Server mit Browser zu betreiben. citeturn3search12turn3search4  
Wails v3 ist (Stand heute) als **ALPHA** ausgewiesen; das sollte in deiner Roadmap als Risiko markiert werden. citeturn3search4

**Backend**
- `noise serve` so bauen, dass er „in-proc“ ohne Netzwerkport laufen kann (Vorbereitung für Wails IPC)
- Asset embedding (UI build wird in Go-Binary gebündelt)

**Frontend**
- Build-Targets: `web` (normal), `wails` (angepasste Base-Path, IPC transport)

**Tests**
- Smoke Tests für Desktop Build (CI matrix)

---

### Phase Ökosystem & Erweiterbarkeit: Plugin-SDK, Standard-Versionierung, Update-Strategie

**Backend**
- Standard-Versionierung (z. B. CNOSSOS 2015/996 vs 2021/1226 Profiles) – weil 2021/1226 Anhang II explizit Anpassungen/Verbesserungen enthält. citeturn0search1turn0search25
- Plugin-SDK: neue Standards als Go-Modul + registrierter Adapter
- Konfig/Provenance: jeder Run schreibt „Norm-ID + Version + Parameter + Datenquellen“ in ein auditierbares Manifest

**Frontend**
- Plugin-aware Forms (Schema generiert aus Backend Metadaten)
- „Compliance View“: zeigt Normversion und welche Test-Suites bestanden wurden

---

## Ergebnis: Was du nach diesem Plan wirklich „komplett“ abdeckst

Wenn du alle Phasen durchläufst, hast du am Ende ein System, das:

- EU-END-konforme Indikatoren und CNOSSOS-EU Berechnungspfad als Kern unterstützt (inkl. Updates über 2021/1226). citeturn0search0turn0search1turn2search15  
- Deutschland-spezifische Kartierungsmethoden (BUB/BUF/BEB) als separaten Modus abbilden kann und die Zwecktrennung zu 16. BImSchV/TA Lärm sauber hält. citeturn4search0turn4search12  
- Deutschland-spezifische Projektstandards (RLS‑19, Schall 03) mit offiziellen Testaufgaben/QA (TEST‑20 etc.) in CI/Release-Prozess integriert. citeturn2search31turn4search5turn4search16  
- Eine moderne Web-GUI besitzt, die auf MapLibre GL JS (WebGL/Vector-Tiles) basiert. citeturn3search1turn3search33  
- Zunächst als CLI (Cobra) automatisierbar ist und später als Desktop-App per Wails gebündelt werden kann. citeturn1search19turn3search12  
- „Big project“-fähig ist, weil Compute, Datenmodell und Standards entkoppelt sind und QS „first class“ ist (DIN 45687-orientierte Testaufgaben/Prozesse). citeturn4search15turn4search7turn2search31