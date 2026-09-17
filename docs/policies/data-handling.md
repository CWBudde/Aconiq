# Data Handling Policy

Status date: 2026-09-17

## The decision

Licensed third-party material lives under `interoperability/`, on the machines entitled to hold it,
and **never in git**. `.gitignore` excludes the directory wholesale;
`just check-no-third-party-data` refuses any commit whose index contains a path under it, and
`.github/workflows/repo-hygiene.yml` runs that same recipe as a status check of its own.

The alternative considered was a private repository — or a private submodule — holding the material
so that CI and every developer get it automatically. It was rejected because entitlement here is
**per holder, not per project**: the FGSV and ISO texts are licensed to a person or a seat, and the
SoundPLAN bundles are a customer's data held under an engagement. A repository is a distribution
channel, and "private" is not a licence term. Worse, the failure is irreversible: a wrong `go vet`
fix is a revert away, but a copy that has been pushed cannot be un-given. The guard is therefore
placed where it can hold a line — the index — and everything else in this document is the part no
tool checks.

This is the operational half of `docs/preflight/compliance-boundaries.md`. That document carries the
legal frame — §2 on standards texts, §4 on test data, §6 on the SoundPLAN interoperability boundary.
This one says where the material sits, who may hold it, how a fixture reaches a test run without
being tracked, and what to do when it escapes.

## What may be stored under `interoperability/`

Lawfully obtained reference material that the repository is developed **against** but must not
contain: standards documents, their annexes and correction sheets, and vendor or customer project
bundles that a local test run reads as input. Nothing else. It is not a scratch area for source,
for generated artefacts, or for anything that belongs in the repository and is merely inconvenient
to commit.

"Input" is the load-bearing word, and `compliance-boundaries.md` §6 is where it matters: real
SoundPLAN project files are "never used as fixtures in this repository". That is the same rule as
this one rather than a competing one. A _fixture_ is checked in and travels with the repository,
and no customer bundle ever becomes one. What a bundle may be is an untracked local input, found at
run time by the mechanism the next section describes, present on an entitled machine and nowhere
else — and a test that cannot find it skips.

What matters for each item is not whether it may be _stored_ there — it may — but whether its
content may cross into tracked source. That answer differs per source, and the distinction that
decides it is whether the document is an **amtliches Werk** under §5 UrhG or a **third-party
licensed** work.

| Path                                             | What it is                                                                                       | Rights                                                                                     | May its content enter the repository?                                                                                                                                                                   |
| ------------------------------------------------ | ------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `interoperability/Schall03/`                     | Anlage 2 zu §4 der 16. BImSchV, and the 2014 amending ordinance                                  | Amtliches Werk, §5 UrhG — no copyright                                                     | **Yes, the coefficients.** They are embedded directly, with a citation comment (`beiblatt1.go`, `beiblatt2.go`, `beiblatt3.go`, the Tabellen). The PDFs themselves stay out, like everything else here. |
| `interoperability/RLS-19/`                       | The FGSV text incl. Korrekturblatt 2/2020, the Allgemeines Rundschreiben, the BT-Drucksache      | FGSV-published; **not clearly** amtliches Werk                                             | **No text, no table.** Cite section and equation numbers; a coefficient read from the text carries that citation. See the scratchpad rule below.                                                        |
| `interoperability/ISO9613-2/`                    | ISO 9613-2:1996 and :2024                                                                        | Private Normwerk. §5(3) UrhG says explicitly that being referenced by law does not free it | **No text, no table layout.** Values derived from a lawfully obtained copy, referenced by table and equation number.                                                                                    |
| `interoperability/TA-Laerm/`                     | TA Lärm and the LAI-Hinweise                                                                     | Verwaltungsvorschrift — amtliches Werk; the LAI-Hinweise are a published official aid      | Threshold tables may be embedded with a citation, as in `assessment/talaerm`. Commentary and layout stay out.                                                                                           |
| _(a SoundPLAN bundle directory, not named here)_ | A customer's SoundPLAN project bundle                                                            | Customer data, and personal data (addresses, identifiable project detail)                  | **Nothing.** Not the geometry, not a receiver row, not the project name. See `compliance-boundaries.md` §6.                                                                                             |
| _(a consulting engagement directory, not named)_ | Live consulting projects — Gutachten, tender documents, DWG drawings, a nested SoundPLAN project | Customer data; some of it under tender confidentiality                                     | **Nothing.**                                                                                                                                                                                            |

The two customer rows name no directory, deliberately. A path, a project name or a customer name in
tracked source is itself a disclosure — the rule step 2 of the next section states, and the one
`26ce6df` acted on when it made the fixture resolver key on a marker file. This document is tracked,
so it is bound by that rule like any other file; the directories are found by content, never by a
name written down here.

