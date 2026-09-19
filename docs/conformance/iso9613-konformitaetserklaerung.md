# ISO 9613-2 — Konformitätserklärung

## Modul

`backend/internal/standards/iso9613/`

## Normative Grundlage

- ISO 9613-2:1996, Acoustics — Attenuation of sound during propagation outdoors — Part 2: General method of calculation
- Referenzdokument in `interoperability/ISO9613-2/`

ISO 9613-2:2024 (Second edition) liegt als Vorabansicht vor, ist aber nicht Implementierungsziel. Die 1996-Ausgabe ist die normative Referenz für TA Lärm.

## Implementierter Umfang

### Grundgleichungen (Abschnitt 6)

- [x] Gl. 1 — Äquivalenter A-bewerteter Dauerschalldruckpegel L_AT
- [x] Gl. 2 — Äquivalenter Oktavband-Dauerschalldruckpegel L_fT(DW) bei Mitwind
- [x] Gl. 3 — Oktavband-Immissionspegel: L_fT(DW) = L_W + D_c − A
- [x] Gl. 4 — Gesamtdämpfung: A = A_div + A_atm + A_gr + A_bar + A_misc
- [x] Gl. 5 — Energetische A-bewertete Summation über Quellen und Oktavbänder
- [x] Gl. 6 — Langzeit-Mittelungspegel: L_AT(LT) = L_AT(DW) − C_met, wobei C_met gemäß Abschnitt 8 je Quelle-Empfänger-Pfad vor der energetischen Summation angewandt wird

### Quellenmodell (Abschnitt 4)

- [x] Punktquellen mit Oktavband-Schallleistungspegeln
- [x] Einzelwert-Fallback (A-bewerteter Gesamtpegel → Dämpfungsterme bei 500 Hz, Anmerkung 1): ein einzelnes Band, ohne erneute A-Bewertung
- [x] Richtwirkungskorrektur D_c
- [ ] Linienquellen (Zerlegung in Punktquellen, Abschnitt 4)
- [ ] Flächenquellen (Zerlegung in Punktquellen, Abschnitt 4)

### Geometrische Ausbreitungsdämpfung A_div (Abschnitt 7.1)

- [x] Gl. 7 — A_div = 20·lg(d/d_0) + 11 dB

### Luftabsorption A_atm (Abschnitt 7.2)

- [x] Gl. 8 — A_atm = α·d/1000
- [x] Absorptionskoeffizient α analytisch nach ISO 9613-1 berechnet — klassische Absorption
      sowie O₂- und N₂-Relaxation — für jede Temperatur/Feuchte-Kombination, ausgewertet beim
      Referenzluftdruck 101,325 kPa
- [x] Tabelle 2 — die sechs tabellierten Referenzbedingungen dienen als Prüforakel, nicht als
      Rechenweg: die Berechnung weicht bis 1 kHz um ≤ 0,05 dB/km und bei 2–8 kHz um ≤ 1,4 %
      von der Tabelle ab, was der Rundung der Tabelle entspricht

### Bodendämpfung A_gr (Abschnitt 7.3)

- [x] Gl. 9 — Drei-Regionen-Modell: A_gr = A_s + A_r + A_m
- [x] Tabelle 3 — Frequenzabhängige Ausdrücke mit Funktionen a'(h), b'(h), c'(h), d'(h)
- [x] Gewichtungsfaktor q für die Mittelregion
- [x] Gl. 10 — Vereinfachtes Verfahren für A-bewertete Pegel
- [ ] Gl. 11 — D_Ω Nahfeldkorrektur bei vereinfachtem Verfahren

### Abschirmung A_bar (Abschnitt 7.4)

- [x] Gl. 12 — A_bar = D_z − A_gr ≥ 0; bei freier Sichtverbindung (z ≤ 0) ist A_bar = 0 und A_gr bleibt in Gl. 4 wirksam
- [x] Gl. 14 — D_z = 10·lg[3 + (C_2/λ)·C_3·z·K_met]
- [x] Gl. 15 — C_3 für Doppelbeugung
- [x] Gl. 16 — Wegdifferenz z bei Einfachbeugung
- [x] Gl. 17 — Wegdifferenz z bei Doppelbeugung
- [x] Abschnitt 7.4 — negatives Vorzeichen von z, wenn die Sichtverbindung über die Oberkante der Barriere verläuft (Feld `LineOfSightClear`)
- [x] Gl. 18 — Meteorologischer Korrekturfaktor K_met
- [x] Begrenzung D_z ≤ 20 dB (einfach) bzw. ≤ 25 dB (doppelt)
- [ ] Gl. 13 — Seitliche Beugung um vertikale Kanten (Abschnitt 7.4.3)
- [x] Geometrische Barriereerkennung (Strahl-Barriere-Verschneidung); die Beugungskanten werden nach der Gummibandmethode aus der oberen konvexen Hülle des Vertikalschnitts ausgewählt

### Reflexionen (Abschnitt 7.5)

- [ ] Gl. 19 — Frequenzbedingung für Reflexionsfläche
- [ ] Gl. 20 — Schallleistungspegel der Spiegelquelle
- [ ] Tabelle 4 — Reflexionskoeffizienten
- [ ] Spiegelquellenkonstruktion

