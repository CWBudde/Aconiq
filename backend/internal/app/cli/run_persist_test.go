package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	bebexposure "github.com/aconiq/backend/internal/standards/beb/exposure"
	bubroad "github.com/aconiq/backend/internal/standards/bub/road"
	bufaircraft "github.com/aconiq/backend/internal/standards/buf/aircraft"
	cnossosaircraft "github.com/aconiq/backend/internal/standards/cnossos/aircraft"
	cnossosindustry "github.com/aconiq/backend/internal/standards/cnossos/industry"
	cnossosrail "github.com/aconiq/backend/internal/standards/cnossos/rail"
	cnossosroad "github.com/aconiq/backend/internal/standards/cnossos/road"
	"github.com/aconiq/backend/internal/standards/framework"
	"github.com/aconiq/backend/internal/standards/iso9613"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/standards/schall03"
)

// Every standard-backed run summary must name the version of the model that
// produced it, under one key. Before this was unified the schema differed per
// standard: three summaries carried no version at all and RLS-19 spelled the
// key `data_pack_version`.
func TestPersistRunOutputsWriteUnifiedModelVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		want     string
		wantTier framework.EvidenceTier
		persist  func(runDir string) (persistedRunOutputs, string, error)
	}{
		{
			name:     "cnossos-road",
			wantTier: framework.EvidenceTierScaffold,
			want:     cnossosroad.BuiltinModelVersion,
			persist: func(runDir string) (persistedRunOutputs, string, error) {
				out, hash, _, err := persistCnossosRoadRunOutputs(runDir, nil, 0, 0, 0, receiverModeCustom, framework.EvidenceTierScaffold)

				return out, hash, err
			},
		},
		{
			name:     "cnossos-rail",
			wantTier: framework.EvidenceTierScaffold,
			want:     cnossosrail.BuiltinModelVersion,
			persist: func(runDir string) (persistedRunOutputs, string, error) {
				out, hash, _, err := persistCnossosRailRunOutputs(runDir, nil, 0, 0, 0, receiverModeCustom, framework.EvidenceTierScaffold)

				return out, hash, err
			},
		},
		{
			name:     "cnossos-aircraft",
			wantTier: framework.EvidenceTierScaffold,
			want:     cnossosaircraft.BuiltinModelVersion,
			persist: func(runDir string) (persistedRunOutputs, string, error) {
				out, hash, _, err := persistCnossosAircraftRunOutputs(runDir, nil, 0, 0, 0, receiverModeCustom, framework.EvidenceTierScaffold)

				return out, hash, err
			},
		},
		{
			name:     "cnossos-industry",
			wantTier: framework.EvidenceTierScaffold,
			want:     cnossosindustry.BuiltinModelVersion,
			persist: func(runDir string) (persistedRunOutputs, string, error) {
				out, hash, _, err := persistCnossosIndustryRunOutputs(runDir, nil, 0, 0, 0, receiverModeCustom, framework.EvidenceTierScaffold)

				return out, hash, err
			},
		},
		// bub-rail and bub-industry version nothing of their own: they alias the
		// CNOSSOS scaffolds and delegate every number to them, so the CNOSSOS
		// model version is the one their summaries must name.
		{
			name:     "bub-rail",
			wantTier: framework.EvidenceTierScaffold,
			want:     cnossosrail.BuiltinModelVersion,
			persist: func(runDir string) (persistedRunOutputs, string, error) {
				out, hash, _, err := persistBUBRailRunOutputs(runDir, nil, 0, 0, 0, receiverModeCustom, framework.EvidenceTierScaffold)

				return out, hash, err
			},
		},
		{
			name:     "bub-industry",
			wantTier: framework.EvidenceTierScaffold,
			want:     cnossosindustry.BuiltinModelVersion,
			persist: func(runDir string) (persistedRunOutputs, string, error) {
				out, hash, _, err := persistBUBIndustryRunOutputs(runDir, nil, 0, 0, 0, receiverModeCustom, framework.EvidenceTierScaffold)

				return out, hash, err
			},
		},
		{
			name:     "bub-road",
			wantTier: framework.EvidenceTierScaffold,
			want:     bubroad.BuiltinModelVersion,
			persist: func(runDir string) (persistedRunOutputs, string, error) {
				out, hash, _, err := persistBUBRoadRunOutputs(runDir, nil, 0, 0, 0, receiverModeCustom, framework.EvidenceTierScaffold)

				return out, hash, err
			},
		},
		{
			name:     "buf-aircraft",
			wantTier: framework.EvidenceTierScaffold,
			want:     bufaircraft.BuiltinModelVersion,
			persist: func(runDir string) (persistedRunOutputs, string, error) {
				out, hash, _, err := persistBUFAircraftRunOutputs(runDir, nil, 0, 0, 0, receiverModeCustom, framework.EvidenceTierScaffold)

				return out, hash, err
			},
		},
		{
			name:     "iso9613",
			wantTier: framework.EvidenceTierNormative,
			want:     iso9613.BuiltinModelVersion,
			persist: func(runDir string) (persistedRunOutputs, string, error) {
				out, hash, _, err := persistISO9613RunOutputs(runDir, nil, 0, 0, 0, receiverModeCustom, framework.EvidenceTierNormative)

				return out, hash, err
			},
		},
		{
			name:     "schall03-normative",
			wantTier: framework.EvidenceTierNormative,
			want:     schall03.NormativeModelVersion,
			persist: func(runDir string) (persistedRunOutputs, string, error) {
				out, hash, _, err := persistSchall03RunOutputs(runDir, nil, 0, 0, 0, receiverModeCustom, schall03.EngineNormative, framework.EvidenceTierNormative)

				return out, hash, err
			},
		},
		{
			name:     "schall03-preview",
			wantTier: framework.EvidenceTierNormative,
			want:     schall03.PreviewModelVersion,
			persist: func(runDir string) (persistedRunOutputs, string, error) {
				out, hash, _, err := persistSchall03RunOutputs(runDir, nil, 0, 0, 0, receiverModeCustom, schall03.EnginePreview, framework.EvidenceTierNormative)

				return out, hash, err
			},
		},
		{
			// RLS-19 carries no model version of its own; its data pack is what
			// versions the result. The key must still be the unified one.
			name:     "rls19-road",
			wantTier: framework.EvidenceTierNormative,
			want:     rls19road.BuiltinDataPackVersion,
			persist: func(runDir string) (persistedRunOutputs, string, error) {
				out, hash, _, err := persistRLS19RoadRunOutputs(runDir, nil, 0, 0, 0, 0, receiverModeCustom, framework.EvidenceTierNormative)

				return out, hash, err
			},
		},
		{
			name:     "beb-exposure",
			wantTier: framework.EvidenceTierPreview,
			want:     bebexposure.BuiltinModelVersion,
			persist: func(runDir string) (persistedRunOutputs, string, error) {
				outputs := []bebexposure.BuildingExposureOutput{{
					Building:               bebexposure.BuildingUnit{ID: "b1", UsageType: "residential", HeightM: 10},
					RepresentativeReceiver: geo.PointReceiver{ID: "b1-r", HeightM: 4},
				}}

				out, hash, _, err := persistBEBExposureRunOutputs(runDir, outputs, bebexposure.Summary{BuildingCount: 1}, 0, framework.EvidenceTierPreview)

				return out, hash, err
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			runDir := t.TempDir()

			persisted, _, err := testCase.persist(runDir)
			if err != nil {
				t.Fatalf("persist run outputs: %v", err)
			}

			summary := readRunSummary(t, persisted.SummaryPath)

			got, ok := summary["model_version"]
			if !ok {
				t.Fatalf("run summary carries no model_version: %v", summary)
			}

			if got != testCase.want {
				t.Fatalf("model_version = %v, want %q", got, testCase.want)
			}

			// A summary that names a model version but not an evidence tier
			// would let the reader assume the numbers are as trustworthy as the
			// standard the module is named after.
			if summary["evidence_tier"] != string(testCase.wantTier) {
				t.Fatalf("evidence_tier = %v, want %q", summary["evidence_tier"], testCase.wantTier)
			}

			if _, stale := summary["data_pack_version"]; stale && testCase.name == "rls19-road" {
				t.Fatal("rls19-road still writes the old data_pack_version key")
			}
		})
	}
}

