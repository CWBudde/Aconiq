package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/qa/golden"
	"github.com/aconiq/backend/internal/standards"
	"github.com/aconiq/backend/internal/standards/framework"
)

// This file is the output oracle for restructuring the run pipeline. The nine
// TestRun<Standard>ProducesOutputs tests assert that particular files exist and
// that a handful of fields inside them look right; none of them would notice a
// receiver level drifting in the last digit, a column disappearing from the CSV,
// or a raster losing a band. A refactor of the per-standard dispatch has to be
// provably output-identical, so what is pinned here is every byte every
// registered standard writes.
//
// Two things are held, for different reasons:
//
//   - files: a SHA-256 over each artifact, because most of them are large and
//     a hash is the only honest way to say "nothing moved".
//   - run_summary and provenance: their normalized content verbatim, because
//     they are small, they carry the metadata a pipeline refactor is most
//     likely to perturb, and a hash mismatch there would tell a reader nothing
//     about what actually changed.

// These are the keys that differ between two identical runs and therefore
// cannot be part of a snapshot. Keeping the lists this short is the point:
// everything not named here is asserted byte for byte.
//
//   - created_at  — results.SaveRaster stamps time.Now().UTC() into every
//     raster metadata sidecar.
//   - run_id      — run-summary.json takes it from the run directory name, and
//     projectfs.buildID draws it from crypto/rand.
//   - generated_at, tool_version — provenance.json only; the clock again, and
//     the build stamp, which moves with the checkout rather than with the code
//     under test.
var (
	// volatileResultFields is applied to every JSON file under results/,
	// whichever of the two keys that file happens to carry.
	volatileResultFields = []string{"created_at", "run_id"}
	// The next two are per-file, and every key they name must be present:
	// a field that has gone missing means the snapshot has silently started
	// pinning something it should not, or has stopped normalizing something
	// it still must.
	volatileRunSummaryFields = []string{"run_id"}
	volatileProvenanceFields = []string{"run_id", "generated_at", "tool_version"}
)

// runResultsDigest is the snapshot value. It is a struct rather than a bare map
// so the golden file names what each section is.
type runResultsDigest struct {
	Standard   string            `json:"standard"`
	Fixture    string            `json:"fixture"`
	Files      map[string]string `json:"files"`
	RunSummary map[string]any    `json:"run_summary"`
	Provenance map[string]any    `json:"provenance"`
}

// Every registered standard is run end to end, twice, into two separate
// projects. The two digests must agree before either is compared against the
// golden file: that is what proves the normalization above is complete, and so
// that a golden mismatch means the outputs changed rather than that the harness
// leaked a clock.
func TestRunResultsDigestsAreStable(t *testing.T) {
	t.Parallel()

	registry, err := standards.NewRegistry()
	if err != nil {
		t.Fatalf("new standards registry: %v", err)
	}

	descriptors := registry.List()
	if len(descriptors) == 0 {
		t.Fatal("standards registry is empty")
	}

	for _, descriptor := range descriptors {
		t.Run(descriptor.ID, func(t *testing.T) {
			t.Parallel()

			first := digestOneRun(t, descriptor.ID, descriptor.EvidenceTier)
			second := digestOneRun(t, descriptor.ID, descriptor.EvidenceTier)

			assertDigestsAgree(t, first, second)

			golden.AssertJSONSnapshot(t, testdataPath(t, "digest", descriptor.ID+".golden.json"), first)
		})
	}
}

// assertDigestsAgree compares two runs of the same standard field by field, so
// a reproducibility failure names the artifact that moved instead of printing
// two large structures.
func assertDigestsAgree(t *testing.T, first, second runResultsDigest) {
	t.Helper()

	for name, digest := range first.Files {
		other, ok := second.Files[name]
		if !ok {
			t.Errorf("%s: second run did not write it", name)

			continue
		}

		if digest != other {
			t.Errorf("%s: two identical runs disagree: %s vs %s\n"+
				"Either the run is nondeterministic or this file carries a field volatileResultFields does not name.", name, digest, other)
		}
	}

	for name := range second.Files {
		_, ok := first.Files[name]
		if !ok {
			t.Errorf("%s: only the second run wrote it", name)
		}
	}

	assertJSONEqual(t, "run-summary.json", first.RunSummary, second.RunSummary)
	assertJSONEqual(t, "provenance.json", first.Provenance, second.Provenance)
}

func assertJSONEqual(t *testing.T, label string, first, second map[string]any) {
	t.Helper()

	firstJSON := mustMarshalCanonical(t, first)

	secondJSON := mustMarshalCanonical(t, second)
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Errorf("%s: two identical runs disagree\nfirst:\n%s\nsecond:\n%s", label, firstJSON, secondJSON)
	}
}