### Meteorologische Korrektur C_met (Abschnitt 8)

- [x] Gl. 21 — C_met = 0 für d_p ≤ 10·(h_s + h_r)
- [x] Gl. 22 — C_met = C_0·[1 − 10·(h_s + h_r)/d_p]
- [x] C_0 als konfigurierbarer Parameter (Standard: 0 für reine Mitwindbeurteilung)
- [x] C_met wird je Quelle aus deren h_s und d_p gebildet und vor der Summation angewandt

### Sonstige Dämpfung A_misc (Anhang A, informativ)

- [ ] A.1 — Bewuchs (Tabelle A.1)
- [ ] A.2 — Industriegelände (Tabelle A.2)
- [ ] A.3 — Bebauung

## Bekannte Einschränkungen

- Luftabsorption wird beim Referenzluftdruck (101,325 kPa) berechnet; ein abweichender
  Standortluftdruck ist nicht parametrierbar. Die Lufttemperatur ist auf [−60, 60] °C begrenzt.
- Bodendämpfung nutzt einen einzigen globalen Bodenfaktor G für alle drei Regionen
- Barrierendämpfung wird nur für die Beugung über die Oberkante berechnet, im Vertikalschnitt
  zwischen Quelle und Immissionsort. Der Abstandsanteil a parallel zur Beugungskante (Bild 6) ist
  daher konstruktionsbedingt 0; er wäre nur auf dem seitlichen Beugungsweg von null verschieden,
  den dieses Modul nicht berechnet.
- Eine vorberechnete Beugungsgeometrie kann weiterhin übergeben werden und hat Vorrang vor der
  Szene; nur dann ist das Vorzeichen von z nach Abschnitt 7.4 über `LineOfSightClear` vom Aufrufer
  mitzuliefern, und geometrisch inkonsistente Eingaben werden zurückgewiesen. Eine automatisch
  hergeleitete Geometrie entsteht ausschließlich dort, wo eine Kante die Sichtverbindung
  tatsächlich unterbricht.
- Keine Reflexionsberechnung (Spiegelquellen)
- Keine Linien-/Flächenquellzerlegung
- Keine seitliche Beugung um vertikale Kanten
- Keine A_misc (Bewuchs, Industriegelände, Bebauung)

## Koeffizienten und Datenpakete

Tabelle 2 (Absorptionskoeffizienten) ist als Referenztabelle aus der öffentlich zugänglichen Norm eingebettet und dient ausschließlich als Prüforakel für die nach ISO 9613-1 berechneten Koeffizienten. Tabelle 3 (Bodendämpfung) wird durch die implementierten Funktionen a'(h)–d'(h) abgebildet. Keine weiteren normativen Daten werden im Repository gebündelt.

## Toleranzen

Gemäß Abschnitt 9, Tabelle 5 der Norm:

| Höhe h | Entfernung d < 100 m | Entfernung 100–1000 m |
| ------ | -------------------- | --------------------- |
| 0–5 m  | ±3 dB                | ±3 dB                 |
| 5–30 m | ±1 dB                | ±3 dB                 |

Diese Genauigkeitsangaben gelten für Breitbandrauschen unter Mitwind-/Inversionsbedingungen ohne Abschirmung oder Reflexion.

## Validierung

Golden-Test-Szenarien in `backend/internal/qa/acceptance/testdata/iso9613/`:

- `point_preview.scenario.json` — 2 Quellen, 4 Empfänger (Gitteranordnung)
- `point_contextual.scenario.json` — 3 Quellen, 6 Empfänger (abweichender Bodenfaktor, Temperatur,
  Luftfeuchte und Mindestabstand) mit einer Abschirmwand, die einen Teil der Ausbreitungswege
  kreuzt und die übrigen nicht: A_bar ist für einen Teil des Gitters von null verschieden und für
  den Rest null

Einheitentests in `backend/internal/standards/iso9613/`:

- Oktavband-Konstanten und A-Bewertung
- Atmosphärische Absorption gegen Tabelle-2-Referenzwerte
- Bodendämpfung für harten Boden (G=0) und porösen Boden (G=1)
- Anmerkung-1-Pfad: absoluter Pegel bei 500 Hz und Gleichwertigkeit mit einem äquivalenten 500-Hz-Spektrum (keine doppelte A-Bewertung)
- Bodendämpfung Mittelregion bei 63 Hz: A_m = −3q nach Tabelle 3
- Barrierenbeugung: Wegdifferenz mit Vorzeichen, C_3, K_met, D_z-Begrenzung, freie Sichtverbindung, Geometrievalidierung
- Meteorologische Korrektur: Nah-/Fernbereich, C_0=0-Sonderfall, pfadweise Anwendung bei mehreren Quellen
- Determinismus und Rückwärtskompatibilität

## Stand

Erstellt: 2026-03-28
Zuletzt geprüft: 2026-08-28
Modellversion: `iso9613-octaveband-v1`
Konformitätsgrenze: `iso9613-engineering-octaveband`
