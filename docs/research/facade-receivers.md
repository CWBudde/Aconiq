# Facade Immissionsorte: Placement Rules per Standard

Status date: 2026-10-21

This note is item A2 of `PLAN.md` Priority 18. It records where each standard Aconiq runs puts an
Immissionsort (receiver) at a building: the horizontal reference and offset, the height, and what
happens to the reflection at the receiver's own facade. It proposes a small set of
`placement_rule` identifiers for item A3, and lays out the open decision on what the storey-slab
height ("Höhe der Geschossdecke") is in metres when a model only knows `building:levels`.

**What may be quoted.** The rules below come from three kinds of source, and
`docs/policies/data-handling.md` treats them differently:

- **RLS-19** is FGSV-published and not clearly an amtliches Werk. Its content is **paraphrased in
  English and cited by section and page**. No sentence of it is reproduced here.
- **TA Lärm** is a Verwaltungsvorschrift (amtliches Werk). Short passages are quoted verbatim, with
  the number they carry.
- **16. BImSchV and its Anlage 2 (Schall 03)** are an ordinance (§ 5 (1) UrhG). Short passages are
  quoted verbatim, with the BGBl. page.

Secondary sources (published noise studies, a federal mapping method) are marked as such. They
document practice. They are not rules.

**Pilot privacy.** The pilot building is called "the pilot" in this note. Its street, house number
and coordinates are not written here. The pilot is a closed block-perimeter row house in
Hannover-Linden, with a street facade and a courtyard facade.

## 1. RLS-19 (road traffic)

**Source.** RLS-19, Ausgabe 2019 (FGSV 052), Section 1 "Begriffe, Definitionen, Abkürzungen und
Symbole", entry _Schallimmission, Immissionsort_, the Anmerkung to it, page 5. Reflections in
general: Section 3.6 "Berücksichtigung von Reflexionen", page 18.

**Legal weight.** This is not a recommendation that a project can depart from. § 3 of the
ordinance (16. BImSchV) refers to Section 3 of the RLS-19 together with Section 1. The Allgemeines
Rundschreiben Straßenbau Nr. 19/2020 (VkBl. 2020 S. 844), Part I, states that Sections 1 and 3
are therefore directly part of the ordinance. The definitions, including the Anmerkung on the
Immissionsort, are binding wherever the 16. BImSchV is.

**What the Anmerkung says** (paraphrase):

- **At a building**, the Immissionsort is at the height of the storey slab (_Geschossdecke_), and
  5 cm in front of the outer facade.
- **Own-facade exclusion.** Propagation paths whose **last** reflection lies on the facade that
  belongs to the Immissionsort are not counted.
- **Balconies and loggias.** The Immissionsort is at the outer facade, or at the parapet
  (_Brüstung_), at the slab height of the affected dwelling.
- **Outdoor living areas** (_Außenwohnbereiche_, for example terraces). The Immissionsort is
  2.00 m above the centre of the area defined as the outdoor living area.

**Horizontal.** Measured from the outer facade surface, along its outward normal, 0.05 m. The
window does not fix the position. It only decides **which facade and which storey** carries an
Immissionsort, and where along the facade it sits.

**Vertical.** The slab height of the storey. The RLS-19 gives no number for it, and no
relationship to the window (compare Schall 03 below, which gives one).

**Reflection.** Section 3.6 requires first- and second-order reflections, as mirror sources, at
facades and walls at least 1.0 m high. The Anmerkung then removes every path, of either order,
whose last mirror plane is the receiver's own facade. A second-order path that reflects first on
the own facade and then on the facade opposite still counts. A path whose second (last)
reflection is on the own facade does not. The result is the **incident level** at the facade:
it includes the street canyon, but not the doubling that the facade directly behind the receiver
would add.

**Balcony and outdoor area, reflection.** The Anmerkung states the exclusion for the Immissionsort
"at buildings". It does not say whether the parapet variant of a balcony has an "own facade" (the
parapet, or the wall behind the balcony). It also does not give the 5 cm offset again for that
case. For an outdoor living area there is no own facade, and every reflection counts. See
Open questions.

**Korrekturblatt and Rundschreiben.** The Korrekturblatt (Stand Februar 2020) corrects
equation (3) on page 12, equation (9) on page 16 and equation (10) on page 17. It changes nothing
in Section 1. The ARS 19/2020 says nothing about the Immissionsort beyond making Sections 1 and 3
binding. The Bundestag paper on the ordinance change (BT-Drs. 19/18471) describes the Immissionsort
only as the place the Beurteilungspegel is computed for.

## 2. TA Lärm (industrial and commercial noise)

**Source.** Sechste Allgemeine Verwaltungsvorschrift zum BImSchG (TA Lärm) of 26 August 1998
(GMBl. 1998 S. 503), as amended on 1 June 2017 (BAnz AT 08.06.2017 B5). Nr. 2.3 defines the
_maßgeblicher Immissionsort_ as the place "an dem eine Überschreitung der Immissionsrichtwerte am
ehesten zu erwarten ist", to be found according to Anhang A.1.3.

**Anhang A.1.3, verbatim:**

