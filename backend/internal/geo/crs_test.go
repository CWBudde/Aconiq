package geo

import "testing"

func TestParseCRS(t *testing.T) {
	t.Parallel()

	crs, err := ParseCRS("EPSG:4326")
	if err != nil {
		t.Fatalf("parse CRS: %v", err)
	}

	if crs.Kind != CRSKindGeographic {
		t.Fatalf("expected geographic kind, got %s", crs.Kind)
	}

	projected, err := ParseCRS("epsg:25832")
	if err != nil {
		t.Fatalf("parse projected CRS: %v", err)
	}

	if projected.Kind != CRSKindProjected {
		t.Fatalf("expected projected kind, got %s", projected.Kind)
	}
}

func TestBuildTransformPipelineIdentity(t *testing.T) {
	t.Parallel()

	crs, _ := ParseCRS("EPSG:25832")

	pipeline, err := BuildTransformPipeline(crs, crs)
	if err != nil {
		t.Fatalf("build pipeline: %v", err)
	}

	out, err := pipeline.ApplyPoint(Point2D{X: 10, Y: 20})
	if err != nil {
		t.Fatalf("apply point: %v", err)
	}

	if out != (Point2D{X: 10, Y: 20}) {
		t.Fatalf("unexpected transform result: %#v", out)
	}
}

func TestBuildTransformPipelineUnsupported(t *testing.T) {
	t.Parallel()

	projectCRS, _ := ParseCRS("EPSG:25832")

	importCRS, _ := ParseCRS("EPSG:4326")
	if _, err := BuildTransformPipeline(projectCRS, importCRS); err == nil {
		t.Fatal("expected unsupported transform error")
	}
}
