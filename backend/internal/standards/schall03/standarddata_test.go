package schall03

import (
	"testing"

	"github.com/aconiq/backend/internal/standards/framework"
)

// The digests below are pinned deliberately. Anlage 2 zu §4 der 16. BImSchV is
// a fixed published table set, so any movement here is either a transcription
// fix — which belongs in a commit that says so and cites the source — or a
// silent coefficient edit, which is exactly what this test exists to catch.
//
// If this test fails, do not update the constant to make it pass. Find which
// table moved from the per-table list, confirm the change against the published
// Anlage 2, and record it in the same commit.
const (
	pinnedSchall03DataDigest = "d7b679d5dac4560fd2acbb9f41714d5f36b9c4fe4cf9fd2a6e7a322fae714e6b"
)

var pinnedSchall03TableDigests = map[string]string{
	"anlage2/beiblatt1-fahrzeugkategorien":              "68c1b8bd61467566f54a829b889cc77d0cccfdc927f7dec56349a61efcda9f9e",
	"anlage2/beiblatt1-kesselwagen":                     "bd3a98692517ffbdc5a043684c60730dc3901058c1b7f34ef454eb7f73819da0",
	"anlage2/beiblatt1-zugarten":                        "27b878b191da1ae726291b6770ac0a12e5c24e92490bf071e7385aa5c3da4f41",
	"anlage2/beiblatt2-fahrzeugkategorien-strassenbahn": "e10364e28f3f30d9a0036a44f05128844b0260d699585c3957247a340f3273b9",
	"anlage2/beiblatt2-zugarten-strassenbahn":           "7f1ca4c5e28a4546c2c21c87e8b7bcc1e958284888b02f6f5d43a64d93bca09d",
	"anlage2/beiblatt3-gleisbremsen":                    "0cfaff3e83be1de720eb11db02639caef8f211bd074cd3b5fadfc55e4d28c079",
	"anlage2/beiblatt3-rangierquellen":                  "9b05bb2c9fd4450befda6a498d1e2c535da5aca85522a4c56df30a1c9e725021",
	"anlage2/oktavband-mittenfrequenzen":                "3edc95b56cc95340d2c49d96dc53dbd8b49ebc7bc64e52c64568dfd1ed4a8cae",
	"anlage2/tabelle-06-geschwindigkeitsfaktor":         "86a1f3e086cda28eea69177dcbc1ae62c4bc25ba998557fababdb1596401cd2c",
	"anlage2/tabelle-07-fahrbahnart":                    "7461bbc26a39a2b75fa90513f59c857dc161644c6d99a6b362d3fcb4e5f18665",
	"anlage2/tabelle-08-fahrbahnzustand":                "c8cc7c5e3a1c10a98838d1799cd3b0710b08bd7d7b61abc3e19369b45282b420",
	"anlage2/tabelle-09-bruecken":                       "fca1c9c421f2464ef765fdd3ee475a02fc2f52c64c4afcd14c0056694012900e",
	"anlage2/tabelle-11-kurven":                         "9de913399239e657e773cc5a045ba157d7f6fbe678ae7e3bd4951a6f35097700",
	"anlage2/tabelle-15-fahrbahnart-strassenbahn":       "cdc7e5ba57d23cdd7dff2a9ee320537873679903fb6e2b28c73801e4add5003f",
	"anlage2/tabelle-16-bruecken-strassenbahn":          "3f1e8f2a36d4c49cf4addc6e4f8b787c03c81d836e1b76fe388e31b6509ac413",
	"anlage2/tabelle-17-luftabsorption":                 "ded5d688c395f234c8695ab6b35196f8b5e0499ff898d59f9ed23a29e375cbf8",
	"preview/datapack":                                  "ba6494a8ac35f7d0d49a0f7d80ec47714dcaaaa8b7e6374ac56f9b19f1470088",
}

func TestStandardDataDigestIsPinned(t *testing.T) {
	digest, err := StandardData().Digest(StandardID, framework.EvidenceTierNormative)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	if digest.Digest != pinnedSchall03DataDigest {
		t.Errorf("schall03 standard data digest = %s, pinned %s", digest.Digest, pinnedSchall03DataDigest)
	}

	seen := make(map[string]struct{}, len(digest.Tables))

	for _, table := range digest.Tables {
		seen[table.Name] = struct{}{}

		want, pinned := pinnedSchall03TableDigests[table.Name]
		if !pinned {
			t.Errorf("table %q is not pinned; add it to pinnedSchall03TableDigests", table.Name)

			continue
		}

		if table.Digest != want {
			t.Errorf("table %q digest = %s, pinned %s", table.Name, table.Digest, want)
		}
	}

	for name := range pinnedSchall03TableDigests {
		if _, ok := seen[name]; !ok {
			t.Errorf("pinned table %q is no longer carried by the module", name)
		}
	}
}

func TestStandardDataCoversTheEmbeddedAnlage2Tables(t *testing.T) {
	data := StandardData()

	if len(data.Tables) < 16 {
		t.Fatalf("expected the Anlage 2 tables to be covered, got %d tables", len(data.Tables))
	}

	// The digest exists to make a coefficient edit visible, so an edit to any
	// covered table must move it.
	before, err := data.Digest(StandardID, framework.EvidenceTierNormative)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	// FzKategorie holds its Teilquellen in a slice, so the surrounding array
	// copy shares their backing store; the slice is copied explicitly to keep
	// the edit local to this test.
	edited := StandardData()
	categories := FzKategorien
	categories[0].Teilquellen = append([]Teilquelle(nil), categories[0].Teilquellen...)
	categories[0].Teilquellen[0].AA += 0.1
	edited.Tables[0] = framework.StandardDataTable{
		Name:  "anlage2/beiblatt1-fahrzeugkategorien",
		Value: categories,
	}

	after, err := edited.Digest(StandardID, framework.EvidenceTierNormative)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	if before.Digest == after.Digest {
		t.Fatal("editing a Beiblatt 1 coefficient did not move the digest")
	}
}
