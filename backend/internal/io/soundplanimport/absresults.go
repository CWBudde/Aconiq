package soundplanimport

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	absdb "github.com/cwbudde/go-absolute-database"
)

// ReceiverResult holds per-receiver immission levels from RREC*.abs files.
type ReceiverResult struct {
	RecNo     int32
	Floor     int32
	ObjID     int32
	PointID   string
	Name      string
	Usage     int32
	X         float64 // may be zero for building facade receivers
	Y         float64
	Z         float64
	GH        float64 // ground height
	Limit1    float64
	ZB1       float64 // assessment period 1 level (e.g. Tag)
	ZBLimit1  float64
	Limit2    float64
	ZB2       float64 // assessment period 2 level (e.g. Nacht)
	ZBLimit2  float64
	HasCoords bool // true if X/Y were non-null
}

// GroupResult holds per-receiver group immission levels from RGRP*.abs files.
type GroupResult struct {
	GrpNo int32
	RecNo int32
	Floor int32
	GName string  // noise group name (e.g. "Default railway noise")
	ZB1   float64 // assessment period 1 level (e.g. Lr,Tag)
	ZB2   float64 // assessment period 2 level (e.g. Lr,Nacht)
}

// PartialResult holds per-source/per-receiver propagation data from RMPA*.abs files.
type PartialResult struct {
	IDX      int32
	SrcNo    int32
	RecNo    int32
	Floor    int32
	ZBName   string
	QName    string // source name
	SrcObjID int32
	Lw       float64 // sound power level
	LwStar   float64 // sound power level with directivity
	KI       float64 // ground correction
	KT       float64 // meteorological correction
	KO       float64 // obstacle correction
	KBonus   float64 // rail bonus
	Distance float64 // s/m
	Adiv     float64 // divergence attenuation
	Agnd     float64 // ground attenuation
	Abar     float64 // barrier attenuation
	Aair     float64 // air absorption
	ADI      float64 // directivity index
	Ls       float64 // sound pressure level
	DLrefl   float64 // reflection correction
	Lr       float64 // rating level
}

// TrainType holds a Schall 03 train type definition from TS03.abs.
type TrainType struct {
	ZugArt  int32   // train type ID (AutoInc)
	Name    string  // train type name
	SBA     float64 // base sound level / Schallleistungspegel
	Vmax    float64 // max speed km/h
	LZug    float64 // train length m
	DFz     float64 // vehicle correction
	DAo     float64 // other correction
	Comment string  // optional comment (from Memo field)
}

// RailEmission holds per-track emission data from RRAI*.abs files.
type RailEmission struct {
	IDX      int32
	ObjID    int32
	Railname string
	KM       float64
	DFb      float64 // trackbed correction
	DRa      float64 // roughness correction
	DRz      float64 // braking correction
	DBue     float64 // bridge surcharge
	DBr      float64 // brake type correction
	TrackV   float64 // track max speed km/h
	LmEDay   float64 // emission level day
	LmENight float64 // emission level night
}

// TrainEmission holds per-train emission data from RRAD*.abs files.
type TrainEmission struct {
	No        int32
	IDX       int32  // references RRAI IDX
	Trainname string // train type name
	NDay      float64
	NNight    float64
	Percent   float64
	Speed     float64 // v/km/h
	Length    float64 // l/m
	DFzDAo    float64 // DFz+DAo/dB
	Max       bool
	LmEDay    float64
	LmENight  float64
}

// RunResults holds all parsed result data from a SoundPlan calculation run.
type RunResults struct {
	Receivers []ReceiverResult
	Groups    []GroupResult
	Partials  []PartialResult
}

