package framework

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
)

const (
	// StandardDataDigestAlgorithm names the hash used for every standard-data
	// digest. It is recorded alongside the digest so a future change of hash is
	// visible in the artifact rather than inferred from the digest length.
	StandardDataDigestAlgorithm = "sha256"

	// standardDataDigestDomain separates this hash from any other SHA-256 in the
	// project, so a table digest can never collide with an input-file hash.
	standardDataDigestDomain = "aconiq.standard-data.v1"
)

// StandardDataTable is one named coefficient source a standards module carries:
// an embedded normative table, a bundled data pack, or — for a scaffold module
// — the invented constants it computes with.
//
// Name is a stable identifier, not a Go symbol: renaming the Go variable must
// not move the digest, and renaming the table deliberately must.
type StandardDataTable struct {
	Name  string
	Value any
}

// StandardData is everything a standards module carries that can change its
// numbers without changing its code path. A module that carries nothing —
// dummy-freefield computes from its parameters alone — returns the zero value,
// and no digest is recorded for it.
type StandardData struct {
	Tables []StandardDataTable
}

// IsEmpty reports whether the module carries no coefficient data at all.
func (d StandardData) IsEmpty() bool {
	return len(d.Tables) == 0
}

// StandardDataTableDigest is the digest of one named table.
type StandardDataTableDigest struct {
	Name   string
	Digest string
}

// StandardDataDigest identifies which module, at which evidence tier, with
// which coefficient tables produced a run. Two runs whose digests agree used
// byte-identical coefficient data; two runs whose digests differ did not, even
// if every other provenance field matches.
//
// The evidence tier is an input to the hash rather than a neighbouring field:
// re-tiering a module is a change in what its numbers mean, and a digest that
// ignored it would claim two incomparable runs were comparable.
type StandardDataDigest struct {
	StandardID   string
	EvidenceTier EvidenceTier
	Algorithm    string
	Digest       string
	Tables       []StandardDataTableDigest
}

// IsZero reports whether the digest is absent, which is the case for a module
// that carries no standard data.
func (d StandardDataDigest) IsZero() bool {
	return d.Digest == ""
}

// Digest computes the standard-data digest for one module.
//
// The result is independent of the order the module listed its tables in and of
// Go map iteration order: table entries are sorted by name and every map is
// encoded with its keys sorted by their canonical encoding. See
// docs/policies/determinism.md.
func (d StandardData) Digest(standardID string, tier EvidenceTier) (StandardDataDigest, error) {
	standardID = strings.TrimSpace(standardID)
	if standardID == "" {
		return StandardDataDigest{}, errors.New("standard data digest: standard id is required")
	}

	if d.IsEmpty() {
		return StandardDataDigest{}, nil
	}

	tables := make([]StandardDataTableDigest, 0, len(d.Tables))
	seen := make(map[string]struct{}, len(d.Tables))

	for _, table := range d.Tables {
		name := strings.TrimSpace(table.Name)
		if name == "" {
			return StandardDataDigest{}, fmt.Errorf("standard %q: standard data table name is required", standardID)
		}

		if _, exists := seen[name]; exists {
			return StandardDataDigest{}, fmt.Errorf("standard %q: standard data table %q is duplicated", standardID, name)
		}

		seen[name] = struct{}{}

		encoded, err := canonicalEncode(reflect.ValueOf(table.Value))
		if err != nil {
			return StandardDataDigest{}, fmt.Errorf("standard %q table %q: %w", standardID, name, err)
		}

		sum := sha256.Sum256([]byte(standardDataDigestDomain + "\x00table\x00" + name + "\x00" + encoded))
		tables = append(tables, StandardDataTableDigest{Name: name, Digest: hex.EncodeToString(sum[:])})
	}

	slices.SortFunc(tables, func(a, b StandardDataTableDigest) int {
		return strings.Compare(a.Name, b.Name)
	})

	overall := sha256.New()
	overall.Write([]byte(standardDataDigestDomain + "\x00" + standardID + "\x00" + string(tier)))

	for _, table := range tables {
		overall.Write([]byte("\x00" + table.Name + "\x00" + table.Digest))
	}

	return StandardDataDigest{
		StandardID:   standardID,
		EvidenceTier: tier,
		Algorithm:    StandardDataDigestAlgorithm,
		Digest:       hex.EncodeToString(overall.Sum(nil)),
		Tables:       tables,
	}, nil
}