// digestOneRun drives init → import → run into a fresh project and reduces
// everything the run left behind to a comparable value.
func digestOneRun(t *testing.T, standardID string, tier framework.EvidenceTier) runResultsDigest {
	t.Helper()

	fixture, ok := registryRunFixtures[standardID]
	if !ok {
		t.Fatalf("standard %q is registered but declares no run fixture; add one rather than skipping it", standardID)
	}

	projectDir := t.TempDir()

	mustRunCLI(t, "--project", projectDir, "init", "--name", "ResultsDigest", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", testdataPath(t, fixture...))

	tierArgs := tierRunArgs[tier]

	args := make([]string, 0, 6+len(tierArgs))
	args = append(args, "--project", projectDir, "run", "--standard", standardID)
	args = append(args, tierArgs...)

	mustRunCLI(t, args...)

	run := digestRunRecord(t, projectDir, standardID)
	runDir := filepath.Join(projectDir, ".noise", "runs", run)

	return runResultsDigest{
		Standard:   standardID,
		Fixture:    strings.Join(fixture, "/"),
		Files:      digestResultsTree(t, filepath.Join(runDir, "results")),
		RunSummary: readNormalizedJSON(t, filepath.Join(runDir, "results", "run-summary.json"), volatileRunSummaryFields),
		Provenance: readNormalizedJSON(t, filepath.Join(runDir, "provenance.json"), volatileProvenanceFields),
	}
}

// digestRunRecord returns the id of the single run the project holds, failing
// if the run did not complete — a digest of a failed run's leftovers would be a
// snapshot of the failure.
func digestRunRecord(t *testing.T, projectDir, standardID string) string {
	t.Helper()

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if len(proj.Runs) != 1 {
		t.Fatalf("expected exactly one run, got %d", len(proj.Runs))
	}

	run := proj.Runs[0]
	if run.Standard.ID != standardID {
		t.Fatalf("expected run standard %q, got %q", standardID, run.Standard.ID)
	}

	if run.Status != project.RunStatusCompleted {
		t.Fatalf("expected completed run status, got %q", run.Status)
	}

	return run.ID
}

// digestResultsTree hashes every file under the results directory. JSON files
// are normalized first; everything else is hashed as written, which is what
// makes the raster .bin payload part of the guarantee.
func digestResultsTree(t *testing.T, resultsDir string) map[string]string {
	t.Helper()

	digests := make(map[string]string)

	err := filepath.WalkDir(resultsDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			return nil
		}

		relative, relErr := filepath.Rel(resultsDir, path)
		if relErr != nil {
			return fmt.Errorf("relativize %s: %w", path, relErr)
		}

		payload, readErr := digestPayload(path)
		if readErr != nil {
			return readErr
		}

		sum := sha256.Sum256(payload)
		digests[filepath.ToSlash(relative)] = fmt.Sprintf("sha256:%s bytes=%d", hex.EncodeToString(sum[:]), len(payload))

		return nil
	})
	if err != nil {
		t.Fatalf("walk results %s: %v", resultsDir, err)
	}

	if len(digests) == 0 {
		t.Fatalf("results directory %s is empty", resultsDir)
	}

	return digests
}

// digestPayload returns the bytes to hash for one artifact: the raw file, or
// for JSON a re-serialization with the volatile fields removed.
func digestPayload(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	if filepath.Ext(path) != ".json" {
		return raw, nil
	}

	decoded, err := decodeJSONObject(raw)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}

	dropFields(decoded, volatileResultFields)

	normalized, err := json.Marshal(decoded)
	if err != nil {
		return nil, fmt.Errorf("re-encode %s: %w", path, err)
	}

	return normalized, nil
}

// readNormalizedJSON decodes one JSON object with the named fields removed,
// requiring each of them to have been there. Numbers keep their literal
// spelling (json.Number), so a change in how a float is formatted is a change
// the snapshot sees.
func readNormalizedJSON(t *testing.T, path string, volatile []string) map[string]any {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	decoded, err := decodeJSONObject(raw)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}

	for _, field := range volatile {
		_, present := decoded[field]
		if !present {
			t.Fatalf("%s carries no %q field; the volatile-field list is stale and the snapshot may now be pinning a clock", filepath.Base(path), field)
		}
	}

	dropFields(decoded, volatile)

	return decoded
}

func decodeJSONObject(raw []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	var decoded map[string]any

	err := decoder.Decode(&decoded)
	if err != nil {
		return nil, fmt.Errorf("decode json object: %w", err)
	}

	return decoded, nil
}

func dropFields(object map[string]any, fields []string) {
	for _, field := range fields {
		delete(object, field)
	}
}

func mustMarshalCanonical(t *testing.T, value any) []byte {
	t.Helper()

	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal snapshot value: %v", err)
	}

	return encoded
}
