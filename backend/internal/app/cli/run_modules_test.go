package cli

import (
	"testing"

	"github.com/aconiq/backend/internal/standards"
)

// The dispatch table and the registry have to agree in both directions: a
// standard the registry offers but the table does not carry cannot be run, and
// a table entry for a standard nobody registers is dead wiring that the
// end-to-end fixtures would never exercise. Neither is visible from a run that
// happens to pass.
func TestRunModuleTableMatchesTheRegistry(t *testing.T) {
	t.Parallel()

	registry, err := standards.NewRegistry()
	if err != nil {
		t.Fatalf("new standards registry: %v", err)
	}

	descriptors := registry.List()
	if len(descriptors) == 0 {
		t.Fatal("standards registry is empty")
	}

	registered := make(map[string]bool, len(descriptors))

	for _, descriptor := range descriptors {
		registered[descriptor.ID] = true

		_, moduleErr := runModuleFor(descriptor.ID)
		if moduleErr != nil {
			t.Errorf("standard %q is registered but has no run module: %v", descriptor.ID, moduleErr)
		}
	}

	for standardID := range runModuleTable {
		if !registered[standardID] {
			t.Errorf("run module %q is wired for a standard the registry does not offer", standardID)
		}
	}
}

func TestRunModuleForRejectsAnUnknownStandard(t *testing.T) {
	t.Parallel()

	_, err := runModuleFor("not-a-standard")
	if err == nil {
		t.Fatal("expected an error for an unregistered standard")
	}
}
