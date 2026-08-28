# BEB Belastetenzahlen — Umfangserklärung

**Dies ist keine Konformitätserklärung.** Die übrigen Dokumente in diesem Verzeichnis mit dem Suffix
`-konformitaetserklaerung.md` erklären, in welchem Umfang ein Modul die von ihm benannte Norm
umsetzt. Dieses Dokument beschreibt ein Modul der Evidenzstufe `preview`: seine eigene
Aggregationslogik ist nachvollziehbar und geprüft, aber die Pegel, auf denen sie aufsetzt, stammen
aus Gerüstmodulen ohne normative Koeffizienten.

## Modul

| Registry-ID    | Paketpfad                                  | Evidenzstufe |
| -------------- | ------------------------------------------ | ------------ |
| `beb-exposure` | `backend/internal/standards/beb/exposure/` | `preview`    |

## Normative Grundlage

- **BEB** — Berechnungsmethode zur Ermittlung der Belastetenzahlen durch Umgebungslärm,
  veröffentlicht durch das Umweltbundesamt (`cnossos-de_anlage_3-beb-2021`), samt der
  veröffentlichten Testaufgaben.
- Vorgelagert: Richtlinie (EU) 2015/996 in der deutschen Umsetzung BUB bzw. BUF.

Weder das BEB-Dokument noch seine Testaufgaben liegen dem Projekt vor; die Beschaffung steht im
Forschungs-Backlog von `PLAN.md`.

## Implementierter Umfang

### Vorhanden

- [x] Gebäudeweise Auswertung aus Grundrisspolygonen mit Schwerpunkt- oder Fassadenauswertung
      (`facade_evaluation_mode`: `centroid` bzw. `max_facade`)
- [x] Belegungsschätzung aus Gebäudehöhe → Geschosszahl → Wohneinheiten → Personen, mit Vorrang
      expliziter Feature-Angaben (`occupancy_mode`: `prefer_feature_overrides` bzw.
      `height_derived`)
- [x] Schwellenauswertung für `Lden` (Vorbelegung 55 dB) und `Lnight` (Vorbelegung 50 dB)
- [x] 5-dB-Bänder für die Summenauswertung: `Lden` ab 55/60/65/70/75 dB, `Lnight` ab
      50/55/60/65/70 dB
- [x] Deterministische Aggregation zu Gebäudetabellen, Summenkennzahlen und Summenraster
- [x] Indikatoren `Lden`, `Lnight`, `estimated_dwellings`, `estimated_persons`,
      `affected_dwellings_lden`, `affected_persons_lden`, `affected_dwellings_lnight`,
      `affected_persons_lnight`

### Der entscheidende Vorbehalt

Die Pegel, die das Modul aggregiert, berechnet es nicht selbst. `beb-exposure` ruft
(`compute.go`) ausschließlich

- `bub-road` (`internal/standards/bub/road`) oder
- `buf-aircraft` (`internal/standards/buf/aircraft`)

auf, ausgewählt über den Laufparameter `upstream_mapping_standard`. **Beide Vorlieferanten tragen
die Evidenzstufe `scaffold`**: keine normativen Koeffizienten, erfundene Basispegel, keine
Oktavbänder, keine Ausbreitungsgeometrie. Siehe
[`cnossos-umfangserklaerung.md`](cnossos-umfangserklaerung.md).

Damit gilt: Zähl- und Bandlogik sind belastbar, die gezählten Pegel sind es nicht. Jede ausgegebene
Betroffenenzahl ist genau so gut wie ihre Eingangspegel, also nicht belastbar.

### Nicht vorhanden

- [ ] Die BEB-Vorschriften selbst — Zuordnungsregeln für Wohneinheiten und Einwohner, Behandlung von
      Misch- und Sondernutzungen, amtliche Bezugsdaten
- [ ] Andere Nutzungsarten als `residential` (das Schema lässt nur diese eine zu)
- [ ] Auswertung gegen amtliche Gebäude- und Bevölkerungsdaten; die Rechtelage dazu ist in
      `docs/research/beb-dataset-requirements-and-rights.md` festgehalten
- [ ] Anschluss an ein normatives Ausbreitungsmodul

## Zulässige Verwendung

Geeignet für die deterministische Erprobung der Aggregations- und Exportkette sowie für
Regressionsabsicherung. **Nicht geeignet** für Betroffenenzahlen in Lärmaktionsplänen, für
Berichtspflichten nach der Umgebungslärmrichtlinie oder für irgendeine behördenseitige Verwendung.

## Bekannte Einschränkungen

- Belastetenzahlen erben die Evidenzstufe ihrer Eingangspegel; die Ausgabe ist trotz
  `preview`-Einstufung des Moduls nicht verwendbar, solange kein normativer Vorlieferant existiert.
- Die Belegungsschätzung ist eine lineare Ableitung aus Höhe, Geschosshöhe, Wohneinheiten je
  Geschoss und Personen je Wohneinheit (Vorbelegung 2,2) — eine Konvention des Projekts, keine
  Vorgabe aus BEB.
- Die Fassadenauswertung nutzt Ersatzempfänger an den Polygonstützpunkten in fester Höhe
  (`facade_receiver_height_m`, Vorbelegung 4 m), nicht die in BEB vorgesehene Fassadenzuordnung.
- Ausgegebene Werte sind unformatierte `float64`; `ReportingPrecisionCount` dokumentiert lediglich
  die beabsichtigte Ausgabegenauigkeit.
- Die Modellversion trägt eine interne Sprintnummer (`phase16-preview-v2`); deren Umbenennung ist in
  `PLAN.md` offen.

## Validierung

- Akzeptanzszenarien `building_exposure` und `building_exposure_contextual` unter
  `backend/internal/qa/acceptance/testdata/beb-exposure/`
- Modul-Unittests für Schemavalidierung, Belegungsschätzung, Fassadenauswahl, Schwellen- und
  Bandlogik, Export und Provenienz

Es handelt sich um selbst erzeugte Regressionsfixtures. Sie sichern Determinismus, nicht Richtigkeit;
die veröffentlichten BEB-Testaufgaben sind nicht abgebildet.

## Stand

Erstellt: 2026-08-28
Zuletzt geprüft: 2026-08-28
Evidenzstufe: `preview`
Konformitätsgrenze: keine — dieses Dokument erklärt ausdrücklich, dass keine besteht
