package schall03_test

import (
	"testing"

	"github.com/aconiq/backend/internal/standards/schall03"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGleisbremse_DeltaLW_BGBl_p2312 verifies the octave-band DeltaLW spectra
// for three Gleisbremse types against the authoritative BGBl source (p. 2312).
func TestGleisbremse_DeltaLW_BGBl_p2312(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		typ     schall03.GleisbremseType
		wantLWA float64
		wantDLW [8]float64
	}{
		{
			name:    "i=6 TW mit Segmenten",
			typ:     schall03.GleisbremsTalbremsMitSegmenten,
			wantLWA: 98,
			wantDLW: [8]float64{-56, -52, -45, -41, -38, -9, -1, -13},
		},
		{
			name:    "i=8 Gummiwalkbremse",
			typ:     schall03.GleisbremsGummiwalk,
			wantLWA: 83,
			wantDLW: [8]float64{-28, -18, -12, -7, -6, -7, -8, -11},
		},
		{
			name:    "i=10 Schraubenbremse",
			typ:     schall03.GleisbremsSchraubenbremse,
			wantLWA: 72,
			wantDLW: [8]float64{-29, -21, -9, -10, -8, -4, -9, -13},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, ok := schall03.Beiblatt3GleisbremsenByType(tt.typ)
			require.True(t, ok, "Gleisbremse type %d must exist", tt.typ)

			assert.Equal(t, tt.wantLWA, data.LWA, "L_WA mismatch")
			assert.Equal(t, tt.wantDLW, [8]float64(data.DeltaLW), "DeltaLW spectrum mismatch")
		})
	}
}
