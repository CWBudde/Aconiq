package soundplanimport

import (
	"fmt"
	"os"
	"path/filepath"
)

// ProjectBundle collects the currently supported SoundPlan project inputs.
// It is a staging structure for future `noise import --from-soundplan` work.
type ProjectBundle struct {
	Project        *Project
	Runs           []*RunResult
	Standards      []StandardMapping
	GridMaps       []GridMapMetadata
	RailOps        []RailOperationSummary
	RailTracks     []RailTrack
	GeoObjects     *GeoObjects
	Barriers       []NoiseBarrier
	Terrain        *TerrainData
	CalcArea       *CalcArea
	TrainTypes     []TrainType
	Warnings       []string
	ProjectDir     string
	ResultFileRefs []string
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

	for _, run := range runs {
		for _, ref := range run.GeoFiles {
			bundle.ResultFileRefs = append(bundle.ResultFileRefs, ref.Name)
		}
	}

	for _, mapping := range bundle.Standards {
		if !mapping.Supported && mapping.Warning != "" {
			bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("standard %d: %s", mapping.SoundPlanID, mapping.Warning))
		}
	}

	bundle.GridMaps = LoadGridMapMetadata(projectDir, runs)

	loadOptional := func(path string, fn func(string) error) {
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

	loadOptional(filepath.Join(projectDir, "GeoRail.geo"), func(path string) error {
		tracks, parseErr := ParseGeoRailFile(path)
		if parseErr == nil {
			bundle.RailTracks = tracks
		}

		return parseErr
	})

	loadOptional(filepath.Join(projectDir, "GeoObjs.geo"), func(path string) error {
		objs, parseErr := ParseGeoObjsFile(path)
		if parseErr == nil {
			bundle.GeoObjects = objs
		}

		return parseErr
	})

	loadOptional(filepath.Join(projectDir, "GeoWand.geo"), func(path string) error {
		barriers, parseErr := ParseGeoWandFile(path)
		if parseErr == nil {
			bundle.Barriers = barriers
		}

		return parseErr
	})

	loadOptional(filepath.Join(projectDir, "CalcArea.geo"), func(path string) error {
		area, parseErr := ParseCalcAreaFile(path)
		if parseErr == nil {
			bundle.CalcArea = area
		}

		return parseErr
	})

	if terrain, terrainErr := LoadTerrainData(projectDir); terrainErr == nil {
		bundle.Terrain = terrain
	} else {
		bundle.Warnings = append(bundle.Warnings, terrainErr.Error())
	}

	loadOptional(filepath.Join(projectDir, "TS03.abs"), func(path string) error {
		types, parseErr := ParseTrainTypes(path)
		if parseErr == nil {
			bundle.TrainTypes = types
		}

		return parseErr
	})

	railOps, railOpsResultDir, railOpsErr := LoadRailOperationSummaries(projectDir, proj, runs)
	if railOpsErr == nil {
		bundle.RailOps = railOps
		bundle.ResultFileRefs = append(bundle.ResultFileRefs, filepath.Base(railOpsResultDir))
	} else {
		bundle.Warnings = append(bundle.Warnings, railOpsErr.Error())
	}

	return bundle, nil
}