// ParseReceiverResults reads per-receiver results from an RREC*.abs file.
func ParseReceiverResults(path string) ([]ReceiverResult, error) {
	db, err := absdb.Open(path)
	if err != nil {
		return nil, fmt.Errorf("soundplan: open RREC: %w", err)
	}
	defer db.Close()

	reader, err := db.OpenTable()
	if err != nil {
		return nil, fmt.Errorf("soundplan: open RREC table: %w", err)
	}

	cols := buildColumnIndex(reader.Schema())

	var results []ReceiverResult

	for reader.Next() {
		rec := reader.Record()
		r := ReceiverResult{
			RecNo: readInt(rec, cols, "RecNo"),
			Floor: readInt(rec, cols, "Floor"),
			ObjID: readInt(rec, cols, "ObjID"),
			Name:  readStr(rec, cols, "Name"),
			Usage: readInt(rec, cols, "Usage"),
		}

		if idx, ok := cols["PointID"]; ok && !rec.IsNull(idx) {
			r.PointID = rec.String(idx)
		}

		if idx, ok := cols["X/m"]; ok && !rec.IsNull(idx) {
			r.X = rec.Float(idx)
			r.HasCoords = true
		}

		if idx, ok := cols["Y/m"]; ok && !rec.IsNull(idx) {
			r.Y = rec.Float(idx)
		}

		if idx, ok := cols["Z/m"]; ok && !rec.IsNull(idx) {
			r.Z = rec.Float(idx)
		}

		if idx, ok := cols["GH/m"]; ok && !rec.IsNull(idx) {
			r.GH = rec.Float(idx)
		}

		r.Limit1 = readFloat(rec, cols, "Limit1")
		r.ZB1 = readFloat(rec, cols, "ZB1")
		r.ZBLimit1 = readFloat(rec, cols, "ZB_Limit1")
		r.Limit2 = readFloat(rec, cols, "Limit2")
		r.ZB2 = readFloat(rec, cols, "ZB2")
		r.ZBLimit2 = readFloat(rec, cols, "ZB_Limit2")

		results = append(results, r)
	}

	err = reader.Err()
	if err != nil {
		return nil, fmt.Errorf("soundplan: read RREC: %w", err)
	}

	return results, nil
}

// ParseGroupResults reads per-receiver group immission levels from a RGRP*.abs file.
func ParseGroupResults(path string) ([]GroupResult, error) {
	db, err := absdb.Open(path)
	if err != nil {
		return nil, fmt.Errorf("soundplan: open RGRP: %w", err)
	}
	defer db.Close()

	reader, err := db.OpenTable()
	if err != nil {
		return nil, fmt.Errorf("soundplan: open RGRP table: %w", err)
	}

	cols := buildColumnIndex(reader.Schema())

	var results []GroupResult

	for reader.Next() {
		rec := reader.Record()
		results = append(results, GroupResult{
			GrpNo: readInt(rec, cols, "GrpNo"),
			RecNo: readInt(rec, cols, "RecNo"),
			Floor: readInt(rec, cols, "Floor"),
			GName: readStr(rec, cols, "GName"),
			ZB1:   readFloat(rec, cols, "ZB1"),
			ZB2:   readFloat(rec, cols, "ZB2"),
		})
	}

	err = reader.Err()
	if err != nil {
		return nil, fmt.Errorf("soundplan: read RGRP: %w", err)
	}

	return results, nil
}

// ParsePartialResults reads per-source/per-receiver propagation data from an RMPA*.abs file.
func ParsePartialResults(path string) ([]PartialResult, error) {
	db, err := absdb.Open(path)
	if err != nil {
		return nil, fmt.Errorf("soundplan: open RMPA: %w", err)
	}
	defer db.Close()

	reader, err := db.OpenTable()
	if err != nil {
		return nil, fmt.Errorf("soundplan: open RMPA table: %w", err)
	}

	cols := buildColumnIndex(reader.Schema())

	var results []PartialResult

	for reader.Next() {
		rec := reader.Record()
		results = append(results, PartialResult{
			IDX:      readInt(rec, cols, "IDX"),
			SrcNo:    readInt(rec, cols, "SrcNo"),
			RecNo:    readInt(rec, cols, "RecNo"),
			Floor:    readInt(rec, cols, "Floor"),
			ZBName:   readStr(rec, cols, "ZBName"),
			QName:    readStr(rec, cols, "QName"),
			SrcObjID: readInt(rec, cols, "SrcObjID"),
			Lw:       readFloat(rec, cols, "Lw/dB"),
			LwStar:   readFloat(rec, cols, "Lw*/dB"),
			KI:       readFloat(rec, cols, "KI/dB"),
			KT:       readFloat(rec, cols, "KT/dB"),
			KO:       readFloat(rec, cols, "KO/dB"),
			KBonus:   readFloat(rec, cols, "KBonus/dB"),
			Distance: readFloat(rec, cols, "s/m"),
			Adiv:     readFloat(rec, cols, "Adiv/dB"),
			Agnd:     readFloat(rec, cols, "Agnd/dB"),
			Abar:     readFloat(rec, cols, "Abar/dB"),
			Aair:     readFloat(rec, cols, "Aair/dB"),
			ADI:      readFloat(rec, cols, "ADI/dB"),
			Ls:       readFloat(rec, cols, "Ls/dB"),
			DLrefl:   readFloat(rec, cols, "dLrefl/dB"),
			Lr:       readFloat(rec, cols, "Lr"),
		})
	}

	err = reader.Err()
	if err != nil {
		return nil, fmt.Errorf("soundplan: read RMPA: %w", err)
	}

	return results, nil
}

