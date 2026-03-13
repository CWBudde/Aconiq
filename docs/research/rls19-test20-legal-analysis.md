# TEST-20 Legal Analysis and Redistribution Strategy

## 1. What TEST-20 Is

TEST-20 is the official validation suite for software implementations of RLS-19
(Richtlinien fuer den Laermschutz an Strassen), the German road traffic noise
calculation standard mandated by the 16. BImSchV.

Full title:

> Testaufgaben fuer die Ueberpruefung von Rechenprogrammen nach den Richtlinien
> fuer den Laermschutz an Strassen

- **Publisher:** Bundesanstalt fuer Strassenwesen (BASt)
- **Catalogue number:** FGSV 334/2
- **Current version:** 2.1 (July 2025)
- **Length:** 22 pages

The suite is structured into three categories:

| Category | Tasks | Scope |
|----------|-------|-------|
| Emission (E) | E1 -- E7 | Emission level calculations in isolation |
| Immission (I) | I1 -- I9 | Propagation in reference and check settings |
| Complex (K) | K1 -- K4 | Full urban scenarios with multiple sources and obstacles |

Each task defines input geometry, source parameters, and expected output levels.
A conformance declaration form is provided alongside the tasks PDF for vendors to
certify that their software passes all tasks within stated tolerances.

## 2. How TEST-20 Is Obtained

TEST-20 is available through two channels:

### BASt Website (Federal Agency)

BASt provides the documents as free PDF downloads, no login required:

- **Landing page:**
  <https://www.bast.de/DE/Publikationen/Regelwerke/Verkehrstechnik/Unterseiten/test20.html>
- **Tasks PDF:**
  <https://www.bast.de/DE/Publikationen/Regelwerke/Verkehrstechnik/Downloads/test20-aufgaben.pdf>
- **Conformance form:**
  <https://www.bast.de/DE/Publikationen/Regelwerke/Verkehrstechnik/Downloads/test20-konformitaet.pdf>

### FGSV Verlag (Private Publisher)

The document is catalogued at FGSV Verlag at EUR 0.00 but is marked as available
only to premium subscribers. This dual availability creates the ambiguity
discussed below.

## 3. Copyright Analysis

### German Copyright Law (UrhG) Framework

The relevant provisions of the Urheberrechtsgesetz (UrhG) are:

- **Section 5(1) UrhG:** Laws, ordinances, official decrees, and court decisions
  enjoy no copyright protection.

- **Section 5(2) UrhG:** Other official works (amtliche Werke) published for
  general knowledge are not protected by copyright, provided the source is cited
  and no modifications are made.

- **Section 5(3) UrhG:** Private standards (private Normwerke) **retain full
  copyright** even when they are referenced by or incorporated into legislation.

### Classification of TEST-20

The copyright status of TEST-20 is ambiguous because two entities are involved:

1. **BASt** is a federal research agency (Bundesanstalt) under the Federal
   Ministry for Digital and Transport. Works produced by BASt could qualify as
   amtliche Werke under Section 5(2) UrhG.

2. **FGSV** (Forschungsgesellschaft fuer Strassen- und Verkehrswesen) is a
   private research society. Publications catalogued by FGSV are generally
   treated as private Normwerke under Section 5(3) UrhG, which retain full
   copyright protection.

TEST-20 is published by BASt but catalogued as FGSV 334/2. This dual identity
creates genuine legal uncertainty:

- If TEST-20 is treated as an amtliches Werk (BASt publication), Section 5(2)
  would apply: no copyright, but source citation is mandatory and modifications
  are prohibited.
- If TEST-20 is treated as a privates Normwerk (FGSV publication), Section 5(3)
  applies: full copyright protection regardless of its role in regulatory
  compliance.

### Supporting Evidence

- **Bundestag Research Service (WD 10-045/20)** confirmed that private technical
  standards retain copyright protection even when they become legally binding
  through reference in legislation. This analysis was specifically about the
  intersection of copyright and technical standardization in Germany.

- **Commercial precedent:** Major commercial noise modeling tools (CadnaA,
  SoundPLAN, IMMI) all reference TEST-20 for conformance certification but none
  redistribute the TEST-20 document or its data tables. This industry-wide
  practice strongly suggests that redistribution is not considered permissible.

- **Open-source precedent:** No known open-source project redistributes TEST-20
  content.

### Even Under the Most Favorable Interpretation

Even if Section 5(2) UrhG applied (the most permissive reading), it would
require:

1. Mandatory source citation on every use
2. Prohibition of any modification

These constraints are fundamentally incompatible with the MIT license, which
grants unrestricted rights to modify and redistribute. Embedding TEST-20 content
in an MIT-licensed repository would create an irreconcilable license conflict.

## 4. Redistribution Conclusion

**Conservative answer: do NOT embed the TEST-20 PDF or verbatim data tables in
this repository.**

The reasons are:

1. The Section 5(3) UrhG argument (FGSV copyright) is at least as strong as the
   Section 5(2) argument (free official work). In a dispute, the FGSV
   classification would likely prevail.

2. Even the favorable Section 5(2) interpretation imposes no-modification and
   citation requirements that conflict with MIT licensing.

3. FGSV Verlag actively commercializes its catalogue. The risk of a
   cease-and-desist (Abmahnung) is real and would be costly to defend.

4. No commercial or open-source project has established a precedent of
   redistribution.

## 5. Our Two-Tier Strategy

We adopt a two-tier validation approach that achieves full coverage without
redistributing any copyrighted material:

| Tier | Location | Content | CI-safe? |
|------|----------|---------|----------|
| CI-safe suite | In-repo (`testdata/ci_safe/`) | Repo-authored scenarios covering every TEST-20 category with independent geometry | Yes |
| Local suite | Outside repo | Extracted TEST-20 tasks from lawfully obtained PDF | No (opt-in) |

### CI-Safe Suite (Tier 1)

The CI-safe suite contains entirely original test scenarios authored by the
project contributors. These scenarios are designed to exercise every calculation
path that TEST-20 covers (emission E1--E7, immission I1--I9, complex K1--K4) but
use independent geometry, source parameters, and expected values derived from our
own implementation.

This suite:

- Proves that all calculation paths produce correct results
- Runs in CI on every commit with no legal risk
- Is MIT-licensed like the rest of the repository

### Local Suite (Tier 2)

The local suite allows developers who have lawfully obtained the TEST-20 PDF to
verify exact numeric conformance against the official expected values. Test data
is extracted from the PDF by the developer and stored outside the repository.

This suite:

- Proves exact numeric match against official BASt/FGSV expected values
- Is opt-in and never committed to the repository
- Is activated via `--mode local-suite --local-suite-dir <path>`

The runner supports both tiers:

```
# CI mode (default, runs in-repo tests only)
--mode ci-safe

# Local mode (requires path to extracted TEST-20 data)
--mode local-suite --local-suite-dir <path>
```

## 6. References

- BASt TEST-20 landing page:
  <https://www.bast.de/DE/Publikationen/Regelwerke/Verkehrstechnik/Unterseiten/test20.html>
- UrhG (German Copyright Act):
  <https://www.gesetze-im-internet.de/urhg/>
- Bundestag Research Service WD 10-045/20 on copyright of referenced technical standards:
  <https://www.bundestag.de/resource/blob/817176/wissenschaftliche-dienste>
- FGSV Verlag catalogue:
  <https://www.fgsv-verlag.de/>
