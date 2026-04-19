package soundplanimport

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

const (
	dgmHeaderValueCount  = 8
	dgmVertexCountIndex  = 3
	dgmVertexBlockOffset = 100
	dgmVertexRecordSize  = 24
)

// DGMData holds one parsed SoundPLAN digital ground model file.
type DGMData struct {
	SourceFile   string
	HeaderValues [dgmHeaderValueCount]uint32
	Points       []ElevationPoint
}

// ParseDGMFile reads one SoundPLAN RDGM*.dgm binary and extracts the verified
// vertex table used for terrain samples.
func ParseDGMFile(path string) (*DGMData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("soundplan: read dgm: %w", err)
	}

	return parseDGMData(filepath.Base(path), data)
}

func parseDGMData(sourceFile string, data []byte) (*DGMData, error) {
	if len(data) < dgmVertexBlockOffset {
		return nil, fmt.Errorf("soundplan: dgm: file too short for header and vertex table")
	}

	dgm := &DGMData{SourceFile: sourceFile}
	for i := range dgm.HeaderValues {
		dgm.HeaderValues[i] = readU32(data, i*4)
	}

	vertexCount := int(dgm.HeaderValues[dgmVertexCountIndex])
	if vertexCount <= 0 {
		return nil, fmt.Errorf("soundplan: dgm: missing vertex count in header")
	}

	vertexBlockEnd := dgmVertexBlockOffset + vertexCount*dgmVertexRecordSize
	if vertexBlockEnd > len(data) {
		return nil, fmt.Errorf("soundplan: dgm: vertex table truncated: need %d bytes, got %d", vertexBlockEnd, len(data))
	}

	dgm.Points = make([]ElevationPoint, 0, vertexCount)
	for i := 0; i < vertexCount; i++ {
		off := dgmVertexBlockOffset + i*dgmVertexRecordSize
		dgm.Points = append(dgm.Points, ElevationPoint{
			X: readF64(data, off),
			Y: readF64(data, off+8),
			Z: float64(readF32(data, off+16)),
		})
	}

	return dgm, nil
}

func readF32(data []byte, off int) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(data[off:]))
}