The rule that generalises: **the numbers a standard fixes may be typed into Go with a citation; the
document's prose and its table layout may not.** Where the document is an amtliches Werk under §5
UrhG the table itself is free, which is why Schall 03's Anlage 2 is embedded outright. Where it is a
copyrighted Normwerk the _values_ still travel — derived from a lawfully obtained copy and
referenced by table and equation number, which is what `iso9613/atmospheric.go` and
`rls19/road/tables.go` do — while the text, the layout, and anything that would substitute for
buying the document stay where they are. That is `compliance-boundaries.md` §2 in one sentence, and
it is why correcting a coefficient is always permissible. A source not in the table above has no
answer yet — add a row before using it, not after.

Two things may never be stored there at all, whatever the directory's status: material obtained
under terms that forbid holding a copy, and a customer bundle whose engagement has ended and whose
retention period has run out.

## Who may hold it, and on what terms

- **Only the individually entitled.** For a standards document that means a licence of your own or
  one of MeKo's seats; for a customer bundle it means a live engagement that covers the work.
  Membership of this project entitles nobody to anything.
- **On MeKo-managed machines.** Not on a personal device, not in personal cloud storage, not on a
  share the whole company can read.
- **No onward sharing.** A colleague who needs the RLS-19 text obtains it from FGSV or from a MeKo
  seat. Passing your copy on is redistribution, and it is the act the licence forbids.
- **Transmission is disclosure.** Sending the content anywhere off the machine — a hosted model, an
  issue, a chat, a CI log, a pastebin — is a disclosure, judged by the same entitlement test as
  handing over the file. For the standards documents, that test is about the recipient's rights. For
  the customer bundles there is no test to apply: the answer is never.
- **Deletion is part of holding it.** A customer bundle is deleted when the engagement's retention
  period ends, not when the branch is merged.

## How a licensed fixture reaches a test run without being tracked

One resolver does this, and tests must go through it: `internal/qa/fixtures.SoundPLANProjectDir`.

1. **`ACONIQ_SOUNDPLAN_FIXTURES` wins.** Set and non-empty, it names the project directory, and the
   data can therefore sit anywhere — a mounted volume on a CI runner, an external disk, a directory
   with a name nobody wants in a log. A value that is set but does not name a SoundPLAN project is a
   hard `t.Fatal`, deliberately: someone stated where the data is and was wrong, and a skip would
   bury that.
2. **Otherwise, discovery by content.** The resolver scans the immediate children of the repository
   root's `interoperability/` for a directory containing a `Project.sp`. Exactly one is used.
   Neither of the other two outcomes fails a test: finding none and finding two or more both come
   back as errors from discovery, and `SoundPLANProjectDir` turns every discovery error into a
   `t.Skipf`. So an ambiguous `interoperability/` skips, with the reason printed, rather than
   stopping the run — consistent with point 1, in that only a _stated_ location that proves wrong
   is worth a hard failure, but it belongs with the hazards below and not among the guarantees.
   Keying on the marker file
   rather than on a directory name is what removed a customer project name from tracked source
   (`26ce6df`), and it is the pattern any future licensed fixture must follow: **discover by
   content, never by name.** A path, a project name or a customer name in tracked source is itself a
   disclosure, however small.
3. **A missing fixture skips.** A clean checkout legitimately has none, so the tests that need it
   call `t.Skip` and the run is green.

That third point is a hazard, and naming it is the point of writing it down. CI has no fixture
today, so `internal/io/soundplanimport` and the `aconiq compare` pipeline test assert nothing there,
and the summary says "ok" all the same. **A green CI is not evidence that the SoundPLAN path
works** — it is, right now, evidence that the most consequential comparison in the project did not
run. `PLAN.md` Priority 3 tracks closing that.

One limit of the discovery rule, worth knowing before trusting it: it is one level deep. A SoundPLAN
project nested inside another directory under `interoperability/` — a customer engagement folder
with its own `SP_<n>/Project.sp`, for instance — is invisible to it, so the ambiguity error does not
fire for it and the top-level bundle is chosen silently. If a nested project is the one you want,
point `ACONIQ_SOUNDPLAN_FIXTURES` at it. Do not promote it to the top level.

### What would have to be true to put reference data into CI

`PLAN.md` Priority 3 leaves open whether a submodule or Git LFS could carry the reference project so
the comparison runs in CI, "licence permitting". The licence is the whole question, and the bar is
higher than the phrasing suggests:

1. **Both mechanisms are distribution.** A submodule hands a copy to every CI runner and to everyone
   with read access; LFS stores the bytes in the repository's own object store. Neither is a way of
   _not_ redistributing. So under the rights held today neither can carry the ISO texts, the FGSV
   texts, or a customer bundle — and tightening access control does not reach it, because the
   question is the right to redistribute and not the size of the audience. Only a rights grant
   reaches it, which is what point 2 is about.
2. **The only admissible route is material whose rights we hold.** A synthetic SoundPLAN project,
   built on an entitled seat from invented geometry and containing no customer's data, qualifies. So
   does a customer's written permission that names redistribution specifically — consent to use is
   not consent to publish.
