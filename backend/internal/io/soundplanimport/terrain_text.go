package soundplanimport

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ParseHoehenTxtFile reads Höhen.txt elevation-point exports.
// Each non-empty line is expected to contain at least X;Y;Z using German decimals.
func ParseHoehenTxtFile(path string) ([]ElevationPoint, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("soundplan: read hoehen.txt: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	points := make([]ElevationPoint, 0, 1024)
	lineNo := 0

	for scanner.Scan() {
		lineNo++

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Split(line, ";")
		if len(parts) < 3 {
			return nil, fmt.Errorf("soundplan: hoehen.txt line %d: expected at least 3 semicolon-separated fields", lineNo)
		}

		points = append(points, ElevationPoint{
			X: parseGermanFloat(strings.TrimSpace(parts[0])),
			Y: parseGermanFloat(strings.TrimSpace(parts[1])),
			Z: parseGermanFloat(strings.TrimSpace(parts[2])),
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("soundplan: scan hoehen.txt: %w", err)
	}

	if len(points) == 0 {
		return nil, fmt.Errorf("soundplan: hoehen.txt: no elevation points found")
	}

	return points, nil
}

// LoadTerrainData loads terrain geometry from GeoTmp.geo when available and
// falls back to Höhen.txt elevation points when binary terrain data is absent.
func LoadTerrainData(projectDir string) (*TerrainData, error) {
	geoTmpPath := filepath.Join(projectDir, "GeoTmp.geo")
	terrain, geoErr := ParseGeoTmpFile(geoTmpPath)
	if geoErr == nil && terrain != nil && (len(terrain.ElevationPoints) > 0 || len(terrain.ContourLines) > 0) {
		return terrain, nil
	}

	hoehenPath := filepath.Join(projectDir, "Höhen.txt")
	points, txtErr := ParseHoehenTxtFile(hoehenPath)
	if txtErr == nil {
		return &TerrainData{ElevationPoints: points}, nil
	}

	if geoErr == nil {
		return nil, txtErr
	}

	if txtErr == nil {
		return &TerrainData{ElevationPoints: points}, nil
	}

	return nil, fmt.Errorf("soundplan: load terrain data: geotmp=%v; hoehen=%v", geoErr, txtErr)
}
