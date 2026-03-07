package framework

import (
	"fmt"
	"sort"
	"strings"
)

// Registry stores standard descriptors by standard ID.
type Registry struct {
	descriptors map[string]StandardDescriptor
}

// NewRegistry validates and registers all standard descriptors.
func NewRegistry(descriptors ...StandardDescriptor) (Registry, error) {
	registered := make(map[string]StandardDescriptor, len(descriptors))
	for _, descriptor := range descriptors {
		if err := descriptor.Validate(); err != nil {
			return Registry{}, err
		}
		id := strings.TrimSpace(descriptor.ID)
		if _, exists := registered[id]; exists {
			return Registry{}, fmt.Errorf("standard %q registered more than once", id)
		}
		registered[id] = descriptor
	}

	return Registry{descriptors: registered}, nil
}

// List returns all registered standard descriptors in deterministic (sorted) order.
func (r Registry) List() []StandardDescriptor {
	ids := make([]string, 0, len(r.descriptors))
	for id := range r.descriptors {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	result := make([]StandardDescriptor, 0, len(ids))
	for _, id := range ids {
		result = append(result, r.descriptors[id])
	}
	return result
}

// Resolve resolves standard ID + optional version/profile to one concrete profile.
func (r Registry) Resolve(standardID string, version string, profile string) (ResolvedProfile, error) {
	standardID = strings.TrimSpace(standardID)
	if standardID == "" {
		return ResolvedProfile{}, fmt.Errorf("standard id is required")
	}

	descriptor, ok := r.descriptors[standardID]
	if !ok {
		return ResolvedProfile{}, fmt.Errorf("unknown standard %q", standardID)
	}

	return descriptor.ResolveVersionProfile(version, profile)
}