// ParseTrainTypes reads Schall 03 train type definitions from a TS03.abs file.
func ParseTrainTypes(path string) ([]TrainType, error) {
	db, err := absdb.Open(path)
	if err != nil {
		return nil, fmt.Errorf("soundplan: open TS03: %w", err)
	}
	defer db.Close()

	reader, err := db.OpenTable()
	if err != nil {
		return nil, fmt.Errorf("soundplan: open TS03 table: %w", err)
	}

	cols := buildColumnIndex(reader.Schema())

	var types []TrainType

	for reader.Next() {
		rec := reader.Record()
		tt := TrainType{
			ZugArt: readInt(rec, cols, "ZugArt"),
			Name:   readStr(rec, cols, "Name"),
			SBA:    readFloat(rec, cols, "SBA"),
			Vmax:   readFloat(rec, cols, "Vmax"),
			LZug:   readFloat(rec, cols, "LZug"),
			DFz:    readFloat(rec, cols, "DFz"),
			DAo:    readFloat(rec, cols, "DAo"),
		}

		// Read Kommentar memo field if present.
		if idx, ok := cols["Kommentar"]; ok && !rec.IsNull(idx) {
			comment, memoErr := rec.Memo(idx)
			if memoErr == nil {
				tt.Comment = comment
			}
		}

		types = append(types, tt)
	}

	err = reader.Err()
	if err != nil {
		return nil, fmt.Errorf("soundplan: read TS03: %w", err)
	}

	return types, nil
}

// ParseRailEmissions reads per-track emission data from an RRAI*.abs file.
func ParseRailEmissions(path string) ([]RailEmission, error) {
	return readAbsTable(path, "RRAI", func(rec absdb.Record, cols map[string]int) RailEmission {
		return RailEmission{
			IDX:      readInt(rec, cols, "IDX"),
			ObjID:    readInt(rec, cols, "ObjID"),
			Railname: readStr(rec, cols, "Railname"),
			KM:       readFloat(rec, cols, "KM"),
			DFb:      readFloat(rec, cols, "DFb/dB"),
			DRa:      readFloat(rec, cols, "DRa/dB"),
			DRz:      readFloat(rec, cols, "DRz/dB"),
			DBue:     readFloat(rec, cols, "DBue/dB"),
			DBr:      readFloat(rec, cols, "DBr/dB"),
			TrackV:   readFloat(rec, cols, "Track vMax/km/h"),
			LmEDay:   readFloat(rec, cols, "LmE(6-22)/dB(A)"),
			LmENight: readFloat(rec, cols, "LmE(22-6)/dB(A)"),
		}
	})
}

// ParseTrainEmissions reads per-train emission data from an RRAD*.abs file.
func ParseTrainEmissions(path string) ([]TrainEmission, error) {
	return readAbsTable(path, "RRAD", func(rec absdb.Record, cols map[string]int) TrainEmission {
		return TrainEmission{
			No:        readInt(rec, cols, "No"),
			IDX:       readInt(rec, cols, "IDX"),
			Trainname: readStr(rec, cols, "Trainname"),
			NDay:      readFloat(rec, cols, "N(6-22)"),
			NNight:    readFloat(rec, cols, "N(22-6)"),
			Percent:   readFloat(rec, cols, "p/%"),
			Speed:     readFloat(rec, cols, "v/km/h"),
			Length:    readFloat(rec, cols, "l/m"),
			DFzDAo:    readFloat(rec, cols, "DFz+DAo/dB"),
			Max:       readBool(rec, cols, "Max"),
			LmEDay:    readFloat(rec, cols, "LmE(6-22)/dB(A)"),
			LmENight:  readFloat(rec, cols, "LmE(22-6)/dB(A)"),
		}
	})
}

