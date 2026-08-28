# CNOSSOS-EU, BUB und BUF — Umfangserklärung

**Dies ist keine Konformitätserklärung.** Die übrigen Dokumente in diesem Verzeichnis
(`iso9613-`, `rls19-`, `schall03-` und `ta-laerm-konformitaetserklaerung.md`) erklären, in welchem
Umfang ein Modul die von ihm benannte Norm umsetzt. Dieses Dokument erklärt das Gegenteil: die hier
beschriebenen Module setzen die von ihnen benannten Vorschriften **nicht** um. Sie enthalten keinen
einzigen Koeffizienten aus Richtlinie (EU) 2015/996, aus den deutschen Berechnungsmethoden BUB und
BUF oder aus ECAC Doc 29.

Die Module tragen die Evidenzstufe `scaffold` (Gerüst). Sie existieren, um die Datenpfade des
Projekts — Import, Schema, Lauf-Pipeline, Export, Provenienz — deterministisch zu betreiben, nicht
um Pegel zu liefern, auf die sich jemand berufen könnte. Ihre Ausgaben sind für keine Beurteilung,
kein Gutachten und keine Vorlage bei einer Behörde geeignet.

## Modul

| Registry-ID        | Paketpfad                                      | Evidenzstufe |
| ------------------ | ---------------------------------------------- | ------------ |
| `cnossos-road`     | `backend/internal/standards/cnossos/road/`     | `scaffold`   |
| `cnossos-rail`     | `backend/internal/standards/cnossos/rail/`     | `scaffold`   |
| `cnossos-industry` | `backend/internal/standards/cnossos/industry/` | `scaffold`   |
| `cnossos-aircraft` | `backend/internal/standards/cnossos/aircraft/` | `scaffold`   |
| `bub-road`         | `backend/internal/standards/bub/road/`         | `scaffold`   |
| `bub-rail`         | `backend/internal/standards/bub/rail/`         | `scaffold`   |
| `bub-industry`     | `backend/internal/standards/bub/industry/`     | `scaffold`   |
| `buf-aircraft`     | `backend/internal/standards/buf/aircraft/`     | `scaffold`   |

Das nachgelagerte Modul `beb-exposure` (Evidenzstufe `preview`) hat eine eigene Erklärung:
[`beb-umfangserklaerung.md`](beb-umfangserklaerung.md). Es bezieht seine Pegel aus `bub-road` bzw.
`buf-aircraft` und ist damit von dieser Erklärung unmittelbar betroffen.

## Normative Grundlage

Die Vorschriften, deren Namen diese Module führen:

- **Richtlinie (EU) 2015/996** der Kommission vom 19. Mai 2015 zur Festlegung gemeinsamer
  Lärmbewertungsmethoden gemäß der Richtlinie 2002/49/EG, **Anhang II** — Straßenverkehr,
  Schienenverkehr, Industrie und Gewerbe.
- **JRC Reference Report, Common Noise Assessment Methods in Europe (CNOSSOS-EU)** — der
  Berichtsband, der die Koeffizientensätze zu Anhang II trägt.
- **BUB** und **BUF** — die deutschen Berechnungsmethoden zur Umsetzung von Richtlinie (EU)
  2015/996, veröffentlicht durch das Umweltbundesamt (`cnossos-de_anlage_1-bub-2021`,
  `cnossos-de_anlage_2-buf-2021`) samt der zugehörigen Datenbankanlagen
  (`cnossos-de_anlage_4-bub-d-2021`, `cnossos-de_anlage_5-bufd-2021`).
- **ECAC Doc 29**, Report on Standard Method of Computing Noise Contours around Civil Airports —
  das Verfahren, auf das die Umgebungslärmrichtlinie für Fluglärm verweist (siehe Abschnitt
  „Luftverkehr“).

**Keines dieser Dokumente und keiner der zugehörigen Koeffizientensätze liegt dem Projekt vor.**
Die Beschaffung steht im Forschungs-Backlog von `PLAN.md`; solange sie aussteht, ist eine
Implementierung dieser Verfahren nicht möglich, und die vorhandenen Module bleiben Gerüste.

## Implementierter Umfang

### Vorhanden

- [x] Typisierte Quellenschemata mit Validierung: Straßen- und Schienenlinien, Industrie-Punkt- und
      Flächenquellen, Flugspuren als 3D-Polylinien
- [x] Import aus normalisiertem GeoJSON, Laufparameter als Vorbelegung je Feature
- [x] Deterministische Emissionsrechnung je Zeitscheibe (Tag/Abend/Nacht) aus benannten
      Einzelkomponenten
