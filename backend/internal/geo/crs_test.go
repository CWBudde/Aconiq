package geo

import (
	"math"
	"testing"
)

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

func TestBuildTransformPipelineSupported(t *testing.T) {
	t.Parallel()

	projectCRS, _ := ParseCRS("EPSG:25832")
	importCRS, _ := ParseCRS("EPSG:4326")

	pipeline, err := BuildTransformPipeline(projectCRS, importCRS)
	if err != nil {
		t.Fatalf("expected supported transform, got error: %v", err)
	}

	if len(pipeline.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(pipeline.Steps))
	}
}

func TestBuildTransformPipelineUnsupported(t *testing.T) {
	t.Parallel()

	projectCRS, _ := ParseCRS("EPSG:25832")
	importCRS, _ := ParseCRS("EPSG:99999")

	_, err := BuildTransformPipeline(projectCRS, importCRS)
	if err == nil {
		t.Fatal("expected unsupported transform error")
	}
}

func TestCRSHelpersAndIdentityTransform(t *testing.T) {
	t.Parallel()

	if _, err := ParseCRS(""); err == nil {
		t.Fatal("expected missing CRS error")
	}

	if _, err := ParseCRS("EPSG:abc"); err == nil {
		t.Fatal("expected invalid EPSG error")
	}

	if _, err := ParseCRS("proj4:+init=epsg:4326"); err == nil {
		t.Fatal("expected unsupported format error")
	}

	if got := classifyEPSG(4258); got != CRSKindGeographic {
		t.Fatalf("expected geographic kind, got %s", got)
	}

	if got := classifyEPSG(1000); got != CRSKindUnknown {
		t.Fatalf("expected unknown kind, got %s", got)
	}

	pipeline, err := BuildTransformPipeline(CRS{}, CRS{ID: "EPSG:4326"})
	if err == nil || pipeline.Steps != nil {
		t.Fatal("expected missing CRS build error")
	}

	identity := IdentityTransform{}
	if identity.Name() != "identity" {
		t.Fatalf("unexpected transform name %q", identity.Name())
	}

	if _, err := identity.TransformPoint(Point2D{X: math.NaN(), Y: 0}); err == nil {
		t.Fatal("expected invalid identity transform point error")
	}

	if _, err := (TransformPipeline{}).ApplyPoint(Point2D{X: math.NaN(), Y: 0}); err == nil {
		t.Fatal("expected invalid pipeline point error")
	}
}