// canonicalEncode renders a value as deterministic text.
//
// JSON would have been the obvious encoding and is not usable here: the RLS-19
// surface-correction table stores NaN for "not applicable", which encoding/json
// refuses outright, and a table holding an unexported struct field would encode
// as an empty object rather than fail. Reflection reads unexported fields
// through the kind-specific accessors — never through Interface(), which would
// panic — so what is hashed is what the table actually holds.
func canonicalEncode(value reflect.Value) (string, error) {
	var builder strings.Builder

	err := encodeValue(&builder, value)
	if err != nil {
		return "", err
	}

	return builder.String(), nil
}

func encodeValue(out *strings.Builder, value reflect.Value) error {
	if !value.IsValid() {
		out.WriteString("nil")

		return nil
	}

	switch value.Kind() {
	case reflect.Pointer, reflect.Interface:
		if value.IsNil() {
			out.WriteString("nil")

			return nil
		}

		return encodeValue(out, value.Elem())
	case reflect.Bool:
		out.WriteString(strconv.FormatBool(value.Bool()))

		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		out.WriteString(strconv.FormatInt(value.Int(), 10))

		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		out.WriteString(strconv.FormatUint(value.Uint(), 10))

		return nil
	case reflect.Float32, reflect.Float64:
		out.WriteString(encodeFloat(value.Float()))

		return nil
	case reflect.String:
		out.WriteString(strconv.Quote(value.String()))

		return nil
	case reflect.Slice, reflect.Array:
		return encodeList(out, value)
	case reflect.Map:
		return encodeMap(out, value)
	case reflect.Struct:
		return encodeStruct(out, value)
	default:
		return fmt.Errorf("standard data cannot contain %s values", value.Kind())
	}
}

// encodeFloat renders a float exactly. NaN and the infinities are legitimate
// table entries — RLS-19 Tabelle 4 uses NaN for "not applicable" — so they are
// spelled out rather than rejected, and -0 is normalized to 0 so a sign that
// carries no acoustic meaning cannot move the digest.
func encodeFloat(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "+Inf"
	case math.IsInf(f, -1):
		return "-Inf"
	case f == 0:
		return "0"
	default:
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
}

func encodeList(out *strings.Builder, value reflect.Value) error {
	if value.Kind() == reflect.Slice && value.IsNil() {
		out.WriteString("nil")

		return nil
	}

	out.WriteString("[")

	for i := range value.Len() {
		if i > 0 {
			out.WriteString(",")
		}

		err := encodeValue(out, value.Index(i))
		if err != nil {
			return err
		}
	}

	out.WriteString("]")

	return nil
}

func encodeMap(out *strings.Builder, value reflect.Value) error {
	if value.IsNil() {
		out.WriteString("nil")

		return nil
	}

	entries := make([]string, 0, value.Len())

	iter := value.MapRange()
	for iter.Next() {
		var entry strings.Builder

		err := encodeValue(&entry, iter.Key())
		if err != nil {
			return err
		}

		entry.WriteString(":")

		err = encodeValue(&entry, iter.Value())
		if err != nil {
			return err
		}

		entries = append(entries, entry.String())
	}

	// Sorting the fully encoded entries, rather than the keys alone, keeps the
	// order total even for key types whose encodings could otherwise tie.
	sort.Strings(entries)

	out.WriteString("{")
	out.WriteString(strings.Join(entries, ","))
	out.WriteString("}")

	return nil
}

func encodeStruct(out *strings.Builder, value reflect.Value) error {
	structType := value.Type()

	out.WriteString("{")

	for i := range structType.NumField() {
		if i > 0 {
			out.WriteString(",")
		}

		out.WriteString(structType.Field(i).Name)
		out.WriteString(":")

		err := encodeValue(out, value.Field(i))
		if err != nil {
			return err
		}
	}

	out.WriteString("}")

	return nil
}
