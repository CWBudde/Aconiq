package schall03

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// This file defines the stable string vocabulary that external model formats
// use to name the normative Schall 03 enumerations.  The enum values
// themselves are ordinals of tables in Anlage 2 (Tabelle 7, 8, 15, 18) and are
// therefore renumbered whenever the reference row moves; a wire format must
// never carry those ordinals.  See PLAN.md 1.2 for the renumbering that made
// this explicit.

// The enumeration names used in wire-format error messages.  They mirror the
// wording of the Parse* helpers so a bad name and a bad JSON type read the
// same way.
const (
	kindFahrbahnart  = "Fahrbahnart"
	kindSFahrbahnart = "Straßenbahn Fahrbahnart"
	kindSurfaceCond  = "surface measure"
	kindWallSurface  = "wall surface"
)

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

// ---------------------------------------------------------------------------
// JSON wire format
// ---------------------------------------------------------------------------
//
// The four enumerations above are ordinals of Anlage 2 tables, and those
// ordinals move whenever the reference row of a table moves.  Scenario files
// therefore carry the names, never the numbers: a file written against an
// older numbering must fail loudly rather than decode to a different track
// type and shift levels by several dB in silence.  That is why a bare JSON
// number is rejected instead of being accepted for compatibility — it removes
// the need for a migration, because no old file can be misread.

// jsonNull is the literal encoding/json hands to an UnmarshalJSON
// implementation for a JSON null.
var jsonNull = []byte("null")

// decodeVocabularyJSON decodes one JSON value into an enum of the stable string
// vocabulary.  Only names are accepted; a JSON null is a no-op, following the
// encoding/json convention, and therefore leaves the reference row in place.
func decodeVocabularyJSON[T ~int](
	data []byte,
	target *T,
	kind string,
	names []string,
	parse func(string) (T, error),
) error {
	if bytes.Equal(bytes.TrimSpace(data), jsonNull) {
		return nil
	}

	var name string

	err := json.Unmarshal(data, &name)
	if err != nil {
		return fmt.Errorf(
			"%s must be named, got %s: expected one of %s, because the Anlage 2 table ordinals are renumbered when the reference row moves and are not a wire format",
			kind, string(data), strings.Join(names, ", "),
		)
	}

	value, err := parse(name)
	if err != nil {
		return err
	}

	*target = value

	return nil
}

// encodeVocabularyJSON encodes an enum as its canonical name.  The name is
// found by parsing every accepted name back, so Marshal and Unmarshal are
// exact inverses by construction rather than by an assumed table order.
func encodeVocabularyJSON[T ~int](
	value T,
	kind string,
	names []string,
	parse func(string) (T, error),
) ([]byte, error) {
	for _, name := range names {
		parsed, parseErr := parse(name)
		if parseErr != nil || parsed != value {
			continue
		}

		payload, err := json.Marshal(name)
		if err != nil {
			return nil, fmt.Errorf("encode %s %q: %w", kind, name, err)
		}

		return payload, nil
	}

	return nil, fmt.Errorf(
		"%s ordinal %d has no name: expected one of %s",
		kind, int(value), strings.Join(names, ", "),
	)
}

// UnmarshalJSON decodes a Tabelle 7 Fahrbahnart from its stable name.
func (t *FahrbahnartType) UnmarshalJSON(data []byte) error {
	return decodeVocabularyJSON(data, t, kindFahrbahnart, FahrbahnartNames(), ParseFahrbahnart)
}

// MarshalJSON encodes a Tabelle 7 Fahrbahnart as its stable name.
func (t FahrbahnartType) MarshalJSON() ([]byte, error) {
	return encodeVocabularyJSON(t, kindFahrbahnart, FahrbahnartNames(), ParseFahrbahnart)
}

// UnmarshalJSON decodes a Tabelle 15 Straßenbahn Fahrbahnart from its stable name.
func (t *SFahrbahnartType) UnmarshalJSON(data []byte) error {
	return decodeVocabularyJSON(data, t, kindSFahrbahnart, SFahrbahnartNames(), ParseSFahrbahnart)
}

// MarshalJSON encodes a Tabelle 15 Straßenbahn Fahrbahnart as its stable name.
func (t SFahrbahnartType) MarshalJSON() ([]byte, error) {
	return encodeVocabularyJSON(t, kindSFahrbahnart, SFahrbahnartNames(), ParseSFahrbahnart)
}

// UnmarshalJSON decodes a Tabelle 8 surface measure from its stable name.
func (t *SurfaceCondType) UnmarshalJSON(data []byte) error {
	return decodeVocabularyJSON(data, t, kindSurfaceCond, SurfaceCondNames(), ParseSurfaceCond)
}

// MarshalJSON encodes a Tabelle 8 surface measure as its stable name.
func (t SurfaceCondType) MarshalJSON() ([]byte, error) {
	return encodeVocabularyJSON(t, kindSurfaceCond, SurfaceCondNames(), ParseSurfaceCond)
}

// UnmarshalJSON decodes a Tabelle 18 wall surface from its stable name.
func (t *WallSurfaceType) UnmarshalJSON(data []byte) error {
	return decodeVocabularyJSON(data, t, kindWallSurface, WallSurfaceNames(), ParseWallSurface)
}

// MarshalJSON encodes a Tabelle 18 wall surface as its stable name.
func (t WallSurfaceType) MarshalJSON() ([]byte, error) {
	return encodeVocabularyJSON(t, kindWallSurface, WallSurfaceNames(), ParseWallSurface)
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