- [x] Linienquellenzerlegung in Teilstücke (Schrittweite höchstens 10 m, Mittelpunktauswertung,
      Längengewichtung `10·lg L`)
- [x] Entfernungsabhängige Dämpfung: `A_div = 20·lg d + 11 dB`
- [x] Energetische Summation über Quellen und Teilstücke in fester Reihenfolge
- [x] `Lden` nach Anhang I der Richtlinie 2002/49/EG (12/4/8 Stunden, +5 dB abends, +10 dB nachts) —
      **die einzige Formel in diesen Modulen, die aus einem normativen Text stammt**
- [x] Empfängertabellen (CSV/JSON), Raster-Export, Provenienzmetadaten, Golden-Tests

### Nicht vorhanden

- [ ] **Oktavbänder.** Sämtliche Module rechnen einen einzelnen Breitbandwert. Es gibt im gesamten
      Teilbaum keine Frequenzstützstelle (63 Hz – 8 kHz) und keine A-Bewertung je Band.
- [ ] **`A_R`/`B_R` (Rollgeräusch) und `A_P`/`B_P` (Antriebsgeräusch)** je Fahrzeugklasse und
      Oktavband — die Kerntabellen des Straßenverfahrens von Anhang II
- [ ] **Rauheitsspektren** (Schienen- und Radrauheit) und **Kontaktfilter** des Schienenverfahrens
- [ ] **`A_gr,H` / `A_gr,F`** — getrennte Bodendämpfung für homogene und für Mitwindbedingungen
- [ ] **`A_diff`** mit Fresnel-Gewichtung und Korrekturterm `C_f`
- [ ] **Wahrscheinlichkeit günstiger Ausbreitungsbedingungen `p`** und die daraus gebildete
      Langzeitmittelung aus homogenem und Mitwind-Fall
- [ ] **NPD-Kurven** (Noise-Power-Distance) für Fluglärm
- [ ] **ECAC-Doc-29-Flugprofile** und Segmentierung der Flugbahn
- [ ] Richtwirkung `ΔL_W,dir`, Quellenhöhenverteilung, Reflexionen, seitliche Beugung, Bewuchs

### Was die Module stattdessen rechnen

Diese Angaben sind am Quellcode geprüft, nicht aus den Phasen-Notizen übernommen:

- **Emission Straße** (`cnossos/road/emission.go`): frei gewählte Basispegel je Fahrzeugklasse —
  35,0 dB (leicht), 39,0 dB (mittel), 42,0 dB (schwer), 37,0 dB (Zweiräder) — zuzüglich
  `10·lg Q` und additiver Korrekturen für Straßenkategorie, Geschwindigkeit, Belag, Steigung,
  Knotenpunkt, Temperatur und Spikeanteil. Die Korrekturwerte sind erfunden. Der Kommentar
  „Values must match the CNOSSOS road baseline exactly“ über `roadCategoryCorrections` meint die
  hausinterne Baseline, nicht CNOSSOS-EU.
- **Emission Schiene** (`cnossos/rail/emission.go`): vier erfundene Basispegel — 43,0 dB (Rollen),
  38,0 dB (Traktion), 35,0 dB (Bremsen), 30,0 dB (Infrastruktur) — je zuzüglich `10·lg Q` und
  logarithmischer Geschwindigkeitsterme.
- **Emission Industrie** (`cnossos/industry/emission.go`): der vom Nutzer gelieferte
  Schallleistungspegel zuzüglich erfundener Zuschläge für Kategorie, Einhausung, Höhe, Ton- und
  Impulshaltigkeit sowie `10·lg A` bei Flächenquellen.
- **Emission Luftverkehr** (`cnossos/aircraft/emission.go`): ein Referenz-Schallleistungspegel aus
  den Laufparametern (Vorbelegung 108 dB bzw. 110 dB bei `buf-aircraft`) zuzüglich erfundener
  Zuschläge für Flugzeugklasse, Betriebsart, Triebwerkszustand, Verfahren und Schubmodus.