// The cnossos-road summary used to be stamped with the cnossos-industry model
// version and reporting precision.
func TestPersistCnossosRoadRunOutputsUsesRoadConstants(t *testing.T) {
	t.Parallel()

	runDir := t.TempDir()

	persisted, _, _, err := persistCnossosRoadRunOutputs(runDir, nil, 0, 0, 0, receiverModeCustom, framework.EvidenceTierScaffold)
	if err != nil {
		t.Fatalf("persist cnossos road outputs: %v", err)
	}

	summary := readRunSummary(t, persisted.SummaryPath)

	if summary["model_version"] == cnossosindustry.BuiltinModelVersion {
		t.Fatalf("cnossos-road summary still reports the industry model version: %v", summary["model_version"])
	}

	if summary["model_version"] != cnossosroad.BuiltinModelVersion {
		t.Fatalf("model_version = %v, want %q", summary["model_version"], cnossosroad.BuiltinModelVersion)
	}

	// JSON numbers decode as float64.
	if summary["reporting_precision_db"] != float64(cnossosroad.ReportingPrecisionDB) {
		t.Fatalf("reporting_precision_db = %v, want %v", summary["reporting_precision_db"], cnossosroad.ReportingPrecisionDB)
	}
}

func readRunSummary(t *testing.T, path string) map[string]any {
	t.Helper()

	if path == "" {
		t.Fatal("no run summary was written")
	}

	payload, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read run summary: %v", err)
	}

	var summary map[string]any

	err = json.Unmarshal(payload, &summary)
	if err != nil {
		t.Fatalf("decode run summary: %v", err)
	}

	return summary
}