> Die maßgeblichen Immissionsorte nach Nummer 2.3 liegen
>
> a) bei bebauten Flächen 0,5 m außerhalb vor der Mitte des geöffneten Fensters des vom Geräusch
> am stärksten betroffenen schutzbedürftigen Raumes nach DIN 4109, Ausgabe November 1989;
>
> b) bei unbebauten Flächen oder bebauten Flächen, die keine Gebäude mit schutzbedürftigen Räumen
> enthalten, an dem am stärksten betroffenen Rand der Fläche, wo nach dem Bau- und Planungsrecht
> Gebäude mit schutzbedürftigen Räumen erstellt werden dürfen;
>
> c) bei mit der zu beurteilenden Anlage baulich verbundenen schutzbedürftigen Räumen, bei
> Körperschallübertragung sowie bei der Einwirkung tieffrequenter Geräusche in dem am stärksten
> betroffenen schutzbedürftigen Raum.

A.1.3 closes by referring to DIN 45645-1 (July 1996), Section 6.1, for substitute measurement
positions and microphone set-up. That governs measurements, not a calculation.

**a) Horizontal.** 0.5 m outward from the **window plane**, in front of the **centre of the
window**. The reference is the window, not the facade surface. On a facade with deep reveals, the
two planes differ.

**a) Vertical.** The window centre. Here the window **is** the position, in all three
coordinates.

**a) Which window.** The window of the _schutzbedürftiger Raum_ most affected by the noise, as
DIN 4109 (November 1989) defines that room. The LAI-Hinweise zur Auslegung der TA Lärm (Stand
24.02.2023, UMK-Umlaufbeschluss 13/2023) name DIN 4109-1:2018-01 as that standard's successor, in
their table "Bezüge zu technischen Normen und Regelwerken". In their section on Nr. 2.3 they add
that offices are schutzbedürftige Räume too. "Most affected" is a maximum over candidate windows.
That is an assessment step after the run, not a placement step: Aconiq places a point at every
candidate window, and the assessment picks the highest.

**b) Unbuilt or non-residential areas.** At the most affected edge of the area where planning
law allows buildings with schutzbedürftige Räume. A.1.3 gives no height for this case. The
LAI-Hinweise (on Nr. 2.3) confirm that, where a Bebauungsplan allows it, such points can lie on a
neighbouring plot that is not built yet. This rule needs a buildable-area polygon and a
permitted storey count as input, which no Aconiq import provides today.

**c) Inside the room.** Structure-borne sound, rooms structurally connected to the plant, and
low-frequency noise are assessed **inside** the room. This is not a point an outdoor propagation
model can compute. Aconiq places nothing for it.

**Propagation route, and ISO 9613-2.** The TA Lärm route does use ISO 9613-2. A.2.3.4 requires the
detailed prognosis per source and octave "entsprechend DIN ISO 9613-2, Entwurf Ausgabe September
1997, Abschnitt 6". Screening and reflections follow its Sections 7.4 and 7.5. A.2.3.2 lists "die
Lage und Höhe der maßgeblichen Immissionsorte" as an **input** to that calculation, so the
position comes from A.1.3 and not from ISO 9613-2. A text search of the 1996 and 2024 editions
held under `interoperability/ISO9613-2/` found no placement rule for a receiver at a facade, and
no own-facade exclusion. Reflections there are mirror sources at every qualifying surface.

**Reflection at the own facade.** The TA Lärm is silent. Taken literally, A.2.3.4 with
ISO 9613-2 Section 7.5 counts the reflection at the facade behind the receiver like any other.
Practice differs. _Secondary:_ a statement by Möhler + Partner Ingenieure AG to a municipality
(28 January 2021, on a Bebauungsplan in Saaldorf-Surheim) explains that a single-point calculation
0.5 m in front of the facade computes **no** back-reflection from the window, because the window
is assumed open. A grid point, by contrast, counts every reflecting surface, including the facade
it lies in front of. The statement also says the single-point result decides, and that a grid
used to check a facade must have the reflections at that facade switched off. So the rule in
practice is the same exclusion as RLS-19's, justified by the opened window rather than by the text.
Aconiq should make it explicit and recorded, not implicit (see Section 6).

## 3. Schall 03 / 16. BImSchV (rail)

**Source.** Anlage 2 zu § 4 der 16. BImSchV (Schall 03), as inserted by the Verordnung zur
Änderung der 16. BImSchV of 18 December 2014 (BGBl. 2014 I S. 2269). Nr. 2.2.10, BGBl. 2014 I
S. 2274, verbatim:

> 2.2.10 Immissionsort
>
> IO
>
> Maßgeblicher Ort für die Ermittlung eines Beurteilungspegels, nach dieser Anlage
>
> – bei Gebäuden in Höhe der Geschossdecke (0,2 m über der Fensteroberkante) auf der Fassade der
> zu schützenden Räume und
>
> – bei Außenwohnbereichen 2 m über der Mitte der als Außenwohnbereich genutzten Fläche
>
> Anmerkung: Für Immissionsorte an Gebäuden werden Reflexionen an der zugehörigen Fassade nicht
> berücksichtigt.

(An earlier grep for "Geschossdecke" missed this passage. The BGBl. PDF is letter-spaced, and
`pdftotext` renders the word as "Ges chos s decke". Searching with whitespace removed finds it.)

**Horizontal.** **On** the facade of the rooms to be protected, with an offset of 0 m. There is no
5 cm and no 0.5 m. The reference is the facade surface.