- **Ausbreitung** (alle Module): `A_div` geometrisch, `A_atm` als ein einziger frequenzunabhängiger
  Skalar (Vorbelegung 0,7 dB/km) und die Bodendämpfung als **feste, vom Nutzer gesetzte Konstante**
  (Vorbelegung 0,8–1,5 dB je Modul). Eine Abschirmung gibt es nur bei `cnossos-road`
  (`barrier_attenuation_db`) und `cnossos-industry` (`screening_attenuation_db`), in beiden Fällen
  als Konstante mit **Vorbelegung 0 dB**; `cnossos-rail`, `cnossos-aircraft` und `buf-aircraft`
  haben überhaupt keinen Abschirmterm. Es findet in keinem Modul eine Geometrieauswertung statt:
  Gebäude, Wälle und Gelände wirken sich auf keinen Pegel aus.

## Modulbeziehungen

Am Quellcode geprüft (Stand 28. August 2026):

- **`bub-road` ist ein umparametrierter Klon von `cnossos/road`.** Gleiche Dateiaufteilung, gleiche
  Rechenstruktur, ebenfalls erfundene Koeffizienten. Abweichungen: das Schemafeld `road_category`
  heißt `road_function_class` und trägt drei statt fünf Klassen; die Ausbreitungsterme
  `barrier_attenuation_db` sind durch `urban_canyon_db` und `intersection_density_per_km` ersetzt;
  die Bodendämpfung ist mit 1,2 dB statt 1,5 dB vorbelegt.
- **`bub-rail` und `bub-industry` sind reine Alias-Pakete.** Je eine Datei (`rail.go`,
  `industry.go`) mit Typ-Aliassen auf `cnossos/rail` bzw. `cnossos/industry`, weiterreichenden
  Funktionshüllen, einem eigenen Deskriptor und einer eigenen Export-Routine. **Akustische
  Rechenlogik enthalten sie keine**: Emission, Ausbreitung und Indikatoren stammen unverändert aus
  dem jeweiligen `cnossos/*`-Paket. Beide sind über die CLI nicht erreichbar (siehe unten).
- **`buf-aircraft` ist eine Beinahe-Kopie von `cnossos/aircraft`.** `compute.go` und `emission.go`
  sind Byte für Byte identisch; die übrigen vier Dateien unterscheiden sich nur in Bezeichnern,
  Deskriptor-Metadaten und Vorbelegungen. Der einzige Unterschied im Rechenweg ist die Konstante
  `LateralDirectivityDB` (0 gegenüber 1,0 dB). Hinzu kommen sieben geänderte Parametervorbelegungen
  des Deskriptors (unter anderem `reference_power_level_db` 108 → 110 dB), die sich auf importierte
  Quellen auswirken — die Beschreibung „bis auf eine Konstante identisch“ trifft also den Rechenweg,
  nicht den vollständigen Unterschied.

## Luftverkehr: CNOSSOS-EU enthält kein Fluglärmverfahren

Richtlinie (EU) 2015/996, Anhang II, deckt **Straßenverkehr, Schienenverkehr sowie Industrie und
Gewerbe** ab. Ein Berechnungsverfahren für Fluglärm ist darin nicht enthalten; für Fluglärm verweist
die Umgebungslärmrichtlinie auf **ECAC Doc 29**. Die Modul-ID `cnossos-aircraft` benennt damit ein
Verfahren, das die genannte Vorschrift nicht enthält. `buf-aircraft` benennt zwar die richtige
deutsche Vorschrift, setzt sie aber ebenso wenig um.

**Projektentscheidung:** Standard-IDs und Paketpfade bleiben unverändert. Standard-IDs stehen in den
`.noise/project.json`-Manifesten bestehender Projekte; eine Umbenennung würde das Projektformat
brechen. Die Offenlegung wird stattdessen an vier Stellen getragen:

1. der Evidenzstufe `scaffold` am Deskriptor, die in Provenienz, Bericht und API mitläuft,
2. der ausdrücklichen `--experimental`-Zustimmung, ohne die `aconiq run` diese Module nicht
   ausführt,
3. den Beschreibungstexten der Module, die keine Konformität mehr behaupten,
4. diesem Dokument.

## Zulässige Verwendung

Wofür diese Module taugen:

- deterministische Erprobung der Lauf-Pipeline von Import bis Export,
- Regressionsabsicherung von Schema, Provenienz und Ausgabeformaten,
- Lastversuche und Benchmarks mit realistisch geformten, aber bedeutungslosen Zahlen.

Wofür sie nicht taugen — abschließend:

- jede Beurteilung nach Umgebungslärmrichtlinie, BImSchG oder Landesrecht,
- jedes Gutachten, jede Stellungnahme, jede Vorlage bei einer Behörde,
- jeder Vergleich mit Ergebnissen anderer Programme,
- jede Aussage über Betroffenenzahlen, Lärmkarten oder Schutzansprüche.

