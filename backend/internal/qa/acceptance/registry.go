package acceptance

import "sort"

// Hook describes one acceptance-suite integration entry.
type Hook struct {
	Name        string
	StandardID  string
	Description string
}

// Registry stores available acceptance hooks by standard ID.
type Registry struct {
	hooks []Hook
}

// NewRegistry returns the currently wired acceptance hooks.
func NewRegistry() Registry {
	fixtures := Catalog()

	hooks := make([]Hook, 0, len(fixtures))
	for _, fixture := range fixtures {
		hooks = append(hooks, Hook{
			Name:        fixture.Name,
			StandardID:  fixture.StandardID,
			Description: fixture.Description,
		})
	}

	return Registry{
		hooks: hooks,
	}
}

// List returns all hooks in deterministic order.
func (r Registry) List() []Hook {
	out := append([]Hook(nil), r.hooks...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].StandardID == out[j].StandardID {
			return out[i].Name < out[j].Name
		}

		return out[i].StandardID < out[j].StandardID
	})

	return out
}
