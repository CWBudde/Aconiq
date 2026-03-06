package standards

import (
	"github.com/soundplan/soundplan/backend/internal/standards/cnossos/rail"
	"github.com/soundplan/soundplan/backend/internal/standards/cnossos/road"
	"github.com/soundplan/soundplan/backend/internal/standards/dummy/freefield"
	"github.com/soundplan/soundplan/backend/internal/standards/framework"
)

// NewRegistry returns the local standards registry used by CLI runs.
func NewRegistry() (framework.Registry, error) {
	return framework.NewRegistry(
		freefield.Descriptor(),
		road.Descriptor(),
		rail.Descriptor(),
	)
}