**Vertical.** Slab height, which the ordinance itself makes concrete: **0.2 m above the top edge
of the window** (_Fensteroberkante_). Schall 03 is the only one of the three texts that ties the
slab height to a window measure. When the window head is known, the height follows exactly.

**Reflection.** Reflections at the associated facade (_zugehörige Fassade_) are not counted. The
wording is broader than RLS-19's: it excludes **all** reflections at that facade, not only the
paths whose last reflection lies there. With first-order image sources the two readings agree.
With higher orders they do not. Nr. 6.6 (BGBl. 2014 I S. 2297 f.) treats reflections as
incoherent mirror sources, and Nr. 6.1 (S. 2293) leaves out only reflections above the third
order. The schall03 module enumerates up to that order (`MaxReflectionOrder = 3`,
`internal/standards/schall03/reflection.go`). So for Schall 03, a second- or third-order path
that touches the own facade **anywhere** is excluded, not only one that ends there.

**Outdoor living areas.** 2 m above the centre of the area used as an outdoor living area. This
is the RLS-19 rule in the ordinance's own words ("genutzten" against the RLS-19's "defined"
area).

**16. BImSchV main text.** §§ 1–5 contain no placement rule. § 2 sets the limits, § 3 refers to
the RLS-19 for roads, and § 4 (1) says: "Der Beurteilungspegel für Schienenwege ist nach Anlage 2
zu berechnen." Anlage 1 is repealed. The placement rules for both modes therefore live in the
calculation methods: the RLS-19 Section 1 for roads, and Anlage 2 Nr. 2.2.10 for rail.

**24. BImSchV** (passive noise protection, for completeness). § 2 (2) defines the schutzbedürftige
Räume by reference to Table 1 of its Anlage. § 2 (3) names "insbesondere Fenster, Türen,
Rolladenkästen, Wände, Dächer" as the Umfassungsbauteile. The Anlage takes the Beurteilungspegel
from the 16. BImSchV calculation and gives no position of its own. It tells Aconiq which rooms
count, not where the point goes.

## 4. Proposed placement rules

These are the `placement_rule` values for A3. Each one names one standard's rule, so a point's
rule identifies the passage it was placed under. The list is deliberately short. A rule is added
when a standard Aconiq runs needs it, not when a geometry would allow it.

| `placement_rule`         | Standard and citation                   | Horizontal reference and offset                                     | Height                                                                | Own-facade reflection                                                         |
| ------------------------ | --------------------------------------- | ------------------------------------------------------------------- | --------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| `rls19-facade`           | RLS-19 Sec. 1, Immissionsort, Anm. p. 5 | Outer facade surface, 0.05 m out along the outward normal           | Slab height of the storey (Section 5)                                 | Excluded: paths whose **last** reflection is on the own facade                |
| `rls19-balcony`          | RLS-19 Sec. 1, Immissionsort, Anm. p. 5 | Outer facade **or** parapet line of the balcony/loggia; offset open | Slab height of the affected dwelling's storey                         | Open (Section 7)                                                              |
| `rls19-outdoor-area`     | RLS-19 Sec. 1, Immissionsort, Anm. p. 5 | Centre of the outdoor-living-area polygon                           | 2.00 m above ground at that centre                                    | Not applicable: no own facade, all reflections count                          |
| `schall03-facade`        | 16. BImSchV Anl. 2 Nr. 2.2.10 (S. 2274) | On the facade surface, 0 m (see Section 6 on the geometry)          | 0.2 m above window head when known, otherwise slab height (Section 5) | Excluded: **all** reflections at the associated facade                        |
| `schall03-outdoor-area`  | 16. BImSchV Anl. 2 Nr. 2.2.10 (S. 2274) | Centre of the outdoor-living-area polygon                           | 2 m above ground at that centre                                       | Not applicable                                                                |
| `talaerm-window`         | TA Lärm Anhang A.1.3 a)                 | Window plane, 0.5 m out, in front of the window centre              | Window centre                                                         | Text silent. Practice excludes it (open window). Must be explicit (Section 6) |
| `talaerm-buildable-edge` | TA Lärm Anhang A.1.3 b)                 | Most affected edge of the buildable area                            | Not given. Practice: window-centre height of each permitted storey    | Not applicable: no building yet                                               |

Not given an identifier:

- **TA Lärm A.1.3 c)** is an indoor position. No outdoor model computes it.
- **The auto-grid and hand-placed `custom` receivers** keep no `placement_rule`. They are not
  Immissionsorte in any standard's sense, and the absence of a rule says so.
- **SoundPLAN-imported receivers** carry SoundPLAN's own geometry: its floor heights, and the
  receiver-to-facade distance the project file stores (`RECFACDIST`,
  `internal/io/soundplanimport`). Giving them an Aconiq rule would claim a placement Aconiq did
  not perform. See Open questions.

The geometry of a point is a pure function of (rule, building, facade edge, station or window,
storey) and the slab-height inputs of Section 5. Two runs with the same inputs place the same
points. That is the determinism B2 needs.

## 5. "Höhe der Geschossdecke" in metres

