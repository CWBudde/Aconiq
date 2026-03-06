package acceptance

import "testing"

func TestRegistryIncludesAcceptanceHooks(t *testing.T) {
	t.Parallel()

	hooks := NewRegistry().List()
	if len(hooks) == 0 {
		t.Fatal("expected at least one hook")
	}

	seen := make(map[string]bool, len(hooks))

	for _, hook := range hooks {
		if hook.Name == "" || hook.StandardID == "" || hook.Description == "" {
			t.Fatalf("unexpected incomplete hook: %#v", hook)
		}

		seen[hook.StandardID] = true
	}

	for _, standardID := range []string{
		"cnossos-road",
		"cnossos-rail",
		"cnossos-industry",
		"cnossos-aircraft",
		"bub-road",
		"buf-aircraft",
		"beb-exposure",
		"rls19-road",
	} {
		if !seen[standardID] {
			t.Fatalf("expected acceptance hook for %s", standardID)
		}
	}
}
