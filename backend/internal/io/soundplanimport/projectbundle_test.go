package soundplanimport

import (
	"path/filepath"
	"testing"
)

func TestLoadProjectBundle(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	bundle, err := LoadProjectBundle(dir)
	if err != nil {
		t.Fatalf("LoadProjectBundle: %v", err)
	}

	if bundle.Project == nil {
		t.Fatal("Project is nil")
	}

	if len(bundle.Runs) < 5 {
		t.Fatalf("got %d runs, want at least 5", len(bundle.Runs))
	}

	if len(bundle.RailTracks) == 0 {
		t.Fatal("no rail tracks loaded")
	}

	if bundle.GeoObjects == nil || len(bundle.GeoObjects.Buildings) == 0 {
		t.Fatal("no geometry objects loaded")
	}

	if len(bundle.Barriers) == 0 {
		t.Fatal("no barriers loaded")
	}

	if bundle.Terrain == nil || len(bundle.Terrain.ElevationPoints) == 0 {
		t.Fatal("no terrain loaded")
	}

	if len(bundle.Terrain.DGMFiles) != 1 {
		t.Fatalf("got %d DGM files, want 1", len(bundle.Terrain.DGMFiles))
	}

	if bundle.CalcArea == nil || len(bundle.CalcArea.Points) == 0 {
		t.Fatal("no calc area loaded")
	}

	if len(bundle.TrainTypes) == 0 {
		t.Fatal("no train types loaded")
	}

	if len(bundle.GridMaps) != 4 {
		t.Fatalf("grid map count = %d, want 4", len(bundle.GridMaps))
	}

	if len(bundle.ImmissionTables) != 1 {
		t.Fatalf("immission table count = %d, want 1", len(bundle.ImmissionTables))
	}

	foundProjectSP := false
	foundGeoRailRef := false
	for _, ref := range bundle.ResultFileRefs {
		if ref == filepath.Base(filepath.Join(dir, "GeoRail.geo")) {
			foundGeoRailRef = true
		}
	}

	for _, mapping := range bundle.Standards {
		if mapping.SoundPlanID == 20490 && mapping.Supported {
			foundProjectSP = true
			break
		}
	}

	if !foundProjectSP {
		t.Fatal("expected Schall 03 standard mapping in bundle")
	}

	if !foundGeoRailRef {
		t.Fatal("expected GeoRail.geo reference in discovered .res metadata")
	}
}
