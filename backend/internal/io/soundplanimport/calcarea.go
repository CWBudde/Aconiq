package soundplanimport

import (
	"errors"
	"fmt"
	"os"
)

// CalcArea represents the calculation area polygon from CalcArea.geo.
// Typically a closed rectangle defining the noise map extent.
type CalcArea struct {
	Points []Point3D // closed polygon (first == last)
}

// ParseCalcAreaFile reads a SoundPlan CalcArea.geo binary file and extracts
// the calculation area polygon.
func ParseCalcAreaFile(path string) (*CalcArea, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("soundplan: read calcarea: %w", err)
	}

	return parseCalcAreaData(data)
}

func parseCalcAreaData(data []byte) (*CalcArea, error) {
	var pts []Point3D

	for i := 0; i <= len(data)-38; i++ {
		if data[i] != ':' || data[i+1] != 'G' || data[i+2] != ' ' {
			continue
		}

		off := i + 6

		pts = append(pts, Point3D{
			X: readF64(data, off),
			Y: readF64(data, off+8),
			Z: readF64(data, off+16),
		})

		i = off + 32 - 1 // advance past record (loop increments i)
	}

	if len(pts) == 0 {
		return nil, errors.New("soundplan: calcarea: no points found")
	}

	return &CalcArea{Points: pts}, nil
}
