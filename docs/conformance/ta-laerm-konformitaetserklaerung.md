# TA Lärm — Konformitätserklärung

## Modul

`backend/internal/assessment/talaerm/`

## Normative Grundlage

- TA Lärm vom 26.08.1998 (GMBl. Nr. 26/1998 S. 503)
- Geändert durch Verwaltungsvorschrift vom 01.06.2017 (BAnz. AT 08.06.2017 B5)
- LAI-Hinweise zur Auslegung der TA Lärm, Stand 24.02.2023

## Implementierter Umfang

### Immissionsrichtwerte (Nr. 6)

- [x] Nr. 6.1 — Immissionsrichtwerte außerhalb von Gebäuden (Kategorien a–g, inkl. 2017-Ergänzung "urbane Gebiete")
- [x] Nr. 6.2 — Immissionsrichtwerte innerhalb von Gebäuden (35/25 dB(A))
- [x] Nr. 6.3 — Immissionsrichtwerte für seltene Ereignisse (70/55 dB(A))
- [x] Nr. 6.4 — Beurteilungszeiten (Tag 06–22, Nacht 22–06, lauteste Nachtstunde)
- [x] Nr. 6.5 — Zuschlag für Tageszeiten mit erhöhter Empfindlichkeit (6 dB, Kategorien d–f)
- [x] Nr. 6.7 — Gemengelagen (Zwischenwerte, begrenzt auf Kern-/Dorf-/Mischgebiet-Niveau)
- [x] Nr. 6.9 — Messabschlag bei Überwachungsmessungen (−3 dB)

### Beurteilungspegel (Anhang)

- [x] Gleichung G1 — Gesamtbelastung (energetische Summe Vor- + Zusatzbelastung)
- [x] Gleichung G2 — Beurteilungspegel Lr aus Teilzeiten mit Zuschlägen KT, KI, KR, Cmet
- [x] Gleichung G6 — Impulshaltigkeit KI = LAFTeq − LAeq (diskretisiert 0/3/6 dB)

### Bewertungslogik

- [x] Regelfallprüfung (Nr. 3.2.1) — Vergleich Lr gegen Richtwert
- [x] Relevanzprüfung (Nr. 3.2.1 Abs. 2) — Irrelevanzkriterium 6 dB
- [x] Spitzenpegelkriterium — Richtwert + 30 dB(A) tags, + 20 dB(A) nachts
- [x] Vorbelastung / Zusatzbelastung / Gesamtbelastung (Nr. 2.4)

### Berichtswesen

- [x] Strukturierte JSON-Ausgabe (ExportEnvelope)
- [x] Deutsche Bewertungstexte (Gutachten-Textbausteine)

## Bekannte Einschränkungen

- Nr. 7.1 Ausnahmeregelung für Notsituationen — nicht implementiert (Verwaltungsentscheidung)
- Nr. 7.2 Seltene Ereignisse — Richtwerte implementiert, Zulassungsentscheidung obliegt dem Gutachter
- Nr. 7.3 Tieffrequente Geräusche — nicht implementiert (erfordert separates DIN-45680-Verfahren)
- Nr. 7.4 Verkehrsgeräusche auf dem Betriebsgrundstück — Eingabe obliegt dem Anwender
- Anhang A.2 Schallausbreitungsrechnung — nicht Teil dieses Moduls (separates ISO-9613-2-Modul)
- Anhang A.3 Messverfahren — nicht implementiert (Messauswertung nicht im Scope)
- Nr. 3.2.2 Ergänzende Prüfung im Sonderfall — nicht automatisierbar

## Validierung

Golden-Test-Szenarien in `backend/internal/assessment/talaerm/testdata/`:

- `full_assessment.golden.json` — 8 Empfänger, alle Bewertungspfade

## Stand

Erstellt: 2026-03-28
