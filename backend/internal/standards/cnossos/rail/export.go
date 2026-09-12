package rail

import (
	"fmt"

	"github.com/aconiq/backend/internal/acoustics"
)

// ExportOutputs describes written files for receiver table and raster output.
type ExportOutputs = acoustics.ExportOutputs

// ExportResultBundle exports Lden/Lnight receiver table and raster outputs.
// The layout is the shared END one; only the raster is named after this
// standard, so two bundles from different modules stay comparable.
func ExportResultBundle(baseDir string, outputs []ReceiverOutput, gridWidth int, gridHeight int) (ExportOutputs, error) {
	bundle, err := acoustics.ExportENDBundle(baseDir, StandardID, outputs, gridWidth, gridHeight)
	if err != nil {
		return ExportOutputs{}, fmt.Errorf("export %s result bundle: %w", StandardID, err)
	}

	return bundle, nil
}
