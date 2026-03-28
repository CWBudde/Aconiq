package soundplanimport

import (
	"testing"

	"github.com/aconiq/backend/internal/standards/iso9613"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/standards/schall03"
)

func TestMapStandardID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id      int
		wantID  string
		wantOK  bool
		version string
		profile string
	}{
		{20490, schall03.StandardID, true, "phase18-baseline-preview", "rail-planning-preview"},
		{10490, rls19road.StandardID, true, "2019", "default"},
		{30000, iso9613.StandardID, true, "1996-octaveband", "point-source"},
		{99999, "", false, "", ""},
	}

	for _, tt := range tests {
		got := MapStandardID(tt.id)
		if got.Supported != tt.wantOK {
			t.Fatalf("MapStandardID(%d).Supported = %v, want %v", tt.id, got.Supported, tt.wantOK)
		}

		if got.Aconiq.ID != tt.wantID {
			t.Fatalf("MapStandardID(%d).Aconiq.ID = %q, want %q", tt.id, got.Aconiq.ID, tt.wantID)
		}

		if got.Aconiq.Version != tt.version {
			t.Fatalf("MapStandardID(%d).Aconiq.Version = %q, want %q", tt.id, got.Aconiq.Version, tt.version)
		}

		if got.Aconiq.Profile != tt.profile {
			t.Fatalf("MapStandardID(%d).Aconiq.Profile = %q, want %q", tt.id, got.Aconiq.Profile, tt.profile)
		}
	}
}

func TestMapEnabledStandards(t *testing.T) {
	t.Parallel()

	proj := &Project{
		EnabledStandards: map[int]bool{
			20490: true,
			10490: true,
			30000: true,
			12345: true,
			10440: false,
		},
	}

	got := MapEnabledStandards(proj)
	if len(got) != 4 {
		t.Fatalf("len(MapEnabledStandards) = %d, want 4", len(got))
	}

	if got[0].SoundPlanID != 10490 || got[1].SoundPlanID != 12345 || got[2].SoundPlanID != 20490 || got[3].SoundPlanID != 30000 {
		t.Fatalf("unexpected mapping order: %#v", got)
	}

	if got[1].Supported {
		t.Fatal("unsupported standard unexpectedly marked supported")
	}
}
