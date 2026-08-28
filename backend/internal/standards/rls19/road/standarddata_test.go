package road

import (
	"testing"

	"github.com/aconiq/backend/internal/standards/framework"
)

// Pinned deliberately: RLS-19 is a fixed published table set, so a change here
// is either a transcription fix that belongs in a commit citing the source, or
// a silent coefficient edit. Do not update these constants to make the test
// pass; find the table that moved and justify it.
const pinnedRLS19DataDigest = "3746dbf29a3285935f440dfebef75a4c86ed0ec5793f3b3fb3637aaa03bf1699"

var pinnedRLS19TableDigests = map[string]string{
	"rls19/ausbreitungskonstanten":          "0bfcc62eb97d35a22947871ef1641b52a1b154f68767729a8aaa2bf2fb0108ff",
	"rls19/data-pack-version":               "f50319bb5de76d90d23c2e93e7291ec42bef217c7f3aa77d25504cd4826173ae",
	"rls19/parkplatz-fahrzeugzuschlaege":    "fac1907825b628c1f60119e2ee8a4bfc97b890ade79e7b6bf32daaff4f99d299",
	"rls19/tabelle-02-verkehrsmengen":       "42e03ed6cc62f8a1d5fe3fcde8fd7264261608ec09d444fc63c973344c5e0447",
	"rls19/tabelle-03-grundemission":        "8ece046316e1b5f753cc9e33ff1c5b41b3856d2cc4f99c94b05c282361832465",
	"rls19/tabelle-04-fahrbahnkorrektur":    "170571c79bcb4780db2c36939d7d6ccb3837da1e22769ce817c5951c2c25c585",
	"rls19/tabelle-05-knotenpunktkorrektur": "b80cca4c4e170c75946b1a686991537f3f161ec408ea7ae516c2eab3a3b1fd16",
	"rls19/tabelle-07-parkbewegungen":       "c8f676166734de23f0bc135fecf99c86633e2d998c1369c18416276e1004c033",
	"rls19/tabelle-08-reflexionsverluste":   "17d70a832e075a546779e2f6c594807068da460d9049fb95226b6f669f01f334",
}

func TestStandardDataDigestIsPinned(t *testing.T) {
	digest, err := StandardData().Digest(StandardID, framework.EvidenceTierNormative)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	if digest.Digest != pinnedRLS19DataDigest {
		t.Errorf("rls19-road standard data digest = %s, pinned %s", digest.Digest, pinnedRLS19DataDigest)
	}

	seen := make(map[string]struct{}, len(digest.Tables))

	for _, table := range digest.Tables {
		seen[table.Name] = struct{}{}

		want, pinned := pinnedRLS19TableDigests[table.Name]
		if !pinned {
			t.Errorf("table %q is not pinned; add it to pinnedRLS19TableDigests", table.Name)

			continue
		}

		if table.Digest != want {
			t.Errorf("table %q digest = %s, pinned %s", table.Name, table.Digest, want)
		}
	}

	for name := range pinnedRLS19TableDigests {
		if _, ok := seen[name]; !ok {
			t.Errorf("pinned table %q is no longer carried by the module", name)
		}
	}
}

// TestStandardDataCoversSwitchHeldTables guards the tables that RLS-19 holds as
// switch statements rather than as data: those are the ones a digest built only
// from declared variables would silently miss.
func TestStandardDataCoversSwitchHeldTables(t *testing.T) {
	if got := reflectionLossTable(); len(got) != 4 || got[1] != 0.5 || got[2] != 3.0 || got[3] != 5.0 {
		t.Fatalf("Tabelle 8 reflection losses not covered: %v", got)
	}

	if got := parkingVehicleSurchargeTable(); len(got) != 3 {
		t.Fatalf("parking vehicle surcharges not covered: %v", got)
	}

	if got := parkingMovementTable(); len(got) != 4 {
		t.Fatalf("Tabelle 7 movement rates not covered: %v", got)
	}
}
