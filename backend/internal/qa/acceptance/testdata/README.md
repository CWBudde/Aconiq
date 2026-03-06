These fixtures are repo-authored synthetic scenarios used as deterministic,
license-safe acceptance evidence for the currently online standards modules.

They are not normative public validation datasets. They centralize scenario and
expected-output snapshots under `internal/qa/acceptance/` so acceptance checks
can run independently of the owning package tests.

`rls19-road` also has a dedicated `internal/qa/acceptance/rls19_test20`
runner. Its `ci-safe` suite is derived from repo-authored scenarios and public
TEST-20 task categories, while `local-suite` mode is reserved for non-committed
external extractions.
