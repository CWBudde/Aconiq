package standards

import (
	bebexposure "github.com/aconiq/backend/internal/standards/beb/exposure"
	bubindustry "github.com/aconiq/backend/internal/standards/bub/industry"
	bubrail "github.com/aconiq/backend/internal/standards/bub/rail"
	bubroad "github.com/aconiq/backend/internal/standards/bub/road"
	bufaircraft "github.com/aconiq/backend/internal/standards/buf/aircraft"
	"github.com/aconiq/backend/internal/standards/cnossos/aircraft"
	"github.com/aconiq/backend/internal/standards/cnossos/industry"
	"github.com/aconiq/backend/internal/standards/cnossos/rail"
	cnossosroad "github.com/aconiq/backend/internal/standards/cnossos/road"
	"github.com/aconiq/backend/internal/standards/dummy/freefield"
	"github.com/aconiq/backend/internal/standards/framework"
	"github.com/aconiq/backend/internal/standards/iso9613"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/standards/schall03"
)

// NewRegistry returns the local standards registry used by CLI runs.
func NewRegistry() (framework.Registry, error) {
	return framework.NewRegistry(
		freefield.Descriptor(),
		bebexposure.Descriptor(),
		bubindustry.Descriptor(),
		bubrail.Descriptor(),
		aircraft.Descriptor(),
		bufaircraft.Descriptor(),
		bubroad.Descriptor(),
		cnossosroad.Descriptor(),
		rail.Descriptor(),
		industry.Descriptor(),
		iso9613.Descriptor(),
		rls19road.Descriptor(),
		schall03.Descriptor(),
	)
}