// The Schall 03 run summary must describe the chain that actually ran: the
// normative chain reads Beiblatt 1/2 and never touches a data pack, so
// stamping a data-pack version there would assert a coefficient source the run
// did not use.
func TestPersistSchall03RunOutputsStampsOnlyTheEngineThatRan(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		engine                 string
		wantModelVersion       string
		wantComplianceBoundary string
		wantDataPackVersionKey bool
	}{
		{schall03.EngineNormative, schall03.NormativeModelVersion, schall03.ComplianceBoundaryNormative, false},
		{schall03.EnginePreview, schall03.PreviewModelVersion, schall03.ComplianceBoundaryPreview, true},
	} {
		t.Run(testCase.engine, func(t *testing.T) {
			t.Parallel()

			persisted, _, _, err := persistSchall03RunOutputs(t.TempDir(), nil, 0, 0, 0, receiverModeCustom, testCase.engine, framework.EvidenceTierNormative)
			if err != nil {
				t.Fatalf("persist schall03 outputs: %v", err)
			}

			summary := readRunSummary(t, persisted.SummaryPath)

			if summary["model_version"] != testCase.wantModelVersion {
				t.Fatalf("model_version = %v, want %q", summary["model_version"], testCase.wantModelVersion)
			}

			if summary["compliance_boundary"] != testCase.wantComplianceBoundary {
				t.Fatalf("compliance_boundary = %v, want %q", summary["compliance_boundary"], testCase.wantComplianceBoundary)
			}

			if summary["schall03_engine"] != testCase.engine {
				t.Fatalf("schall03_engine = %v, want %q", summary["schall03_engine"], testCase.engine)
			}

			_, hasDataPack := summary["data_pack_version"]
			if hasDataPack != testCase.wantDataPackVersionKey {
				t.Fatalf("data_pack_version present = %v, want %v: %v", hasDataPack, testCase.wantDataPackVersionKey, summary)
			}
		})
	}
}