Neither the RLS-19 nor Schall 03 gives a storey height. Schall 03 gives a window relation instead
(0.2 m above the window head). An OSM model carries `building:levels` and at best a `height`. This
section sets out the options and the decision taken in review (2026-10-22): **option C**, see
[Decision](#decision).

### Which slab

A storey has two slabs: the one it stands on, and the one above it (its ceiling). The texts mean
the **ceiling slab**, the top of the storey's rooms:

- Schall 03 equates the slab height with "0,2 m über der Fensteroberkante". A window head lies near
  the top of a room, so 0.2 m above it is the underside of the slab above, not the floor.
- The floor-slab reading puts the ground floor's Immissionsort at the ground-floor level, which is
  ground level or close to it. Nothing in either text suggests that.
- _Secondary_, practice: a 2019 study (Frank & Apfel GbR, Eisenach, Schallimmissionsprognose
  LG 63/2019-A, updated 20 March 2021, for a Bebauungsplan in Apolda) puts its traffic receivers
  "unter jeweiliger Geschossdecke": ground floor 2.8 m, first upper floor 5.6 m, second 8.4 m.
  Its TA Lärm receivers in the same study sit at "Mitte Fenster": 2.0, 4.8 and 7.6 m. That is a
  2.8 m storey height in both cases, with the window centre 0.8 m below the slab. (Its traffic
  method was still the RLS-90, so this documents the slab convention in practice, not an RLS-19
  reading.)

With storey index `n` (0 = ground floor, _Erdgeschoss_), storey height `s`, and ground-floor level
above terrain `h_EG` (the _Erdgeschossfußbodenhöhe_, 0 when unknown):

```
ceiling-slab reading:  z(n) = h_EG + (n + 1) · s
floor-slab reading:    z(n) = h_EG + n · s          (rejected: z(0) = h_EG, at the ground)
Schall 03 with window: z    = window_head + 0.2 m   (window_head above terrain)
TA Lärm with window:   z    = window_centre
```

### Options for `s` when only `building:levels` is known

| Option                         | `s`                                                                                                                       | Ground floor, 1st, 2nd, 4th upper floor (`h_EG` = 0) | For                                                                                                                                                                          | Against                                                                                                                                                  |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| A. Fixed 3.0 m                 | 3.0 m, the OSM importer's `metersPerLevel` (`internal/io/osmimport/osmimport.go`, `buildingHeight`)                       | 3.0, 6.0, 9.0, 15.0 m                                | Agrees with the model's own building heights: the top storey's slab is exactly the derived roof height. Same 3 m as the federal BEB method uses for unknown heights (below). | One number for every building. Underestimates tall-storeyed old buildings.                                                                               |
| B. Fixed 2.8 m                 | 2.8 m, the value common in practice                                                                                       | 2.8, 5.6, 8.4, 14.0 m                                | Matches published practice.                                                                                                                                                  | A second storey constant next to the importer's 3.0 m. In a 5-level OSM building of 15.0 m, the top slab sits 1.0 m under the roof the same model draws. |
| C. Per building, fallback to A | `H / levels` when a height `H` that was **not** derived from levels exists (OSM `height`, LoD2, CityGML); otherwise 3.0 m | Depends on the building                              | Uses the evidence a model has. Equals A wherever A is all there is. Matches how SoundPLAN parameterises it (below).                                                          | A roof height is not an eave height: a pitched or mansard roof inflates `H`. `h_EG` is still unknown.                                                    |
| D. Required input              | None. The rule refuses to place without `s` and `h_EG`                                                                    | n/a                                                  | Nothing is assumed.                                                                                                                                                          | B2 cannot run on an OSM-only model at all, which is the common case.                                                                                     |

Supporting evidence:

- **The federal mapping method uses 3 m.** _Secondary transcription:_ the BEB (Berechnungsmethode
  zur Ermittlung der Belastetenzahlen, BAnz AT 05.10.2021 B4) estimates an unknown building height
  from the storey count "unter Annahme einer durchschnittlichen Geschosshöhe von 3 m"
  (umwelt-online.de, Section 3 of the BEB). That is a height estimate for exposure counting, not
  an Immissionsort rule, but it is the one official number for a storey.
- **SoundPLAN treats it as per-project and per-point input.** Its `Project.sp` carries a receiver
  height above the ground-floor level (`HABOVEEFH`) and a floor height (`FLOORHEIGHT`), and each
  immission point carries a first-floor offset and a floor spacing. Aconiq stacks them as
  `FloorRefZ + FirstFloorOffsetM + (floor − 1) · FloorSpacingM` (`ImmissionPoint.FloorZ` in
  `internal/io/soundplanimport/geoobjs.go`). The value of the reference project is not repeated
  here (customer data).
- **The pilot class is the hard case.** A Gründerzeit block-perimeter row house, which is what
  Linden-Mitte is built of, typically has storeys well above 3 m and a raised ground floor
  (_Hochparterre_). This is an engineering expectation, not a sourced figure. If it holds, A and B
  both place every upper-floor point too low, and the error grows with `n`. F1 should measure it
  from the site photos (D1 gives storey lines in the facade frame).

### Decision

**C, with the inputs recorded** (decided in review, 2026-10-22).

- `z(n) = h_EG + (n + 1) · s`.
- `s` is taken per building from a height the model did not derive from `building:levels`, and
  falls back to the importer's 3.0 m. Using the same constant as the importer keeps one storey
  height in the codebase.
- `h_EG` defaults to 0 and can be set per building.
- A window, once known (C1), overrides the storey arithmetic where its standard allows it:
  Schall 03 takes `window_head + 0.2 m` directly. For RLS-19, the same relation is the most
  defensible slab estimate, but it is borrowed from Schall 03 and must be recorded as borrowed.
- Every placed point records where `s` came from in `slab_height_source`: `measured` (a height
  not derived from levels, or a window), `levels` (the 3.0 m fallback on a known storey
  count) or `assumed` (no storey count either). A point whose `slab_height_source` is `assumed`
  counts as unconfirmed under the rule in
  [what the run does with `proposed`](#decision-what-the-run-does-with-proposed).

**The importer does not yet say which case it is in.** `osmimport` writes `height_source` only
for the fallback (`internal/io/osmimport/osmimport.go:31-37`, value `assumed`); a height read from
the `height` tag and one derived as `building:levels × 3.0` (`buildingHeight`, `osmimport.go:288`)
both leave it absent, and `building:levels` is kept as a property in both cases
(`osmimport.go:246`). So `H / levels` cannot tell a tagged 15 m from a derived 15 m, and in the
derived case it returns exactly 3.0 m, the fallback, only by coincidence. B2 therefore needs
`osmimport` to record the derived case as its own `height_source` value (`levels`), extending the
schema's [Height provenance](../geojson-schema-v1.md#height-provenance) section. Until then a
building with `building:levels` and no `height_source` must be read as `levels`, never as
`measured`. LoD2 and CityGML heights are `measured`.

## 6. Consequences for Aconiq

**One `offset_m` on `FacadeReceiverSet` is the wrong shape.** `geo.FacadeReceiverSet`
(`internal/geo/receivers.go`) has one `OffsetM` and one `VerticalStepM` for the whole set. The
three standards differ in all three coordinates:

- the offset: 0.05 m, 0.5 m and 0 m;
- what the offset is measured from: the facade surface, the window plane, or the parapet;
- the height: the ceiling slab, the window centre, or 2 m above an area.

`VerticalStepM` fails on its own terms. A step from the ground puts the first point at the step
height, which is the ceiling-slab reading for the ground floor only by coincidence (when
`h_EG` = 0 and step = `s`), and it can never express a window centre. The set should name a
`placement_rule` and let the rule derive offset, reference and height. The stub carries no
behaviour (`IsImplemented()` is false), so replacing the fields costs nothing.

**One window yields up to three Immissionsorte.** At a building exposed to a road, a railway and
a plant, the same window of the same bedroom gives a point at slab height 5 cm out (RLS-19), one
at the window head plus 0.2 m on the facade (Schall 03), and one at the window centre 0.5 m out
(TA Lärm). The receivers a run uses therefore depend on the standard. A run should take the points
whose `placement_rule` belongs to its standard, and refuse or warn on the others. Running RLS-19
over `talaerm-window` points would compute a level at a place the RLS-19 does not assess.

**The own-facade exclusion needs the receiver→facade link (B1).** RLS-19 and Schall 03 both
exclude reflections at the receiver's own facade. TA Lärm practice does the same. Nothing in the
tree links a receiver to a facade, so today every hand-placed or SoundPLAN-imported facade point
5 cm out probably gets its own facade's image source counted. With the RLS-19's 0.5 dB loss for a
facade, that image is almost as strong as the direct sound. In the limit the receiver gets close
to +3 dB, which is the doubling the rule exists to remove. B1 measures it before B2 builds on
it. The link (`building_ref`, `facade_edge`) is the input the fix needs.

**"The facade" is a plane, not an edge.** The surprise for the pilot. In a closed block perimeter,
the neighbours' street facades are coplanar with the pilot's, and continuous with it. A receiver
5 cm in front of that plane sees an almost identical mirror image whether the reflection point
lies on its own house's part of the wall or on the neighbour's. If the exclusion only covers the
receiver's own footprint edge, reflections a metre past the party wall still count, and the level
is overestimated. The rule should exclude by plane: every facade segment coplanar with the own
facade, within a tolerance, across building boundaries. It should not exclude by edge id. Whether
the texts' "facade belonging to the Immissionsort" stretches that far is an interpretation, and
belongs in the conformance declaration once chosen.

**Schall 03's 0 m offset is a geometric edge case.** A point exactly on the facade lies on the
footprint boundary. The strict-interior mask (`geo.MaskPointsInFootprints`) keeps it, and the
point-in-polygon tests disagree there (PLAN B2). Propagation must not treat the own wall as an
obstacle for a point on it. There are two ways: use the facade link to exempt the own facade from
screening as well as from reflection, or place the point a recorded numerical distance outward.
Whichever is chosen must be the same on both targets.

**TA Lärm's exclusion must be explicit.** The TA Lärm text does not exclude the own facade, and
ISO 9613-2 counts every reflector. If Aconiq follows practice, the exclusion is a parameter of
the `talaerm-window` rule, recorded in provenance. It is not a property the ISO 9613-2 module
happens to have. A reviewer must be able to see from the run which choice was made.

**The default receiver height is not a facade rule.** `receiver_height_m` defaults to 4 m in
`rls19-road` (`internal/standards/rls19/road/model.go`) and in `schall03`
(`internal/standards/schall03/params.go`). 4 m is the strategic-mapping convention, not either
standard's Immissionsort. It is harmless for grids. Facade points must not inherit it.

**Top-storey points sit at roof height.** Under option A, or C on a levels-derived height, the top
storey's slab is exactly the building height the importer derived. A courtyard-facade point on the
top floor then sees street sources over its own roof edge at zero height difference, a
grazing-diffraction case. That is numerically sensitive. The F2 golden fixture should include it.

**The RLS-19 rule binds through the ordinance.** The placement rule sits in Section 1 of the
RLS-19, and the ARS 19/2020 makes that section part of the 16. BImSchV. A deviation is therefore
a conformance defect in a normative module, not a modelling preference.

## 7. Open questions

1. **Raised ground floor** (Section 5 decided `s`, not `h_EG`): whether `h_EG` gets a default
   other than 0 for buildings known to have a _Hochparterre_, and which evidence sets it.
2. **RLS-19 balcony, reference line:** the outer facade or the parapet, and who decides per
   balcony. Does the 5 cm offset apply, and from which surface?
3. **RLS-19 balcony, own facade:** is it the wall behind the balcony, the parapet, or both? A
   loggia is recessed, so its back wall and the outer facade line are different planes.
4. **Extent of the own facade:** the receiver's own edge, the building's coplanar edges, or every
   coplanar segment across party walls (Section 6). The plane tolerance (angle and offset).
5. **Schall 03, all reflections against last reflection:** the module goes to third order, so
   Nr. 2.2.10's broader wording needs its own filter (drop any path touching the own facade),
   distinct from RLS-19's last-reflection filter. Confirm that reading before B1 fixes both
   with one function.
6. **TA Lärm own facade:** adopt the practice exclusion as the default of `talaerm-window`, or
   compute both and report both? Whether the LAI or a Land guidance document states it anywhere
   (not found in the LAI-Hinweise 2023).
7. **TA Lärm b):** which input carries the buildable area and the permitted storeys (a
   Bebauungsplan import does not exist), and what height practice uses there. Until then,
   `talaerm-buildable-edge` is reserved, not implemented.
8. **SoundPLAN-imported receivers:** keep them rule-less with their SoundPLAN provenance, or map
   them to a rule when their facade distance and floor heights match one. A mapping would let them
   take part in the own-facade exclusion. Without it, B1's fix does not reach them unless they
   also get a facade link.

Decided in review (2026-10-22) and no longer open: the storey height `s` (Section 5) and whether
unconfirmed points may enter a normative run
([Data model](#decision-what-the-run-does-with-proposed)). One rule governs both unconfirmed
windows and assumed slab heights.

## Sources

Primary, read locally from `interoperability/` (untracked) or fetched:

- RLS-19, Ausgabe 2019 (FGSV 052): Section 1, entry _Schallimmission, Immissionsort_, Anmerkung,
  p. 5; Section 3.6, p. 18. Korrekturblatt, Stand Februar 2020. Paraphrased only.
- Allgemeines Rundschreiben Straßenbau Nr. 19/2020 of 24 November 2020 (VkBl. 2020 S. 844), Part I.
- BT-Drucksache 19/18471 (draft second ordinance amending the 16. BImSchV), Section 1.1.
- TA Lärm of 26 August 1998 (GMBl. 1998 S. 503), consolidated as of 1 June 2017 (BAnz AT
  08.06.2017 B5): Nr. 2.3; Anhang A.1.3 a)–c); A.2.3.2; A.2.3.4.
- LAI-Hinweise zur Auslegung der TA Lärm, Stand 24.02.2023: section on Nr. 2.3; "Bezüge zu
  technischen Normen und Regelwerken".
- Anlage 2 (Schall 03) of the 16. BImSchV, in the Verordnung of 18 December 2014 (BGBl. 2014 I
  S. 2269): Nr. 2.2.10 (S. 2274); Nr. 6.1 (S. 2293); Nr. 6.6 (S. 2297 f.).
- The 16. BImSchV, §§ 1–5:
  <https://www.gesetze-im-internet.de/bimschv_16/BJNR010360990.html>
- The 24. BImSchV, § 2 and Anlage:
  <https://www.gesetze-im-internet.de/bimschv_24/BJNR017200997.html>
- ISO 9613-2:1996 and ISO 9613-2:2024, local copies: searched for a facade-receiver rule; none
  found. Not quoted.

Secondary (practice; not rules):

- Möhler + Partner Ingenieure AG, statement on a Bebauungsplan to the Gemeinde Saaldorf-Surheim,
  28 January 2021. Own-facade reflection at TA Lärm single points against grid points:
  <https://www.saaldorf-surheim.de/download/131/laufener-strasse-ost/5094/bebauungsplan-laufener-strasse-ost-schalltechnische-untersuchgung.pdf>
- Frank & Apfel GbR, Schallimmissionsprognose LG 63/2019-A (31 December 2019, updated 20 March
  2021). Receiver heights under the slab against window centre:
  <https://www.apolda.de/fileadmin/Dateien/Cross7/Startseite/Stadt%20Apolda/Aktuelles/Stadtplanung/03_B-Plan_Revitalisierung_RST-Gelaende_Anl._6.1_Schallimmissionsprognose.pdf>
- AMT Ingenieurgesellschaft mbH, Schalltechnisches Gutachten 169366-A (16 April 2025), for a
  Hannover Bebauungsplan. TA Lärm points 0.5 m in front of the opened window, with storeys taken
  from a site visit:
  <https://www.hannover.de/content/download/1067349/file/973,%201.%20%C3%84nd.%20-%20Schalltechnische%20Untersuchung.pdf>
- BEB 2021, storey height of 3 m for estimating an unknown building height, as transcribed by
  umwelt-online: <https://umwelt-online.de/recht/laerm/beb_21.htm>

## Data model (A3)

This section is item A3. It specifies the facade frame and the receiver properties. The two
decisions it needed, what keys a building and what a run does with unconfirmed points, were taken
in review (2026-10-22). Nothing here is implemented.

### Schema v1 takes it additively

No migration is needed for the model. `modelgeojson` keeps every property it does not know:
`normalizeProperties` copies all non-empty keys (`internal/geo/modelgeojson/normalize.go:353`),
and `Validate` checks only `kind`, `height_m`, the geometry and the properties it owns
(`validateFeature`, `internal/geo/modelgeojson/validate.go:101`). The SoundPLAN receivers are the
precedent: `soundplan_obj_id` and `soundplan_floor` ride on ordinary `receiver` features and are
read back by `soundPlanReceiverKeysFromModel` (`internal/app/cli/compare_match.go:384`).
Migrations (`docs/project-migrations.md`) version the project manifest, not model properties.

Three things are not free, and each belongs to the item that first reads the property:

- **Validation.** Once a run reads `placement_rule` or `facade`, a malformed value must be a
  validation error with its own code, not a silently ignored key. B2 and B3 add those codes.
- **Coordinates in properties must be reprojected, and the registry cannot hold this shape
  yet.** The facade key below stores the edge's endpoints. `propertyGeometries`
  (`internal/geo/modelgeojson/reproject.go:92`) lists the properties whose coordinates move with
  the model, and its comment warns that a property missing from it ends up "in the wrong
  hemisphere". Adding `facade` to that list is not enough: every entry names a container that
  must be an **array of objects** (`reproject.go:83-84`), and `reprojectProperties` skips any
  container that is not a `[]any` without a word (`reproject.go:109-112`). `facade` is a single
  object holding two `[x, y]` members. The change that first writes it therefore extends
  `propertyGeometry` with a singleton-object form (or a `[x, y]`-member form inside it), adds the
  entry, and tests that a reprojected model moves `from` / `to` with the point. A silent skip
  would leave the check endpoints in the old CRS and turn every re-import into a
  `receiver.facade.moved` finding.
- **The receiver table has no per-row attributes.** `results.ReceiverRecord`
  (`internal/results/receiver_table.go:11`) holds id, x, y, `height_m` and the values. Naming a
  row's building, facade and storey (B3) adds optional fields to result container v1
  (`docs/result-containers-v1.md`). That is additive for JSON; for CSV it appends columns, and B3
  has to check the readers.

### The facade frame

A facade is one edge of one ring of one building footprint. Its frame is:

- `u`: metres along the edge from its start vertex `a` to its end vertex `b`, `0 ≤ u ≤ |b − a|`;
- `z`: metres above the ground at the facade;
- the outward normal `n̂`, pointing away from the building's interior.

A point in the frame maps to the world as

```
p(u, d) = a + u · t̂ + d · n̂        t̂ = (b − a) / |b − a|
z       = z
```

where `d` is the placement rule's offset (0.05 m, 0 m, or 0.5 m from the window plane). This is
one function in `internal/geo`, and the WASM kernel exposes it for C2. The frontend must not
reimplement it, for the reason it must not reimplement `transform`.

**The outward normal comes from orientation, never from a point-in-polygon test.** Each ring is
normalised before it is indexed: the exterior counter-clockwise, holes clockwise. `geo` already
has the signed shoelace area (`signedRingArea`, `internal/geo/geometry.go:469`). After that
normalisation the outward normal of every edge, exterior or courtyard, is the right-hand normal
`n̂ = (t̂_y, −t̂_x)`. A test against the footprint would sit exactly where the strict and the
inclusive point-in-polygon variants disagree (PLAN B2).

A **window** is a rectangle in that frame: `u` of its left edge, `z` of its sill, `width`, and
`height`. Its centre (`talaerm-window`) and its head (`schall03-facade`) follow from it.

### The facade key

`facade` is an object, not an index, because a building can be a `MultiPolygon` and can have
courtyard holes (the CityGML import keeps them, `internal/io/citygmlimport/citygmlimport.go:252`).
Synthetic values:

```json
"facade": {
  "building_ref": "osm-way-123456",
  "part": 0,
  "ring": 0,
  "edge": 3,
  "from": [500010.0, 5800020.0],
  "to": [500018.5, 5800022.5]
}
```

- `part`: the polygon within a `MultiPolygon` (0 for a `Polygon`), in canonical order.
- `ring`: 0 for the exterior, 1… for courtyard holes, in canonical order.
- `edge`: the index within the canonical ring.

**Canonicalisation** makes all three indices independent of how the source wrote the geometry.
GeoJSON fixes neither the order of a `MultiPolygon`'s parts nor the order of a polygon's holes,
so canonicalising the vertices alone would not survive a re-import. In order:

1. Each ring: orientation normalised as above, then rotated to start at its lexicographically
   smallest vertex (x, then y).
2. The holes of each part: sorted by their canonical vertex sequence, compared lexicographically
   pair by pair (x, then y). The exterior stays ring 0.
3. The parts: sorted the same way by their canonical exterior ring.

Valid parts do not overlap and valid holes do not overlap, so two rings compare equal only if
they are the same ring, and the order is total. The comparison is on the stored `float64`
values, with no tolerance: a tolerance would make the order depend on which neighbour is within
it, and the `from` / `to` check below exists to catch a geometry that moved.

- `from` / `to`: the edge's endpoints in the project CRS, as a check, not as the key. A re-import
  whose canonical edge no longer has these endpoints (within a tolerance) is a validation
  finding, not a silent re-attachment.

A balcony or loggia (`rls19-balcony`) needs a reference line that is not a footprint edge. That
is open in Section 7 (questions 2 and 3), and the key may grow a `line` variant for it.

**Receiver IDs carry the whole key.** A generated point's ID is formed from (`building_ref`,
`part`, `ring`, `edge`, station, `storey`), in that order. Dropping `part` or `ring` would give a
courtyard edge 0 and an exterior edge 0 at the same station and storey the same ID.