// readAbsTable is a generic helper that opens an .abs file, iterates records,
// and maps each to a typed value using the provided function.
func readAbsTable[T any](path, label string, mapRow func(absdb.Record, map[string]int) T) ([]T, error) {
	db, err := absdb.Open(path)
	if err != nil {
		return nil, fmt.Errorf("soundplan: open %s: %w", label, err)
	}
	defer db.Close()

	reader, err := db.OpenTable()
	if err != nil {
		return nil, fmt.Errorf("soundplan: open %s table: %w", label, err)
	}

	cols := buildColumnIndex(reader.Schema())

	var results []T

	for reader.Next() {
		results = append(results, mapRow(reader.Record(), cols))
	}

	err = reader.Err()
	if err != nil {
		return nil, fmt.Errorf("soundplan: read %s: %w", label, err)
	}

	return results, nil
}

// LoadRunResults loads all result .abs files from a SoundPlan result subdirectory
// (e.g. RSPS0011/). It reads RREC, RGRP, and RMPA files.
func LoadRunResults(resultDir string) (*RunResults, error) {
	base := filepath.Base(resultDir)
	// Extract the 4-digit run ID suffix (e.g. "0011" from "RSPS0011").
	suffix := extractRunSuffix(base)

	results := &RunResults{}

	// RREC: receiver metadata and summary levels.
	rrecPath := filepath.Join(resultDir, "RREC"+suffix+".abs")
	if fileExists(rrecPath) {
		recs, err := ParseReceiverResults(rrecPath)
		if err != nil {
			return nil, err
		}

		results.Receivers = recs
	}

	// RGRP: group immission levels.
	rgrpPath := filepath.Join(resultDir, "RGRP"+suffix+".abs")
	if fileExists(rgrpPath) {
		grps, err := ParseGroupResults(rgrpPath)
		if err != nil {
			return nil, err
		}

		results.Groups = grps
	}

	// RMPA: partial levels (source contributions).
	rmpaPath := filepath.Join(resultDir, "RMPA"+suffix+".abs")
	if fileExists(rmpaPath) {
		parts, err := ParsePartialResults(rmpaPath)
		if err != nil {
			return nil, err
		}

		results.Partials = parts
	}

	return results, nil
}

// extractRunSuffix extracts the numeric suffix from a result directory name.
// e.g. "RSPS0011" → "0011", "RRLK0022" → "0022".
func extractRunSuffix(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] < '0' || name[i] > '9' {
			return name[i+1:]
		}
	}

	return name
}

// fileExists returns true if the file exists and is not a directory.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return !info.IsDir()
}

// ---------------------------------------------------------------------------
// column access helpers
// ---------------------------------------------------------------------------

// buildColumnIndex creates a name→position map from a table schema.
func buildColumnIndex(schema *absdb.TableSchema) map[string]int {
	idx := make(map[string]int, len(schema.Columns))
	for _, c := range schema.Columns {
		idx[c.Name] = c.Position
	}

	return idx
}

// readInt reads an int32 column by name, returning 0 if not found or null.
func readInt(rec absdb.Record, cols map[string]int, name string) int32 {
	idx, ok := cols[name]
	if !ok || rec.IsNull(idx) {
		return 0
	}

	return rec.Int(idx)
}

// readFloat reads a float64 column by name, returning 0 if not found or null.
func readFloat(rec absdb.Record, cols map[string]int, name string) float64 {
	idx, ok := cols[name]
	if !ok || rec.IsNull(idx) {
		return 0
	}

	return rec.Float(idx)
}

// readStr reads a string column by name, returning "" if not found or null.
func readStr(rec absdb.Record, cols map[string]int, name string) string {
	idx, ok := cols[name]
	if !ok || rec.IsNull(idx) {
		return ""
	}

	return strings.TrimSpace(rec.String(idx))
}

// readBool reads a boolean column by name, returning false if not found or null.
func readBool(rec absdb.Record, cols map[string]int, name string) bool {
	idx, ok := cols[name]
	if !ok || rec.IsNull(idx) {
		return false
	}

	return rec.Bool(idx)
}
