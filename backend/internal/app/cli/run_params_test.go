package cli

import (
	"slices"
	"testing"

	"github.com/aconiq/backend/internal/standards"
	"github.com/aconiq/backend/internal/standards/framework"
)

// A parameter name used to be written three times over — once in the standards
// module's published schema, once in the CLI parse, once in the module's
// provenance key list — with nothing tying the three together. The binding
// tables remove the repetition; this test is what makes the single-source claim
// enforceable rather than aspirational.
//
// It runs both directions. A parameter the schema declares but the CLI never
// binds is silently ignored at run time: the operator passes --param and the run
// keeps its default. A parameter the CLI binds but the schema does not declare
// can never be present in the normalized map, so the run fails with an internal
// error on a name the user cannot even pass.
func TestRunOptionsCoverParameterSchema(t *testing.T) {
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

			boundKeys, ok := runOptionParamKeys(descriptor.ID)
			if !ok {
				t.Fatalf("standard %q is registered but binds no run options; add a binding table rather than skipping it", descriptor.ID)
			}

			bound := make(map[string]struct{}, len(boundKeys))
			for _, key := range boundKeys {
				if _, duplicate := bound[key]; duplicate {
					t.Errorf("parameter %q is bound twice", key)
				}

				bound[key] = struct{}{}
			}

			declared := declaredParameterNames(descriptor)

			for _, name := range declared {
				if _, found := bound[name]; !found {
					t.Errorf("parameter %q is declared by the schema but bound by no run option; passing --param %s=... would be silently ignored", name, name)
				}
			}

			for _, key := range boundKeys {
				if !slices.Contains(declared, key) {
					t.Errorf("parameter %q is bound by a run option but declared by no profile schema; it can never reach the parser", key)
				}
			}
		})
	}
}

// declaredParameterNames returns every parameter name any version/profile of one
// standard publishes. The union is the right comparison set: the CLI parses one
// options struct per standard, not one per profile, so a parameter that only one
// profile declares still has to be bound.
func declaredParameterNames(descriptor framework.StandardDescriptor) []string {
	var names []string

	for _, version := range descriptor.Versions {
		for _, profile := range version.Profiles {
			for _, parameter := range profile.ParameterSchema.Parameters {
				if !slices.Contains(names, parameter.Name) {
					names = append(names, parameter.Name)
				}
			}
		}
	}

	return names
}