3. **It must be the right vintage to be worth it.** The present reference project is a Schall 03
   _1990_ model, so even a perfect comparison measures the 1990 → 2014 method change as well as this
   implementation (`PLAN.md` Priority 3). A fixture that gets into CI but cannot settle a conformance
   question has bought little.
4. **It does not go under `interoperability/`.** The guard refuses _any_ tracked path there, by
   design, and that is not to be relaxed to make room: a licence-clear fixture is not third-party
   data and belongs in a `testdata/` directory next to the package that reads it. Weakening the
   guard to admit one safe file removes the line for every unsafe one.

Anything added under 2 gets a row in the table above before it lands.

## Extracted standard text stays in a scratchpad

Reading a scanned standard means extracting text and rendering pages. Those extracts _are_ the
standard's text and carry its rights; the extraction does not launder them.

- They live outside the repository — an agent scratchpad, a temporary directory — and are deleted
  when the task ends. They never become a file under `docs/`, a block comment, a testdata fixture, or
  a commit message.
- What crosses into the repository is the **citation** — section, equation, table number — and, where
  the table above allows it, the value that citation identifies.
- Schall 03's Anlage 2 is the one source where copying the numbers in _is_ the policy rather than a
  breach of it, because §5 UrhG frees them. RLS-19 is the case that motivated the rule: it is
  FGSV-published and not clearly an amtliches Werk (`PLAN.md` Priority 1.5).
- A practical warning from the same pass: `pdftotext -layout` is not sufficient evidence. It drops
  terms from stacked fractions and flattens crossed-out cells, and two defects were invisible in the
  extraction while an apparent third was an extraction artefact. Render the page and read it.

## What the guard does not catch

`check-no-third-party-data` runs `git ls-files -- interoperability/` and fails if the output is
non-empty. That catches exactly one thing: a path under that directory present in the commit's
index — from `git add -f`, from a file that was tracked before `.gitignore` gained the line, or from
a branch that predates the rule. It is worth being blunt about the rest.

- **It is not a content scanner.** A Tabelle 4a pasted into a Go file, a receiver table from the
  customer bundle saved as a testdata CSV, a customer address in a commit message, a scanned page
  committed under `docs/` — every one of those passes green. The guard bounds a directory, not a
  kind of content, and it cannot be extended to do the latter.
- **It does not read history.** A tracked file deleted in the tip commit leaves a clean index while
  the content is still reachable from an earlier commit on the branch. The recipe's own failure text
  says so: untracking is not enough once the commit has been pushed.
- **It says nothing about ignoring.** `.gitignore` and the guard are independent; the guard checks
  that nothing is tracked, not that anything is ignored.
- **It is scoped to `interoperability/`.** Third-party material stored anywhere else is outside it
  entirely.

## If it leaks

The procedure, in order. Step 1 decides which of two different situations you are in.

1. **Establish whether the branch was pushed.** If it exists only locally, rewrite the history
   (`git rebase -i`, or `git filter-repo` for anything older), confirm with
   `just check-no-third-party-data`, and continue. The incident ends here and needs no record.
2. **If it was pushed, treat the material as disclosed.** It is not a question of how long it was up
   or whether anyone looked. GitHub keeps unreachable objects fetchable by SHA, forks retain them
   independently, and CI logs and artifacts may hold their own copies. Deleting the file is not
   deleting the data.
3. **Stop the spread first.** Close the pull request rather than pushing a fix commit onto it — the
   PR page keeps every diff it ever showed. Delete the branch, and delete the workflow runs,
   artifacts and caches that contain the content.
4. **Then rewrite and purge.** `git filter-repo` over every affected ref, force-push, and ask GitHub
   Support to expire the cached views and collect the unreachable objects. Forks must be deleted,
   because a fork holds the objects on its own.
5. **Notify, according to what leaked.** A standards publisher's text (FGSV, ISO, DIN): tell the
   repository owner; the exposure is a licence breach and the remedy is removal plus a written
   record. A customer's material: the customer is told in every case, because the confidentiality
   obligation is breached whether or not anyone is identifiable in what leaked — a tender document
   or a DWG drawing can be purely commercial. Whether it is _also_ a personal-data breach is a
   second question, answered from the content rather than assumed: a SoundPLAN bundle carries
   addresses and identifiable project detail, which is the case `compliance-boundaries.md` §6 has
   in mind, while the table above covers material that carries none. Where the answer is yes,
   whether Art. 33 GDPR requires a supervisory-authority notification within 72 hours is decided by
   MeKo and not by the person who pushed it. The clock starts at awareness, so write down when
   awareness began — for that assessment as much as for the notification.
6. **Record it here**, under a dated heading, the way `vulnerability-scanning.md` requires for an
   advisory with no fix: what leaked, from which refs, when it was pushed, when it was removed, and
   who was told. An incident that is embarrassing is still one the next person has to be able to
   find.
7. **Then ask what let it through**, and whether the guard can be made to catch that class at all.
   For a pasted table it cannot — see the section above — and saying so here is better than leaving a
   reader to assume CI covers it.

### Recorded disclosures

None.
