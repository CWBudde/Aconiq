package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/projectfs"
)

// A confirmed receiver on house a's south facade whose key names a building
// that is gone: the shape an LGLN import leaves behind when it replaces the
// OSM copy of a building under a new id.
func orphanedFacadeProject(t *testing.T) string {
	t.Helper()

	orphaned := strings.Replace(facadeProperties("confirmed"), `"building_ref":"a"`, `"building_ref":"osm-way-gone"`, 1)

	return facadeProject(t, facadeModel(facadeRoad, facadeHouses, facadeReceiver("r1", orphaned)))
}

func savedModel(t *testing.T, projectDir string) (modelgeojson.Model, string) {
	t.Helper()

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}

	raw, hash, err := store.ReadModelWithHash()
	if err != nil {
		t.Fatalf("read model: %v", err)
	}

	model, err := modelgeojson.Normalize(raw, "EPSG:25832", "model")
	if err != nil {
		t.Fatalf("normalize model: %v", err)
	}

	return model, hash
}

func savedReceiver(t *testing.T, projectDir, id string) modelgeojson.Feature {
	t.Helper()

	model, _ := savedModel(t, projectDir)
	for _, feature := range model.Features {
		if feature.ID == id {
			return feature
		}
	}

	t.Fatalf("receiver %q not in the saved model", id)

	return modelgeojson.Feature{}
}

// End to end: a broken link blocks a normative run; a dry run proposes the
// building that has the edge and writes nothing; --apply re-keys the receiver
// and marks it proposed, which the run then refuses without the opt-in and
// computes with it.
func TestReattachFacadesEndToEnd(t *testing.T) {
	t.Parallel()

	projectDir := orphanedFacadeProject(t)
	runArgs := facadeRunArgs(projectDir, "rls19-road", "--receiver-mode", receiverModeCustom)

	requireErrorContaining(t, runCLI(append(runArgs, "--allow-unconfirmed")...), modelgeojson.CodeReceiverFacadeUnresolved)

	_, before := savedModel(t, projectDir)

	dry := runCLIOutput(t, "--project", projectDir, "reattach-facades")
	for _, part := range []string{"r1", "proposed", `"a"`, "--apply"} {
		if !strings.Contains(dry, part) {
			t.Fatalf("dry run output %q does not contain %q", dry, part)
		}
	}

	if _, after := savedModel(t, projectDir); after != before {
		t.Fatal("a dry run changed the model")
	}

	mustRunCLI(t, "--project", projectDir, "reattach-facades", "--apply")

	receiver := savedReceiver(t, projectDir, "r1")

	facade, _ := receiver.Properties[modelgeojson.PropertyFacade].(map[string]any)
	if facade["building_ref"] != "a" || receiver.Properties[modelgeojson.PropertyReviewStatus] != geo.ReviewStatusProposed {
		t.Fatalf("receiver properties = %v, want re-keyed to a and proposed", receiver.Properties)
	}

	requireErrorContaining(t, runCLI(runArgs...), "1 proposed", "--allow-unconfirmed")
	mustRunCLI(t, append(runArgs, "--allow-unconfirmed")...)
}

// Under --json stdout is exactly one JSON document naming every outcome.
func TestReattachFacadesJSON(t *testing.T) {
	t.Parallel()

	projectDir := orphanedFacadeProject(t)

	var out struct {
		Command       string                            `json:"command"`
		Applied       int                               `json:"applied"`
		MaxShiftM     float64                           `json:"max_shift_m"`
		Counts        map[string]int                    `json:"counts"`
		Reattachments []modelgeojson.FacadeReattachment `json:"reattachments"`
	}

	stdout := runCLIOutput(t, "--json", "--project", projectDir, "reattach-facades")

	err := json.Unmarshal([]byte(stdout), &out)
	if err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, stdout)
	}

	if out.Command != "reattach-facades" || out.Applied != 0 || out.MaxShiftM != modelgeojson.FacadeMoveToleranceM ||
		out.Counts[modelgeojson.ReattachProposed] != 1 || len(out.Reattachments) != 1 ||
		out.Reattachments[0].Proposed == nil || out.Reattachments[0].Proposed.BuildingRef != "a" {
		t.Fatalf("output = %+v", out)
	}
}

// A model with nothing to re-attach answers an empty list, which a consumer
// can iterate, and writes nothing under --apply.
func TestReattachFacadesWithNothingFlagged(t *testing.T) {
	t.Parallel()

	projectDir := facadeProject(t, facadeModel(facadeRoad, facadeHouses, facadeReceiver("r1", facadeProperties("confirmed"))))
	_, before := savedModel(t, projectDir)

	stdout := runCLIOutput(t, "--json", "--project", projectDir, "reattach-facades", "--apply")
	if !strings.Contains(stdout, `"reattachments": []`) {
		t.Fatalf("stdout = %s, want an empty reattachments list", stdout)
	}

	if _, after := savedModel(t, projectDir); after != before {
		t.Fatal("--apply with nothing to re-attach changed the model")
	}
}

// A model saved after the command read it is not overwritten: the save is a
// compare-and-write against the hash the proposals were made from.
func TestReattachFacadesRefusesAModelThatChangedMeanwhile(t *testing.T) {
	t.Parallel()

	projectDir := orphanedFacadeProject(t)

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}

	proposal, err := proposeProjectReattachments(store, modelgeojson.FacadeMoveToleranceM)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}

	// Another writer replaces the model in between.
	modelPath := filepath.Join(projectDir, "other.geojson")

	err = os.WriteFile(modelPath, []byte(facadeModel(facadeRoad, facadeHouses)), 0o600)
	if err != nil {
		t.Fatalf("write model: %v", err)
	}

	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)

	_, err = proposal.save(store)
	if err == nil || !domainerrors.IsUserError(err) || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("save over a changed model: err = %v, want a user error saying the model changed", err)
	}

	if model, _ := savedModel(t, projectDir); len(model.Features) != 3 {
		t.Fatalf("the other writer's model was overwritten: %d features", len(model.Features))
	}
}

func TestReattachFacadesRefusals(t *testing.T) {
	t.Parallel()

	projectDir := orphanedFacadeProject(t)

	for _, shift := range []string{"0", "-1", "NaN", "+Inf"} {
		err := runCLI("--project", projectDir, "reattach-facades", "--max-shift-m", shift)
		if err == nil || !domainerrors.IsUserError(err) || !strings.Contains(err.Error(), "--max-shift-m") {
			t.Fatalf("--max-shift-m %s: err = %v, want a user error naming the flag", shift, err)
		}
	}
}
