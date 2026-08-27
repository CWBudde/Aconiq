package soundplanimport

import (
	"fmt"
	"os"
	"path/filepath"
)

// ProjectBundle collects the currently supported SoundPlan project inputs.
// It is a staging structure for future `aconiq import --from-soundplan` work.
type ProjectBundle struct {
	Project         *Project
	Runs            []*RunResult
	Standards       []StandardMapping
	GridMaps        []GridMapMetadata
	ImmissionTables []ImmissionTable
	RailOps         []RailOperationSummary
	RailTracks      []RailTrack
	GeoObjects      *GeoObjects
	Barriers        []NoiseBarrier
	Terrain         *TerrainData
	CalcArea        *CalcArea
	TrainTypes      []TrainType
	Warnings        []string
	ProjectDir      string
	ResultFileRefs  []string
}

// LoadProjectBundle parses the supported SoundPlan project inputs found in one directory.
func LoadProjectBundle(projectDir string) (*ProjectBundle, error) {
	projectPath := filepath.Join(projectDir, "Project.sp")

	proj, err := ParseProjectFile(projectPath)
	if err != nil {
		return nil, err
	}

	runs, err := ListRuns(projectDir)
	if err != nil {
		return nil, err
	}

	bundle := &ProjectBundle{
		Project:    proj,
		Runs:       runs,
		Standards:  MapEnabledStandards(proj),
		ProjectDir: projectDir,
	}

	bundle.collectResultFileRefs(runs)
	bundle.collectStandardWarnings()

	bundle.GridMaps = LoadGridMapMetadata(projectDir, runs)
	bundle.loadImmissionTables(projectDir)
	bundle.loadGeoFiles(projectDir)
	bundle.loadTerrain(projectDir)
	bundle.loadTrainTypes(projectDir)
	bundle.loadRailOps(projectDir, proj, runs)

	return bundle, nil
}

// collectResultFileRefs records the geo result file references produced by
// each SoundPLAN run.
func (bundle *ProjectBundle) collectResultFileRefs(runs []*RunResult) {
	for _, run := range runs {
		for _, ref := range run.GeoFiles {
			bundle.ResultFileRefs = append(bundle.ResultFileRefs, ref.Name)
		}
	}
}

// collectStandardWarnings appends a warning for every enabled-but-unsupported
// standard mapping.
func (bundle *ProjectBundle) collectStandardWarnings() {
	for _, mapping := range bundle.Standards {
		if !mapping.Supported && mapping.Warning != "" {
			bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("standard %d: %s", mapping.SoundPlanID, mapping.Warning))
		}
	}
}

// loadImmissionTables loads the immission tables for the project, recording
// any partial-load warnings.
func (bundle *ProjectBundle) loadImmissionTables(projectDir string) {
	if tables, tableWarnings := LoadImmissionTables(projectDir); len(tables) > 0 || len(tableWarnings) > 0 {
		bundle.ImmissionTables = tables
		bundle.Warnings = append(bundle.Warnings, tableWarnings...)
	}
}

// loadOptional loads an optional project input file. A missing file is
// silently skipped; a stat error or parse error becomes a bundle warning.
func (bundle *ProjectBundle) loadOptional(path string, fn func(string) error) {
	if _, statErr := os.Stat(path); statErr != nil {
		if os.IsNotExist(statErr) {
			return
		}

		bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("%s: %v", filepath.Base(path), statErr))

		return
	}

	if parseErr := fn(path); parseErr != nil {
		bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("%s: %v", filepath.Base(path), parseErr))
	}
}

// loadGeoFiles loads the optional GeoRail, GeoObjs, GeoWand, and CalcArea
// geometry files.
func (bundle *ProjectBundle) loadGeoFiles(projectDir string) {
	bundle.loadOptional(filepath.Join(projectDir, "GeoRail.geo"), func(path string) error {
		tracks, parseErr := ParseGeoRailFile(path)
		if parseErr == nil {
			bundle.RailTracks = tracks
		}

		return parseErr
	})

	bundle.loadOptional(filepath.Join(projectDir, "GeoObjs.geo"), func(path string) error {
		objs, parseErr := ParseGeoObjsFile(path)
		if parseErr == nil {
			bundle.GeoObjects = objs
		}

		return parseErr
	})

	bundle.loadOptional(filepath.Join(projectDir, "GeoWand.geo"), func(path string) error {
		barriers, parseErr := ParseGeoWandFile(path)
		if parseErr == nil {
			bundle.Barriers = barriers
		}

		return parseErr
	})

	bundle.loadOptional(filepath.Join(projectDir, "CalcArea.geo"), func(path string) error {
		area, parseErr := ParseCalcAreaFile(path)
		if parseErr == nil {
			bundle.CalcArea = area
		}

		return parseErr
	})
}

// loadTerrain loads terrain data, recording either the terrain's own
// warnings or a single warning for a hard terrain-load failure.
func (bundle *ProjectBundle) loadTerrain(projectDir string) {
	if terrain, terrainErr := LoadTerrainData(projectDir); terrainErr == nil {
		bundle.Terrain = terrain
		bundle.Warnings = append(bundle.Warnings, terrain.Warnings...)
	} else {
		bundle.Warnings = append(bundle.Warnings, terrainErr.Error())
	}
}

// loadTrainTypes loads the optional TS03.abs train type table.
func (bundle *ProjectBundle) loadTrainTypes(projectDir string) {
	bundle.loadOptional(filepath.Join(projectDir, "TS03.abs"), func(path string) error {
		types, parseErr := ParseTrainTypes(path)
		if parseErr == nil {
			bundle.TrainTypes = types
		}

		return parseErr
	})
}

// loadRailOps derives rail operation summaries and records the result
// directory as an additional result file reference.
func (bundle *ProjectBundle) loadRailOps(projectDir string, proj *Project, runs []*RunResult) {
	railOps, railOpsResultDir, railOpsErr := LoadRailOperationSummaries(projectDir, proj, runs)
	if railOpsErr == nil {
		bundle.RailOps = railOps
		bundle.ResultFileRefs = append(bundle.ResultFileRefs, filepath.Base(railOpsResultDir))
	} else {
		bundle.Warnings = append(bundle.Warnings, railOpsErr.Error())
	}
}
