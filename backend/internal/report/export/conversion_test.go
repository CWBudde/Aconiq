package export

import (
	"math"
	"strings"
	"testing"
)

func TestValidateGeoKeyEPSGRefusesWhatAGeoKeyCannotHold(t *testing.T) {
	t.Parallel()

	// 102100 and 900913 are the codes users actually paste: the first is the
	// ESRI spelling of Web Mercator, the second its legacy Google name. Both
	// pass classifyEPSG as projected, so a run accepts them today.
	for _, epsgCode := range []int{math.MaxUint16 + 1, 102100, 102008, 900913} {
		err := validateGeoKeyEPSG(epsgCode)
		if err == nil {
			t.Fatalf("validateGeoKeyEPSG(%d) accepted a code no GeoKey can hold", epsgCode)
		}

		if !strings.Contains(err.Error(), "GeoTIFF") {
			t.Fatalf("validateGeoKeyEPSG(%d) error does not name the format that imposes the limit: %v", epsgCode, err)
		}
	}
}

func TestValidateGeoKeyEPSGAcceptsEveryTransformableCode(t *testing.T) {
	t.Parallel()

	// Every code the project can build a transform for, plus the boundary and
	// the "no CRS" case buildGeoKeys handles by omitting the keys entirely.
	for _, epsgCode := range []int{0, -1, 4326, 4258, 25832, 25833, 31467, 32633, 3857, math.MaxUint16} {
		err := validateGeoKeyEPSG(epsgCode)
		if err != nil {
			t.Fatalf("validateGeoKeyEPSG(%d) refused a representable code: %v", epsgCode, err)
		}
	}
}

func TestValidateSRSIDAcceptsTheGeoPackageSentinels(t *testing.T) {
	t.Parallel()

	// initGeoPackage writes rows for -1 ("Undefined cartesian SRS") and 0
	// ("Undefined geographic SRS") itself, so refusing them here would
	// contradict the writer sitting beside it.
	for _, srsID := range []int{-1, 0, 4326, 25832, math.MinInt32, math.MaxInt32} {
		err := validateSRSID(srsID)
		if err != nil {
			t.Fatalf("validateSRSID(%d) refused a legal srs_id: %v", srsID, err)
		}
	}
}

func TestValidateSRSIDRefusesWhatInt32CannotHold(t *testing.T) {
	t.Parallel()

	for _, srsID := range []int{math.MaxInt32 + 1, math.MinInt32 - 1} {
		err := validateSRSID(srsID)
		if err == nil {
			t.Fatalf("validateSRSID(%d) accepted an srs_id outside int32", srsID)
		}
	}
}

func TestSRSIDBitsEncodesTheNegativeSentinelsAsTwosComplement(t *testing.T) {
	t.Parallel()

	// The GeoPackageBinaryHeader srs_id field is int32, so -1 is 0xFFFFFFFF
	// rather than an error. This is the conversion that used to go through
	// mustUint32 and panic.
	cases := map[int]uint32{
		-1:    0xFFFFFFFF,
		0:     0,
		4326:  4326,
		25832: 25832,
	}

	for srsID, want := range cases {
		if got := srsIDBits(srsID); got != want {
			t.Fatalf("srsIDBits(%d) = %#x, want %#x", srsID, got, want)
		}
	}
}

func TestMustUint32RejectsBothEnds(t *testing.T) {
	t.Parallel()

	// mustUint32 carries a //nolint:gosec that calls it "bounds-checked". It
	// only checked the lower bound, so a value above uint32 truncated in
	// silence - the failure mode a must* helper exists to prevent.
	for _, value := range []int{-1, math.MaxUint32 + 1} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("mustUint32(%d) returned instead of panicking", value)
				}
			}()

			_ = mustUint32(value)
		}()
	}

	if got := mustUint32(math.MaxUint32); got != math.MaxUint32 {
		t.Fatalf("mustUint32(MaxUint32) = %d, want %d", got, uint32(math.MaxUint32))
	}
}
