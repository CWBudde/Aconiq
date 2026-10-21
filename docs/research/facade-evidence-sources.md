# Facade Evidence Sources

Status date: 2026-10-21

## Purpose

This note is item A1 of `PLAN.md` Priority 18 ("Facade Immissionsorte from site and image
evidence"). For each imagery or document source an engineer might use to place an Immissionsort at
a facade, or to check a source against the site, it records what the source shows, at what
resolution, whether it covers the Linden-Mitte pilot area, under which licence, with which
attribution, and whether features derived from it may be stored in a project and in this
repository.

It is a clarification note, not a legal opinion. Where a question needs a lawyer or a written answer
from a rights holder, it is listed under [Open questions](#open-questions) rather than answered.

Two constraints from Priority 18 frame every row:

- **Imagery is evidence frozen into the model, never a compute input.** What a source contributes
  is a receiver, window or source feature with provenance. Nothing in `engine/` or a standards
  module reads an image, so a licence question here is a question about the _model_, the
  _evidence record_ and what is exported, not about the run.
- **The pilot fixture is synthetic** (F2). Nothing derived from real imagery of the pilot area
  enters the tree, whatever its licence allows. The "in the repository" column below therefore
  answers two different questions: what the licence would permit, and what the project allows.

## How to read this note

- **Verified** means the statement was read on 2026-10-21 from a page, metadata record or API
  response published by the rights holder or operator itself. The URL is given in
  [Sources fetched](#sources-fetched).
- **Secondary** means a community wiki, forum or encyclopaedia reported it. Secondary claims are
  kept where they explain a conflict, never as the basis of a row.
- **Not verified** means the claim could not be checked on that date, and says why.
- The pilot is referred to only as "the Linden-Mitte pilot area" or by the bounding box
  `52.3616,9.7049,52.3757,9.7233` (`LINDEN_BBOX` in `scripts/repro-linden.sh`, lat/lon order).
  Coverage below is coverage of that box, not of a particular building. No street, house number or
  building coordinate belongs in this file.

## Summary matrix

"Project" means a project folder outside the tree (`.noise/`), held under the engagement. "Repo"
means tracked source, including `testdata/`.

| Source                                          | Shows                                                         | Resolution                                                         | Coverage of the pilot area                                                                                       | Licence                                                                                                                                | Required attribution                                                                                                    | Derived features: project                                  | Derived features: repo                                                                                              |
| ----------------------------------------------- | ------------------------------------------------------------- | ------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| LGLN DOP20 / DOP10 (orthophoto)                 | Roofs, courtyards, roads, parking, rooftop plant. No facades. | 20 cm; 10 cm for the 2025 flight here                              | Full. Five DOP20 vintages (2013, 2016, 2019, 2022, 2025) and DOP10 of 2025-03-06 (verified, STAC)                | **CC BY 4.0** (verified, metadata record and STAC). AGNB text not retrievable on that date.                                            | `LGLN (<year of retrieval>) Creative Commons Namensnennung – 4.0 International (CC BY 4.0)` (verified, metadata record) | Yes, with attribution in the evidence record               | Licence would permit it with attribution; project policy: no (synthetic fixture, and no `data-handling.md` row yet) |
| Stadt Hannover DOP20 (city flight)              | As above                                                      | Not stated for current vintages                                    | Whole city area, flown every second spring; 2025 latest listed (verified)                                        | **CC BY 4.0** "soweit nicht anders gekennzeichnet" (verified, Open GeoData terms)                                                      | `Datenquelle: CC-BY-4.0 – Bereich Geoinformation – LH Hannover` (verified)                                              | Yes, with attribution                                      | As LGLN DOP                                                                                                         |
| Oblique aerial imagery (Hannover 3D)            | **Facades and roofs**, four view directions                   | 20 cm (2017, 2020, 2024) in the public viewer                      | The 2024 collection's declared extent contains the box (verified, viewer config). Per-tile coverage not checked. | Viewer config attributes `LH Hannover - Geoinformation, CC-BY-4.0`. No licence document names oblique imagery; no download is offered. | `LH Hannover - Geoinformation, CC-BY-4.0` (viewer string, verified); formal wording to be confirmed                     | Open: confirm licence and access path with the city first  | No                                                                                                                  |
| Mapillary                                       | Street-level: street facades, ground floor detail             | Device-dependent                                                   | **Not verified**: the API needs an access token                                                                  | **CC BY-SA 4.0** for user content (verified, terms §3.2), plus Mapillary's own terms (§5, §11, §12)                                    | Mapillary logo plus link (terms §11), and CC BY-SA 4.0 §3(a)(1) creator, licence notice and URI                         | Open: the share-alike question                             | No                                                                                                                  |
| Panoramax                                       | Street-level, here 360° panoramas: street facades             | 5760×2880 equirectangular, 16 px/° (pictures here)                 | 130 pictures in the box on the meta-catalogue, all captured August 2026 (verified, API)                          | **CC-BY-SA-4.0** for every picture in the box (verified, API); licence is per instance, one instance is `etalab-2.0`                   | Producer name, licence and link, per CC BY-SA 4.0 §3(a)(1)                                                              | Open: the share-alike question                             | No                                                                                                                  |
| Own site photos                                 | Whatever was photographed: street facade, courtyard if access | Device-dependent, typically mm-scale at street distance (estimate) | Only what someone photographs                                                                                    | Rights held by the photographer / employer. **Personal data** (GDPR Art. 4(1)).                                                        | None owed to a third party; record the photographer                                                                     | Yes: features, hash and reference. Never the image itself. | No. Images and derived features from a real site never; a synthetic fixture only                                    |
| Client plans (Bauantrag, Ansichten, Grundrisse) | Facades with window positions, floor heights, room use        | Drawing scale (not verified per plan)                              | Only where the client supplies them                                                                              | Architect's copyright (§2(1) Nr. 4 and Nr. 7 UrhG); use right comes from the engagement. Customer data under `data-handling.md`.       | Per engagement; none in public output by default                                                                        | Yes, within the engagement                                 | **Nothing** (`data-handling.md`, customer rows)                                                                     |
| Google Maps, Street View, Earth, 45° imagery    | Roofs and facades                                             | —                                                                  | —                                                                                                                | **Excluded.** Google Maps Platform Terms §3.2.3 (a) and (c), section number unchanged on 2026-10-21; Google Maps consumer terms §2.    | —                                                                                                                       | **No**                                                     | **No**                                                                                                              |

The rest of this note gives the evidence behind each row.

## LGLN DOP20 and DOP10

**Shows.** Nadir orthophotos: roofs, courtyards, rooftop plant, parking areas, lane markings. No
windows. The LGLN product description says that since the 2021 flight year the DOP are produced as
**TrueDOP**, rectified on a building-based surface model (bDOM) to remove building lean (verified,
LGLN OpenGeoData hub item). That makes them right for Phase E (lanes, Parkplätze, Rückkühler on
roofs) and useless for windows: a TrueDOP shows a facade only as the roof edge. The pre-2021
vintages show some building lean at image edges, but not at a resolution or angle that places a
window (inference, not tested).

**Resolution.** "20cm, in Teilen auch 10cm" (verified, hub item). The DOP metadata record's
abstract says 0.4 m before 2007 and 0.2 m since (verified). For the pilot area the STAC catalogue
answers both: `bodenpixelgroesse` 20 for the DOP20 tiles and 10 for the DOP10 tiles (verified).

**Coverage of the pilot area.** A STAC search over the box (`/search?bbox=…`) returned 29 items on
2026-10-21: DOP20 RGBI tiles of 2013-06-18, 2016-08-24, 2019-04-08, 2022-05-08 and 2025-03-06 (flight
name "Hannover"), and nine DOP10 RGBI tiles of 2025-03-06 whose union contains the box (verified).
Each item carries `datetime`, `bildflugname`, `bildflugnummer` and `camera`, which is what an
evidence record needs for its capture date. Access paths: STAC (`dop.stac.lgln.niedersachsen.de`),
a WMS, and Cloud Optimized GeoTIFF assets per tile.

**Licence.** This is the row the plan flagged: public sources quote both dl-de/by-2-0 and CC BY 4.0.

- _Verified, current:_ the DOP20 metadata record (`87890b7a-5a8a-4100-8a1e-78ced663a5d4`, date
  stamp 2026-09-22) states: „Es gelten die Lizenzbedingungen „Creative Commons Namensnennung – 4.0
  International (CC BY 4.0)" bzw. „cc-by/4.0"", with the structured form
  `{"id":"cc-by/4.0", "name":"Creative Commons Namensnennung – 4.0 International (CC BY 4.0)", …}`.
  The STAC collection `DOP` declares `"license": "CC-BY-4.0"`, and so does every one of the 29
  items over the pilot area, the 2013 vintage included.
- _Verified, but only a pointer:_ the LGLN OpenGeoData hub item says the data "können gemäß den
  Allgemeinen Geschäfts- und Nutzungsbedingungen (AGBN) genutzt werden", naming §7 „Nutzung von
  offenen Geodaten".
- _Not verified:_ the AGNB themselves. Every `lgln.niedersachsen.de` URL tried on 2026-10-21 (the
  AGNB page, the AGNB PDF, two product documents) answered with a maintenance notice
  („Aufgrund planmäßiger Wartungsarbeiten ist diese Seite zurzeit nicht zu erreichen").
- _Secondary:_ the OSM wiki page on the Niedersachsen geoportal (last edited 2025-09-12) says the
  DOP were provided under „Datenlizenz Deutschland 2.0 Namensnennung", and that the AGNB as of
  2024-10-01 put open geodata under CC BY 4.0. The OSM community thread dates the move to CC BY 4.0
  to June 2024. That is the likely source of the two licences in circulation: **dl-de/by-2-0 is the
  earlier state, CC BY 4.0 the current one.** The current state is verified from LGLN's own
  metadata; the history is secondary.

**Required attribution.** The metadata record asks for „LGLN" as rights holder and provider and
„das Jahr des Datenbezugs in Klammern", with the example „LGLN (2024) Creative Commons
Namensnennung – 4.0 International (CC BY 4.0)"; the structured field reads „Quelle: LGLN 2024 (Jahr
des letzten Datenbezugs)" (verified). The OSM wiki quotes the AGNB with a different form,
„© GeoBasis-DE/LGLN Jahr, Daten geändert" (secondary, and possibly the older AGNB). Until the AGNB
can be read, use the metadata form. Note that the year is the year of **retrieval**, not of the
flight.

**Derived features.** CC BY 4.0 permits adaptation and commercial use with attribution, so lane
geometry or a parking polygon traced from a DOP may be stored in a project with the attribution in
its evidence record. In the repository the licence would permit it too, but `data-handling.md` has
no row for LGLN imagery ("add a row before using it, not after"), and the pilot fixture is
synthetic regardless.

The **Stadt Hannover DOP20** is a separate product from the city's own biennial spring flight
(listed vintages back to 1981, the latest 2025), covering the whole city area. The city's Open
GeoData terms put its products under „Creative Commons Namensnennung 4.0 International Lizenz – CC
BY 4.0" „soweit nicht anders gekennzeichnet", with the attribution „Datenquelle: CC-BY-4.0 –
Bereich Geoinformation – LH Hannover" (verified). The product page itself states no resolution,
format or download method for current vintages (verified, by absence).

## Oblique aerial imagery

**Shows.** Facades and roofs from four directions: the only aerial source that shows windows.

**Availability.** The city's „Hannover 3D" page says the application offers „Schrägluftbilder
flächendeckend" and „aus vier Perspektiven" (verified). It does not offer them for download, and the
Open GeoData product list does not name oblique imagery at all (verified, by absence). The viewer
(`www.hannover-3d.de`, a VC Map application) publishes its configuration as JSON. It defines three
public oblique collections, „Schrägluftbilder 2017", „2020" and „2024", each named "20 cm - extern",
in EPSG:25832, each with the attribution string `LH Hannover - Geoinformation, CC-BY-4.0`
(verified). The 2024 dataset's declared extent (about 9.596–9.925 E, 52.297–52.461 N) contains the
pilot box. Whether every tile over the pilot area exists was not checked. The configuration also
names a 6 cm 2024 collection on a host that does not resolve publicly (NXDOMAIN on 2026-10-21),
presumably the city's internal service.

**Licence.** The only licence statement found is the viewer's own attribution string, CC BY 4.0.
That is a statement by the rights holder (verified), but it is not a licence document, and the
imagery is offered only as viewer tiles, not as a dataset. Reading a window position off the viewer
by eye is the use the viewer is built for. Fetching the tiles behind it, or hashing them as
evidence, is an access path the city has not offered.

Regional oblique imagery from **Region Hannover** was not found. An LGLN oblique-imagery project
exists by title in a search result ("Ausblick Projekt Schraegluftbilder im LGLN"), with a test area
outside Hannover according to the search summary. Its PDF could not be read (LGLN site maintenance),
so its status is not verified.

**Derived features.** In a project: plausible under CC BY 4.0, but confirm with the city's
Servicecenter Geoinformation first (licence scope and whether a sanctioned access path exists).
Repo: no.

## Mapillary

**Shows.** Street-level photographs: street facades, ground-floor detail, signage. Courtyards only
where someone walked in.

**Coverage of the pilot area.** Not verified. The Graph API answered „Invalid OAuth 2.0 Access
Token" without a token, and no token was used for this note.

**Licence.** Verified from the Mapillary Terms of Use (effective 2024-02-15):

- §3.2: use of other users' content "is subject to the Creative Commons Share Alike (CC BY-SA)
  license". The text names no version; the link goes to `creativecommons.org/licenses/by-sa/4.0/`.
  Mapillary may put some datasets under other terms, for example CC BY-NC-SA.
- §12.1: commercial use is limited to (i) "improvement, training, and development of products,
  services, maps, studies, …" and (ii) "the provision of services for or on behalf of one or more of
  your clients". A noise study for a client falls under (ii). The same section requires safeguards
  that "prohibit reidentification or unblurring of any Content, including any individual or license
  plate", processes against inadvertent disclosure, and incident notification to Meta.
- §5.1 prohibits data mining or extraction methods "not approved by Mapillary", access other than
  through the published interfaces, and "derivative works based upon the Mapillary Services".
- §13 permits use of the services and their content for contributions to OpenStreetMap and Overture.
  Nothing in the terms addresses derived data for other purposes.

**Required attribution.** §11: for images served from one's own servers, and for data Mapillary
extracts and serves through the API, "visibly displaying the Mapillary logo" and linking back. In
addition, CC BY-SA 4.0 §3(a)(1) asks for the creator's identification, a licence notice and a URI
where reasonably practicable. A record would therefore carry the contributor's username, the image
id and URL, and `CC BY-SA 4.0` (composed from both, not quoted from Mapillary).

**Derived features.** Open: see [the share-alike question](#the-share-alike-question). Repo: no.

## Panoramax

**Shows.** Street-level imagery. Every picture over the pilot area is a 360° panorama.

**Coverage of the pilot area.** The federated meta-catalogue (`api.panoramax.xyz`) returned 130
pictures inside the box on 2026-10-21, all captured in August 2026 by one contributor with a 360°
camera (sensor 5760×2880, 16 px per degree horizontally, stated horizontal accuracy 4 m), hosted on
one privately run instance that the instance list shows as `CC-BY-SA-4.0` (verified, API; each
item's `via` link names it). Collections with older dates intersect the box by
extent but returned no pictures inside it. Whether any picture shows a particular facade was not
checked, deliberately: that belongs to the pilot run, outside the tree.

**Licence.** Panoramax is federated, and the licence is per instance. The instance list shows
`CC-BY-SA-4.0` for almost all instances, `etalab-2.0` for the IGN instance (France only) and
`CC0-1.0` for one (verified, instance list). Every picture in the pilot box carries a `license`
link to SPDX `CC-BY-SA-4.0` (verified, API). Wikipedia and the OSM wiki report that the OSM France
instance adds an explicit authorisation to derive non-photographic data under ODbL or CC BY
(secondary). Whether the hosting instance here grants anything similar was not checked; its terms
were not fetched.

**Required attribution.** CC BY-SA 4.0 §3(a)(1): the producer's name as given in the item, the
licence, and the picture's URL. No Panoramax-specific attribution rule was found.

**Derived features.** Open, as for Mapillary. Repo: no.

## The share-alike question

Both street-level sources are CC BY-SA 4.0. The question the plan asks is whether share-alike
reaches a receiver table: if an engineer looks at a CC BY-SA photograph and places a window
rectangle or an Immissionsort from it, is the resulting feature "Adapted Material", which would
have to be shared under CC BY-SA if it is shared at all?

That matters because receiver tables travel. They are exported, printed in reports and handed to
clients and authorities, often under confidentiality. If share-alike reached them, every such
output would have to be offered under CC BY-SA. That is not a rule a client engagement can live with.

The text, verified from the CC BY-SA 4.0 legal code and the Creative Commons FAQ:

- §1(a): Adapted Material is "material subject to Copyright and Similar Rights that is derived from
  or based upon the Licensed Material", modified "in a manner requiring permission".
- §2(a)(2): where exceptions and limitations apply, "this Public License does not apply".
- §4: where sui generis database rights apply, extracting and reusing "all or a substantial portion"
  of the database contents is permitted, and a database in which one has sui generis rights that
  contains them is Adapted Material for the purposes of share-alike.
- FAQ: "Facts are not subject to copyright, nor are the ideas underlying copyrighted content", and
  using the facts within licensed content "will not require attribution" unless the use implicates
  the protected structure.

**Arguments that it is not an adaptation:**

1. A window's position, size and floor are facts about the building, not expression of the
   photograph. The photograph is protected (as a Lichtbildwerk or as a Lichtbild under §72 UrhG),
   but recording where a window is copies none of the photograph's expression. Under §1(a) it is
   therefore not material "requiring permission".
2. The D1 evidence stores four point pairs and a content hash, never pixels. Pixel coordinates of
   four corners are measurements of the image, not a reproduction of it.
3. Priority 18 places one building's windows from a handful of images. That is far from "all or a
   substantial portion" of a contributor's or the platform's database, so §4 does not engage.

**Arguments that it might be, or that the answer is not only a copyright question:**

1. Mapillary's terms are a contract on top of the licence. §5.1 forbids extraction methods "not
   approved by Mapillary" and "derivative works based upon the Mapillary Services". §12 conditions
   commercial use. A feature that is free of copyright can still be a breach of contract.
2. §4 does engage if the method scales. A tool that proposes windows for a whole district from
   street-level imagery (D2 run across a city) is systematic extraction from a database, and the
   resulting feature set may then be Adapted Material under §4(b).
3. A rectified facade image (the D1 homography applied to the photograph) clearly is an adaptation.
   It must never be stored, exported or cached as evidence. The design already says so; this is
   why it must stay that way.
4. The platforms themselves treat derived data as needing a grant. Mapillary's terms carry an
   explicit OSM/Overture permission (§13), and the OSM France Panoramax instance an explicit ODbL/CC
   BY authorisation (secondary). That suggests their operators did not consider derived data free
   of share-alike by default. It is not evidence of the legal position, but a reviewer will raise it.
5. The facts-versus-expression line is national law. Whether an extraction is a reproduction is
   decided under UrhG, not under the licence's wording.

**This note draws no conclusion.** Until the question is answered, CC BY-SA street-level imagery is
evidence a person may _look at_, but Aconiq should not record it as the derived-from source of a
feature in a client project. Own site photos and client plans carry the same information without
the question. See [Open questions](#open-questions), item 3.

## Own site photos

**Shows.** Whatever the photographer captured: the street facade from public ground, the
courtyard facade only with access to the property. **Resolution** depends on the camera; at street
distance a phone camera gives millimetre-scale detail on the facade (estimate, not measured).
**Coverage** is exactly what was photographed.

**Rights.** The photographer holds the rights to the photograph. For an employee on an engagement,
the employer typically holds the use rights (not verified per contract). §59 UrhG permits
photographing works „die sich bleibend an öffentlichen Wegen, Straßen oder Plätzen befinden", but
for buildings only „die äußere Ansicht" (verified), so the architect's copyright in the facade does
not stand in the way of a street-side photo. It does not cover a courtyard reached through private
property, where the owner's permission (Hausrecht) is the relevant question.

**Attribution.** None owed to a third party. Record the photographer and the device in the evidence
record.

**Derived features.** In a project: yes, as features plus a hash and a reference to the image, never
the image. Repo: no; see the next section.

## Site photos are personal data

A photograph of a residential facade shows windows of a dwelling, often people, and number plates.
Its EXIF block carries a capture time and, usually, a GPS position. Each of these is information
relating to an identifiable person in the sense of GDPR Art. 4(1) (text read from a third-party
copy, see [Sources fetched](#sources-fetched)): the residents behind a
window, the passer-by, the car's keeper. A set of photos of one building is personal data about its
residents even when nobody is in frame.

`docs/policies/data-handling.md` already has the shape that fits, and Priority 18 adopts it:
photos are handled like the customer rows of that table.

- **They stay out of the tree.** Not under `testdata/`, not under `docs/`, not under
  `interoperability/` (that directory is for licensed reference material a test reads, and a photo
  is neither). They live with the engagement, on MeKo-managed machines, and are deleted with it.
- **The model stores a hash and a reference, not the image.** The evidence record carries the
  content hash of the file as taken and an opaque reference to where it is held. The D1 point
  pairs are measurements, not pixels. A project folder shared with a client therefore discloses no
  image. It does disclose that a photo exists, when it was taken, and what was measured from it.
- **Transmission is disclosure.** Sending a photo to a hosted vision model (the D2 path) is a
  transfer of personal data to a processor. Whether that is permitted under the engagement, and on
  which legal basis, is decided per engagement before the first image leaves the machine. It is not
  something an agent infers from `--allow-writes`.
- **Nothing in a public output.** Reports and exports carry the derived features and the evidence
  record, not the image or its EXIF.
- **The CI fixture is synthetic** (F2). It must contain nothing from a real photo, including a hash.
  A hash of a real photo in tracked source is a pointer to personal data.

The same holds, with less force, for street-level imagery from Mapillary or Panoramax. Those
platforms blur faces and plates, and Mapillary's §12 forbids undoing that, but a downloaded copy is
still personal data in the hands of whoever holds it.

## Client plans

**Shows.** Ansichten show every facade with window positions and sizes. Schnitte give floor and
slab heights. Grundrisse show room use, which is what decides whether a room is _schutzbedürftig_.
They are the only source that answers that question directly, and the only one that gives "Höhe der
Geschossdecke" as a number rather than an estimate. **Resolution** is the drawing scale (not
verified per plan). **Coverage** is whatever the client supplies.

**Rights.** Architectural drawings are protected as „Werke der Baukunst … und Entwürfe solcher
Werke" and as „Darstellungen wissenschaftlicher oder technischer Art, wie Zeichnungen, Pläne"
(§2(1) Nr. 4 and Nr. 7 UrhG, verified). The right to use them comes from the client's engagement,
and they are customer data under `data-handling.md`: the answer to "may its content enter the
repository?" is **nothing**.

**Attribution.** As the engagement specifies. A report usually names the plan set (author, date,
drawing number) as a source. That is good practice, not a licence term.

**Derived features.** In a project: yes, within the engagement. Repo: nothing.

## Google Maps, Street View and Earth: excluded

Google is **excluded** as a source Aconiq fetches from or records features as derived from.

Verified from the Google Maps Platform Terms of Service on 2026-10-21. The page lists the current
version as "Last modified August 26, 2026". **The section number has not moved**: it is still §3.2.3,
„Restrictions Against Misusing the Services".

- §3.2.3(a) **No Scraping**: "Customer will not export, extract, or otherwise scrape Google Maps
  Content for use outside the Services." The examples include bulk downloading "Street View images".
- §3.2.3(b) **No Caching**, except as the service-specific terms permit.
- §3.2.3(c) **No Creating Content From Google Maps Content**: "Customer will not create content
  based on Google Maps Content." The examples include "(ii) create 3D building models from 45°
  Imagery" and "(v) construct an index of tree locations within a city from Street View imagery".
  A list of window positions read from Street View is the same kind of act as (v). A facade model
  from 45° imagery is (ii).
- §3.2.3(c)(vii) also forbids using Google Maps Content to train, test, validate or fine-tune
  machine-learning models. That closes the D2 path for Google imagery separately.
- "Google Maps Content" is defined as "any content provided through the Services …, including map
  and terrain data, imagery, …".

The Platform terms bind API customers. An engineer in a browser is bound by the **Google Maps /
Google Earth Additional Terms of Service** instead (last modified 2026-01-27, verified). Its §2
„Prohibited Conduct" forbids, among other things, "mass download or create bulk feeds of the
content" and "use Google Maps to create or augment any other mapping-related dataset". The plan's
live constraint calls a desk-study look at Street View the engineer's "own business". The consumer
terms make that narrower than it sounds: looking is fine, but a receiver table built from what was
seen is arguably a "mapping-related dataset". See [Open questions](#open-questions), item 6.

## What the evidence record should carry

This is input to A3, not a schema. Each derived feature's `evidence` names the source it was read
from, in enough detail that a reviewer can find the same pixels or the same sheet again and knows
which terms applied. The six fields the plan names, filled per source kind:

| Kind (`kind`)              | Source id                                                                        | Content hash                                                                                          | Capture date                     | Licence                                                       | Attribution                                               |
| -------------------------- | -------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------- | -------------------------------- | ------------------------------------------------------------- | --------------------------------------------------------- |
| `orthophoto` (LGLN, city)  | STAC item id and asset key (`dop20_rgb`, …), or the WMS request URL              | SHA-256 of the asset as downloaded. For a WMS, of the response, with the request URL that produced it | STAC `datetime` (flight date)    | SPDX `CC-BY-4.0`, plus the terms URL                          | The metadata form, with the year of retrieval             |
| `oblique` (Hannover 3D)    | Collection name and image id, if the city offers a way to address one            | Open: the viewer offers no stable bytes to hash                                                       | Collection year                  | Per the city's answer (open)                                  | `LH Hannover - Geoinformation, CC-BY-4.0` until confirmed |
| `street-level` (Mapillary) | Image id and URL                                                                 | SHA-256 of the image variant used, with its size named (variants differ)                              | The platform's capture timestamp | `CC-BY-SA-4.0`, plus the Mapillary terms version (2024-02-15) | Contributor username, Mapillary, licence, URL             |
| `street-level` (Panoramax) | Item id, collection id, originating instance (`via` link)                        | SHA-256 of the picture as downloaded                                                                  | Item `datetime`                  | The item's `license` link (SPDX id)                           | Producer name, licence, URL                               |
| `site-photo`               | Opaque reference into the engagement's store, never a path that names the client | SHA-256 of the original file, before any EXIF stripping or resizing                                   | EXIF `DateTimeOriginal`          | `own` (photographer, employer)                                | Photographer                                              |
| `plan`                     | Drawing number, revision, sheet                                                  | SHA-256 of the file as received                                                                       | Plan date or revision date       | `client-engagement`                                           | Plan author, date, drawing number                         |

Three things follow for A3:

- Every record should also carry **when it was retrieved** and **the terms it was retrieved under**
  (URL plus version or date). Licences changed under this very note: LGLN moved from dl-de/by-2-0
  to CC BY 4.0 in 2024. A record that says only "CC BY 4.0" cannot later show it was right on the day.
- The **transform** that turns image measurements into the facade frame (D1's four point pairs)
  belongs with the evidence, so the same evidence always yields the same window.
- A `google` kind should not exist. A feature whose only basis is a Google image should not be
  created, rather than created and labelled.

## Open questions

1. **LGLN AGNB §7.** The current licence is verified from LGLN metadata, but the AGNB text, which
   governs, could not be read (site maintenance on 2026-10-21). Re-read it and settle two things:
   the attribution form („LGLN (Jahr)" from the metadata versus „© GeoBasis-DE/LGLN Jahr, Daten
   geändert" quoted secondarily), and the reported duty to notify LGLN when the source note is not
   shown next to an online display.
2. **Attribution year in `lglnimport`.** `lglnimport.Attribution` uses the latest tile `Updated`
   year. The DOP metadata asks for the year of **retrieval** („Jahr des Datenbezugs"). Check whether
   the LoD2 metadata says the same. If it does, the LoD2 attribution needs correcting. That is
   outside A1's scope; it is recorded here because it was found here.
3. **CC BY-SA and derived features.** Is a window or Immissionsort placed by a person looking at a
   CC BY-SA 4.0 photograph Adapted Material, and does Mapillary's §5.1/§12 contract reach it
   independently of copyright? Needs legal review, or a written statement from Mapillary and from
   the Panoramax instance concerned. Until then: look, but do not record it as derived-from.
4. **Hannover 3D oblique imagery.** Ask the Servicecenter Geoinformation whether the oblique
   collections are CC BY 4.0 as the viewer says, whether a sanctioned access path exists (an image
   id to cite, a file to hash), and whether the 6 cm 2024 collection can be licensed.
5. **Mapillary coverage of the pilot area.** Not checked: it needs an API token. Only worth doing
   if question 3 comes out in favour.
6. **The Street View desk-study wording in `PLAN.md`.** The Google Maps consumer terms forbid using
   Google Maps "to create or augment any other mapping-related dataset". The plan's "their own
   business" reads more permissive than that. Whether the plan should say that a desk-study look at
   Street View may inform where someone goes to photograph, but not what is recorded, is a call for
   the plan's owner.
7. **Site photos and hosted vision models.** The legal basis for sending a residential facade photo
   to a hosted model (D2) is decided per engagement. A3 or D2 should define where that decision is
   recorded.
8. **A `data-handling.md` row** for each source here, before any of it is used, as that policy
   requires. At least LGLN/city DOP (CC BY 4.0, may enter with attribution, but not from the pilot
   area), CC BY-SA street-level imagery (pending question 3), site photos (nothing) and client plans
   (nothing, already covered by the customer rows).

## Sources fetched

All fetched on 2026-10-21. "Verified" claims in this note rest on these.

Primary (rights holder or operator):

- LGLN OpenGeoData hub, search API and DOP item (licence pointer to AGNB §7, TrueDOP since 2021,
  20/10 cm, access paths):
  https://ni-lgln-opengeodata.hub.arcgis.com/api/search/v1/collections/all/items?q=Orthophoto&limit=20
  and
  https://ni-lgln-opengeodata.hub.arcgis.com/api/search/v1/collections/all/items/d1a3ea808e0840b4808d8a9ad86f5dea
  (the hub front page, https://ni-lgln-opengeodata.hub.arcgis.com/, renders no text without
  JavaScript)
- LGLN DOP20 metadata record (CC BY 4.0, attribution form):
  https://ni-harvest-prod.geocat.live/catalogue/srv/api/records/87890b7a-5a8a-4100-8a1e-78ced663a5d4/formatters/xml
- LGLN DOP STAC root, collection and search over the pilot box (licence per item, vintages, pixel
  size): https://dop.stac.lgln.niedersachsen.de,
  https://dop.stac.lgln.niedersachsen.de/collections/DOP,
  https://dop.stac.lgln.niedersachsen.de/search?bbox=9.7049,52.3616,9.7233,52.3757&limit=50
- LGLN AGNB page, AGNB PDF and two product PDFs: all answered with a maintenance page.
  https://www.lgln.niedersachsen.de/startseite/wir_uber_uns_amp_organisation/allgemeine_geschafts_und_nutzungsbedingungen_agnb/allgemeine-geschafts-und-nutzungsbedingungen-agnb-97401.html,
  https://www.lgln.niedersachsen.de/download/59483/Allgemeine_Geschaefts-_und_Nutzungsbedingungen_AGNB_.pdf,
  https://lgln.niedersachsen.de/download/126452,
  https://www.lgln.niedersachsen.de/download/143770/Ausblick_Projekt_Schraegluftbilder_im_LGLN.pdf,
  https://www.lgln.niedersachsen.de/download/147825/Schraegluftbilder.pdf
- Stadt Hannover, Open GeoData overview (reached by redirect from https://www.opengeodata-hannover.de):
  https://www.hannover.de/Leben-in-der-Region-Hannover/Verwaltungen-Kommunen/Die-Verwaltung-der-Landeshauptstadt-Hannover/Dezernate-und-Fachbereiche-der-LHH/Baudezernat/Fachbereich-Planen-und-Stadtentwicklung/Geoinformation/Open-GeoData
- Stadt Hannover, Open GeoData terms (CC BY 4.0, attribution):
  https://www.hannover.de/Leben-in-der-Region-Hannover/Verwaltungen-Kommunen/Die-Verwaltung-der-Landeshauptstadt-Hannover/Dezernate-und-Fachbereiche-der-LHH/Stadtentwicklung-und-Bauen/Fachbereich-Planen-und-Stadtentwicklung/Geoinformation/Open-GeoData/Nutzungsbedingungen
- Stadt Hannover, DOP20 page (vintages, coverage):
  https://www.hannover.de/Leben-in-der-Region-Hannover/Verwaltungen-Kommunen/Die-Verwaltung-der-Landeshauptstadt-Hannover/Dezernate-und-Fachbereiche-der-LHH/Stadtentwicklung-und-Bauen/Fachbereich-Planen-und-Stadtentwicklung/Geoinformation/Open-GeoData/Digitale-Orthophotos-DOP20
- Stadt Hannover, Hannover 3D page:
  https://www.hannover.de/Leben-in-der-Region-Hannover/Verwaltungen-Kommunen/Die-Verwaltung-der-Landeshauptstadt-Hannover/Dezernate-und-Fachbereiche-der-LHH/Stadtentwicklung-und-Bauen/Fachbereich-Planen-und-Stadtentwicklung/Geoinformation/Hannover-3D
- Hannover 3D viewer configuration (oblique collections, attribution, extent):
  https://stadtmodell-prod4.hannover-stadt.de/HostingMap-extern/app.config.json, the module
  configurations it lists under `configs/`, and
  https://stadtmodell-prod3.hannover-stadt.de/HostingMap-extern/datasource-data/5a9fc5b4-029a-4adb-bfa7-bd6c1c6b87a6/image.json
- Mapillary Terms of Use (§3.2, §5.1, §11, §12, §13): https://www.mapillary.com/terms. The help
  article https://help.mapillary.com/hc/en-us/articles/115001770409-CC-BY-SA-license-for-open-data
  answered 403. The Graph API (https://graph.mapillary.com/images) refused without a token.
- Panoramax instance list: https://explore.panoramax.fr/instances. Meta-catalogue search over the
  pilot box: https://api.panoramax.xyz/api/search?bbox=9.7049,52.3616,9.7233,52.3757&limit=500 and
  …/api/collections?bbox=…. The OSM France instance API (https://panoramax.openstreetmap.fr/api/)
  answered 403.
- Creative Commons BY-SA 4.0 legal code: https://creativecommons.org/licenses/by-sa/4.0/legalcode.en
- Creative Commons FAQ (adaptation, facts and databases): https://creativecommons.org/faq/
- Google Maps Platform Terms of Service (§3.2.3, definitions, version list):
  https://cloud.google.com/maps-platform/terms
- Google Maps / Google Earth Additional Terms of Service (§2): https://www.google.com/help/terms_maps/
- UrhG §2 and §59: https://www.gesetze-im-internet.de/urhg/__2.html,
  https://www.gesetze-im-internet.de/urhg/__59.html

Secondary (reported, not relied on for a row):

- OSM wiki, Niedersachsen geoportal (dl-de/by-2-0 history, AGNB 2024-10-01, attribution form):
  https://wiki.openstreetmap.org/wiki/DE:Niedersachsen/Geoportal
- OSM community thread on LGLN open geodata (timing of the move to CC BY 4.0):
  https://community.openstreetmap.org/t/nutzung-opengeodaten-niedersachsen/101855
- GDPR Art. 4(1) as reproduced by https://gdpr-info.eu/art-4-gdpr/ (EUR-Lex returned an empty page
  to the fetcher). The text is the regulation's, but the copy is a third party's.
- The Panoramax OSM France derived-data authorisation (ODbL or CC BY), as reported in web search
  summaries of Wikipedia and the OSM wiki (not fetched directly). Mapillary's own OSM/Overture
  permission, by contrast, is verified from its terms (§13).
