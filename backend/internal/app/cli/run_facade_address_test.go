package cli

import (
	"strings"
	"testing"
)

// End to end: the run selects house a by its address, and provenance records
// the feature id it resolved to, which is what keys the generated receivers.
func TestFacadeModeSelectsABuildingByItsAddress(t *testing.T) {
	t.Parallel()

	houses := strings.Replace(facadeHouses, `"id":"a",`, `"id":"a","addr:street":"Musterstraße","addr:housenumber":"1",`, 1)
	projectDir := facadeProject(t, facadeModel(facadeRoad, houses))

	mustRunCLI(t, facadeRunArgs(projectDir, "rls19-road",
		"--receiver-mode", receiverModeFacade, "--facade-building", "Musterstraße 1",
		"--facade-spacing-m", "4", "--allow-unconfirmed")...)

	metadata, _ := facadeRunOutputs(t, projectDir)
	if metadata["facade_buildings"] != `["a"]` {
		t.Errorf("provenance facade_buildings = %q, want [\"a\"]", metadata["facade_buildings"])
	}
}
