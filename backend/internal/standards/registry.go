package standards

import (
	"github.com/aconiq/backend/internal/standards/cnossos/rail"
	cnossosroad "github.com/aconiq/backend/internal/standards/cnossos/road"
	"github.com/aconiq/backend/internal/standards/dummy/freefield"
	"github.com/aconiq/backend/internal/standards/framework"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
)

// NewRegistry returns the local standards registry used by CLI runs.
func NewRegistry() (framework.Registry, error) {
	return framework.NewRegistry(
		freefield.Descriptor(),
		cnossosroad.Descriptor(),
		rail.Descriptor(),
		rls19road.Descriptor(),
	)
}