### Receiver properties

| Property             | Type             | Meaning                                                                                                   |
| -------------------- | ---------------- | --------------------------------------------------------------------------------------------------------- |
| `placement_rule`     | string           | One of Section 4's identifiers. Absent on grid, `custom` and SoundPLAN receivers.                         |
| `facade`             | object           | The facade key above. Absent for the outdoor-area rules.                                                  |
| `storey`             | integer          | 0 = ground floor (_Erdgeschoss_), as in Section 5.                                                        |
| `station_u_m`        | number           | `u` of the point on the facade. For a window-derived point, the `u` of the window's centre.               |
| `window`             | object           | `{ "u_m", "sill_z_m", "width_m", "height_m" }` in the facade frame. Absent for B2 geometry-only points.   |
| `slab_height_source` | string           | `measured`, `levels` or `assumed` (Section 5, Decision). Absent when a window fixes the height.           |
| `evidence`           | array of objects | Where the placement came from. Empty for a pure B2 point. Fields below.                                   |
| `review_status`      | string           | `proposed`, `confirmed` or `rejected`. B2 points start `proposed`. A `rejected` point never enters a run. |

`height_m` stays what it is for every receiver: the height above ground the run uses. The
properties above say **why** it has that value; they do not replace it. A run never recomputes a
receiver's position from them. It reads `Feature.Coordinates` and `height_m` as today, so a model
that carries them computes exactly what it would without them, apart from the own-facade
exclusion that reads `facade` (B1).

