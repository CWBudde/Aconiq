package soundplanimport

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
		return nil, errors.New("soundplan: hoehen.txt: no elevation points found")
	}

	return points, nil
}

// LoadTerrainData loads terrain geometry from GeoTmp.geo when available and
// falls back to Höhen.txt elevation points when binary terrain data is absent.
func LoadTerrainData(projectDir string) (*TerrainData, error) {
	terrain := &TerrainData{}
	loadErrors := make([]string, 0, 3)
	baseTerrainLoaded := false

	geoTmpPath := filepath.Join(projectDir, "GeoTmp.geo")
	geoTmpTerrain, geoErr := ParseGeoTmpFile(geoTmpPath)

	switch {
	case geoErr == nil && geoTmpTerrain != nil && (len(geoTmpTerrain.ElevationPoints) > 0 || len(geoTmpTerrain.ContourLines) > 0):
		terrain.ElevationPoints = geoTmpTerrain.ElevationPoints
		terrain.ContourLines = geoTmpTerrain.ContourLines
		baseTerrainLoaded = true
	case geoErr != nil:
		loadErrors = append(loadErrors, fmt.Sprintf("geotmp=%v", geoErr))
	default:
		loadErrors = append(loadErrors, "geotmp=no terrain features found")
	}

	if !baseTerrainLoaded {
		hoehenPath := filepath.Join(projectDir, "Höhen.txt")
		points, txtErr := ParseHoehenTxtFile(hoehenPath)
		if txtErr == nil {
			terrain.ElevationPoints = points
			baseTerrainLoaded = true
		} else {
			loadErrors = append(loadErrors, fmt.Sprintf("hoehen=%v", txtErr))
		}
	}

	dgmFiles, dgmWarnings, dgmLoadErrors := loadDGMFiles(projectDir, baseTerrainLoaded)
	terrain.DGMFiles = append(terrain.DGMFiles, dgmFiles...)
	terrain.Warnings = append(terrain.Warnings, dgmWarnings...)
	loadErrors = append(loadErrors, dgmLoadErrors...)

	if baseTerrainLoaded || len(terrain.DGMFiles) > 0 {
		return terrain, nil
	}

	return nil, fmt.Errorf("soundplan: load terrain data: %s", strings.Join(loadErrors, "; "))
}

// loadDGMFiles globs and parses *.dgm files in projectDir. Parse failures
// become warnings once some terrain data (base terrain or a prior DGM file)
// has already loaded successfully; otherwise they become fatal load errors.
func loadDGMFiles(projectDir string, baseTerrainLoaded bool) (dgmFiles []DGMData, warnings []string, loadErrors []string) {
	dgmPaths, globErr := filepath.Glob(filepath.Join(projectDir, "*.dgm"))
	if globErr != nil {
		loadErrors = append(loadErrors, fmt.Sprintf("dgm=%v", globErr))
		return dgmFiles, warnings, loadErrors
	}

	sort.Strings(dgmPaths)
	for _, path := range dgmPaths {
		dgm, dgmErr := ParseDGMFile(path)
		if dgmErr == nil {
			dgmFiles = append(dgmFiles, *dgm)
			continue
		}

		msg := fmt.Sprintf("%s: %v", filepath.Base(path), dgmErr)
		if baseTerrainLoaded || len(dgmFiles) > 0 {
			warnings = append(warnings, msg)
		} else {
			loadErrors = append(loadErrors, "dgm="+msg)
		}
	}

	return dgmFiles, warnings, loadErrors
}