## Bekannte Einschränkungen

- Kein Frequenzbezug: ein Breitbandwert je Zeitscheibe, keine Oktavbänder, keine A-Bewertung je
  Band.
- Keine Ausbreitungsgeometrie: Boden- und Abschirmdämpfung sind Konstanten aus den Laufparametern.
  Gebäude, Wälle und Gelände bleiben ohne Wirkung.
- Keine meteorologische Langzeitmittelung; es gibt keinen Mitwind-/Homogenfall und kein `p`.
- Keine Spiegelquellen-Reflexionen, keine seitliche Beugung, keine Richtwirkung. `cnossos-industry`
  kennt lediglich einen konstanten Zuschlag `facade_reflection_db` (Vorbelegung 0 dB), der ohne
  jede Geometrie auf jeden Pfad wirkt.
- Ausgegebene Werte sind unformatierte `float64`; ein nutzerseitiger Rundungsvertrag ist für diese
  Module nicht definiert.
- Die Modellversionen tragen interne Sprintnummern (`phase10-preview-v2` … `phase16-preview-v2`).
  Deren Umbenennung ist in `PLAN.md` offen.

## Erreichbarkeit über die CLI

Die Registry (`backend/internal/standards/registry.go`) meldet alle acht Module an. Die
Lauf-Pipeline (`backend/internal/app/cli/run_pipeline.go`) kennt jedoch nur sechs davon:

| Modul                                                                  | Über `aconiq run` erreichbar |
| ---------------------------------------------------------------------- | ---------------------------- |
| `cnossos-road`, `cnossos-rail`, `cnossos-industry`, `cnossos-aircraft` | ja                           |
| `bub-road`, `buf-aircraft`                                             | ja                           |
| `bub-rail`, `bub-industry`                                             | **nein**                     |

`bub-rail` und `bub-industry` sind angemeldet, aber nicht verdrahtet; ein Lauf endet im
`default:`-Zweig mit „standard is registered but not wired in run pipeline yet“. Dass die Registry
etwas anbietet, was der Ausführer nicht ausführen kann, ist als offener Punkt in `PLAN.md`
vermerkt.

## Koeffizienten und Datenpakete

Das Repository bündelt **keine** normativen Daten zu diesen Vorschriften: keinen Koeffizientensatz
aus dem JRC-Berichtsband, keine BUB-/BUF-Datenbankanlage, keine NPD-Kurve, kein Flugprofil. Es
werden auch keine amtlichen Rohdatensätze mitgeliefert; die Rechtelage dazu ist in
`docs/research/bub-dataset-availability-and-rights.md` festgehalten.

Eine echte Implementierung setzt voraus, dass die Koeffizientensätze des JRC-Berichtsbands sowie die
BUB-/BUF-Datenbankanlagen beschafft und ihre Weitergaberechte geklärt werden. Beides steht im
Forschungs-Backlog von `PLAN.md`; ob echte Implementierungen überhaupt auf die Roadmap kommen, ist
dort ausdrücklich noch offen.

## Validierung

Vorhanden sind ausschließlich selbst erzeugte Regressionsfixtures:

- Akzeptanzszenarien unter `backend/internal/qa/acceptance/testdata/` für `cnossos-road`,
  `cnossos-rail`, `cnossos-industry`, `cnossos-aircraft`, `bub-road` und `buf-aircraft`,
- Modul-Unittests je Paket für Schemavalidierung, Emission, Ausbreitung, Indikatoren, Export,
  Deskriptor und Provenienz,
- CLI-End-to-End-Abdeckung unter `backend/internal/app/cli/testdata/`.

Diese Fixtures sichern ab, dass das Gerüst deterministisch dieselben Zahlen liefert. Sie sagen
nichts darüber aus, ob diese Zahlen richtig sind — es gibt keine normative Referenz, gegen die sie
geprüft werden könnten. Für `bub-rail` und `bub-industry` existieren keine Akzeptanzfixtures.

Die öffentlich attribuierbaren Referenzsummen unter `docs/research/*-public-reference-totals.md`
sind Größenordnungsanker aus veröffentlichten Kartierungen Dritter. Sie sind keine Testfälle und
kein Nachweis.

## Stand

Erstellt: 2026-08-28
Zuletzt geprüft: 2026-08-28
Evidenzstufe: `scaffold` (alle acht Module)
Konformitätsgrenze: keine — dieses Dokument erklärt ausdrücklich, dass keine besteht