Each `evidence` entry carries the fields `docs/research/facade-evidence-sources.md` derives per
source kind: `kind` (`orthophoto`, `oblique`, `street-level`, `site-photo` or `plan`; there is no
`google` kind), `source_id`, `content_sha256`, `captured_at`, `retrieved_at`, `licence` (an SPDX id,
or `own` / `client-engagement`), `terms_url`, `attribution`, and for D1 the `transform`: the four
image-to-facade point pairs. A site photo's `source_id` is an opaque reference into the
engagement's store, never a path that names the client.

### Decision: what keys a building

`building_ref` has to survive a re-import. The candidates:

1. **The model feature `id`.** OSM buildings are `osm-way-<id>`
   (`internal/io/osmimport/osmimport.go:168`), CityGML and LGLN buildings carry their `gml:id`
   (`internal/io/citygmlimport/citygmlimport.go:259`). This is stable as long as the source is.
   It breaks in two known ways. An LGLN import replaces the OSM buildings in its box
   (`docs/geojson-schema-v1.md`, Import provenance), so every `osm-way-*` reference there
   dangles. And a CityGML building without a `gml:id` gets `citygml-building-%03d`, an index into
   the file (`citygmlimport.go:246`), which is not stable at all.
2. **An Aconiq-own id stamped at import.** Stable across Aconiq's own edits, but a fresh import
   of the same source mints new ids, so it moves the problem rather than solving it.
