package schall03

import (
	"fmt"
	"strings"
)

// This file defines the stable string vocabulary that external model formats
// use to name the normative Schall 03 enumerations.  The enum values
// themselves are ordinals of tables in Anlage 2 (Tabelle 7, 8, 15, 18) and are
// therefore renumbered whenever the reference row moves; a wire format must
// never carry those ordinals.  See PLAN.md 1.2 for the renumbering that made
// this explicit.

// fahrbahnartNames maps the Tabelle 7 vocabulary to FahrbahnartType.
var fahrbahnartNames = []struct {
	Name string
	Type FahrbahnartType
}{
	{"schwellengleis", FahrbahnartSchwellengleis},
	{"feste-fahrbahn", FahrbahnartFesteFahrbahn},
	{"feste-fahrbahn-mit-absorber", FahrbahnartFesteFahrbahnMitAbsorber},
	{"bahnuebergang", FahrbahnartBahnuebergang},
}

// sFahrbahnartNames maps the Tabelle 15 vocabulary to SFahrbahnartType.
var sFahrbahnartNames = []struct {
	Name string
	Type SFahrbahnartType
}{
	{"schwellengleis", SFahrbahnSchwellengleis},
	{"strassenbuendig", SFahrbahnStrassenbuendig},
	{"begruent-tief", SFahrbahnGruenTief},
	{"begruent-hoch", SFahrbahnGruenHoch},
}

// surfaceCondNames maps the Tabelle 8 vocabulary to SurfaceCondType.
var surfaceCondNames = []struct {
	Name string
	Type SurfaceCondType
}{
	{"none", SurfaceCondNone},
	{"bug", SurfaceCondBuG},
	{"schienenstegdaempfer", SurfaceCondSchienenstegdaempf},
	{"schienenstegabschirmung", SurfaceCondSchienenstegabschirm},
}

// wallSurfaceNames maps the Tabelle 18 vocabulary to WallSurfaceType.
var wallSurfaceNames = []struct {
	Name string
	Type WallSurfaceType
}{
	{"hard", WallSurfaceHard},
	{"building", WallSurfaceBuilding},
	{"absorbing", WallSurfaceAbsorbing},
	{"highly-absorbing", WallSurfaceHighlyAbsorbing},
}

func normalizeVocabularyName(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// ParseFahrbahnart resolves a Tabelle 7 Fahrbahnart name.  An empty string
// resolves to the reference type Schwellengleis, which carries no correction.
func ParseFahrbahnart(raw string) (FahrbahnartType, error) {
	name := normalizeVocabularyName(raw)
	if name == "" {
		return FahrbahnartSchwellengleis, nil
	}

	for _, entry := range fahrbahnartNames {
		if entry.Name == name {
			return entry.Type, nil
		}
	}

	return 0, fmt.Errorf("unknown Fahrbahnart %q, expected one of %s", raw, strings.Join(FahrbahnartNames(), ", "))
}

// FahrbahnartNames lists the accepted Tabelle 7 Fahrbahnart names.
func FahrbahnartNames() []string {
	out := make([]string, 0, len(fahrbahnartNames))
	for _, entry := range fahrbahnartNames {
		out = append(out, entry.Name)
	}

	return out
}

// ParseSFahrbahnart resolves a Tabelle 15 Straßenbahn Fahrbahnart name.  An
// empty string resolves to the reference type Schwellengleis.
func ParseSFahrbahnart(raw string) (SFahrbahnartType, error) {
	name := normalizeVocabularyName(raw)
	if name == "" {
		return SFahrbahnSchwellengleis, nil
	}

	for _, entry := range sFahrbahnartNames {
		if entry.Name == name {
			return entry.Type, nil
		}
	}

	return 0, fmt.Errorf("unknown Straßenbahn Fahrbahnart %q, expected one of %s", raw, strings.Join(SFahrbahnartNames(), ", "))
}

// SFahrbahnartNames lists the accepted Tabelle 15 Fahrbahnart names.
func SFahrbahnartNames() []string {
	out := make([]string, 0, len(sFahrbahnartNames))
	for _, entry := range sFahrbahnartNames {
		out = append(out, entry.Name)
	}

	return out
}

// ParseSurfaceCond resolves a Tabelle 8 surface-measure name.  An empty string
// resolves to "no measure active".
func ParseSurfaceCond(raw string) (SurfaceCondType, error) {
	name := normalizeVocabularyName(raw)
	if name == "" {
		return SurfaceCondNone, nil
	}

	for _, entry := range surfaceCondNames {
		if entry.Name == name {
			return entry.Type, nil
		}
	}

	return 0, fmt.Errorf("unknown surface measure %q, expected one of %s", raw, strings.Join(SurfaceCondNames(), ", "))
}

// SurfaceCondNames lists the accepted Tabelle 8 surface-measure names.
func SurfaceCondNames() []string {
	out := make([]string, 0, len(surfaceCondNames))
	for _, entry := range surfaceCondNames {
		out = append(out, entry.Name)
	}

	return out
}

// ParseWallSurface resolves a Tabelle 18 wall-surface name.  An empty string
// resolves to the hard reference surface (D_ρ = 0 dB), which is the
// conservative choice for a reflecting wall of unstated construction.
func ParseWallSurface(raw string) (WallSurfaceType, error) {
	name := normalizeVocabularyName(raw)
	if name == "" {
		return WallSurfaceHard, nil
	}

	for _, entry := range wallSurfaceNames {
		if entry.Name == name {
			return entry.Type, nil
		}
	}

	return 0, fmt.Errorf("unknown wall surface %q, expected one of %s", raw, strings.Join(WallSurfaceNames(), ", "))
}

// WallSurfaceNames lists the accepted Tabelle 18 wall-surface names.
func WallSurfaceNames() []string {
	out := make([]string, 0, len(wallSurfaceNames))
	for _, entry := range wallSurfaceNames {
		out = append(out, entry.Name)
	}

	return out
}

// ZugartNames lists every Zugart the Beiblatt 1 (Eisenbahn) and Beiblatt 2
// (Straßenbahn) tables define, in table order.
func ZugartNames() []string {
	out := make([]string, 0, len(Zugarten)+len(ZugartStrassenbahn))
	for _, z := range Zugarten {
		out = append(out, z.Name)
	}

	for _, z := range ZugartStrassenbahn {
		out = append(out, z.Name)
	}

	return out
}