3. **A geometric match.** Re-attach by footprint overlap and the `from` / `to` endpoints. This
   survives a change of source, but it is a heuristic, and a heuristic must not decide where an
   Immissionsort is without a person seeing it.

**Decided (review, 2026-10-22): 1, with 3 as an explicit, reviewed step.** Key by the feature
`id`. A receiver
whose `building_ref` does not resolve, or whose `from` / `to` no longer match, is a validation
finding (`receiver.facade.unresolved`, `receiver.facade.moved`) that blocks a normative run. An
explicit re-attach action (CLI or UI) proposes matches by geometry and sets the matched points to
`review_status: proposed` until a person confirms them. A building that only has the
`citygml-building-%03d` fallback id cannot carry facade receivers until it has a stable id.

### Decision: what the run does with `proposed`

**Decided (review, 2026-10-22): refused unless opted in.** One rule governs every unconfirmed
input to a facade Immissionsort:

- a point with `review_status: proposed`, and
- a point with `slab_height_source: assumed`.

A normative run over a receiver set that contains either is refused, the way a scaffold standard
is refused without `--experimental`. An explicit opt-in flag lets it run. The run then stamps the
number of proposed and of assumed-height points into `provenance.json` and `run-summary.json`,
and the report states them, so the result carries its own caveat to a reader who never sees the
model. A `rejected` point never enters a run, with or without the flag. A non-normative standard
runs over proposed points without the flag. The flag is `--allow-unconfirmed` on `aconiq run`
and `allow_unconfirmed` on the API and MCP (P18 B3).
