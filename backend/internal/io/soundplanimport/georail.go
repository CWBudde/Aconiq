package soundplanimport

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

// RailTrack represents a single rail track extracted from a GeoRail.geo file.
type RailTrack struct {
	Name     string
	Segments []RailSegment
}

// RailSegment is a group of consecutive coordinate points sharing the same
// emission parameters. A track is split into segments at points where
// parameters change (e.g. bridge surcharges).
type RailSegment struct {
	Points []TrackPoint
	Params RailSegmentParams
}

// TrackPoint holds a single coordinate along a rail track.
type TrackPoint struct {
	X       float64 // easting (local or projected CRS)
	Y       float64 // northing
	ZTrack  float64 // rail head elevation
	ZGround float64 // ground elevation below track
}

// RailSegmentParams holds emission-relevant parameters for a rail segment,
// extracted from :D= records.
type RailSegmentParams struct {
	Speed            float64 // design speed [km/h] (field 0)
	BridgeCorrection float64 // K_Br [dB], -1000 = no bridge (field 9)
	TrackHeight      float64 // track height above ground? (field 18)
}

// ParseGeoRailFile reads a SoundPlan GeoRail.geo binary file and extracts
// rail track geometry and parameters.
func ParseGeoRailFile(path string) ([]RailTrack, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("soundplan: read georail: %w", err)
	}

	return parseGeoRailData(data)
}

// railParser holds mutable state during geo rail file parsing.
type railParser struct {
	data          []byte
	tracks        []RailTrack
	current       *RailTrack
	currentPoints []TrackPoint
	currentParams RailSegmentParams
}

func parseGeoRailData(data []byte) ([]RailTrack, error) {
	p := &railParser{data: data}
	i := 0

	for i < len(data)-6 {
		if data[i] != ':' {
			i++

			continue
		}

		tag := string(data[i+1 : i+3])

		switch {
		case tag == "O&":
			i = p.handleObjectGroup(i)
		case tag == "G ":
			i = p.handlePoint(i)
		case tag == "D1":
			i = p.handleName(i)
		case data[i+1] == 'D' && data[i+2] == '=':
			i = p.handleParams(i)
		default:
			i++
		}
	}

	// Flush last track.
	if p.current != nil {
		p.flushSegment()
		p.tracks = append(p.tracks, *p.current)
	}

	return p.tracks, nil
}

func (p *railParser) handleObjectGroup(i int) int {
	if p.current != nil {
		p.flushSegment()
		p.tracks = append(p.tracks, *p.current)
	}

	p.current = &RailTrack{}
	p.currentPoints = make([]TrackPoint, 0, 16)
	p.currentParams = RailSegmentParams{}

	return i + 3
}

func (p *railParser) handlePoint(i int) int {
	recEnd := i + 6 + 32
	if recEnd > len(p.data) {
		return i + 3
	}

	off := i + 6

	p.currentPoints = append(p.currentPoints, TrackPoint{
		X:       readF64(p.data, off),
		Y:       readF64(p.data, off+8),
		ZTrack:  readF64(p.data, off+16),
		ZGround: readF64(p.data, off+24),
	})

	return recEnd
}

func (p *railParser) handleName(i int) int {
	off := i + 6
	if off+9 > len(p.data) {
		return i + 3
	}

	off += 8 // skip hash + unknown u32
	strLen := int(p.data[off])
	off++

	if strLen > 0 && off+strLen <= len(p.data) && p.current != nil {
		p.current.Name = string(p.data[off : off+strLen])
	}

	return off + strLen
}

func (p *railParser) handleParams(i int) int {
	off := i + 3 + 12 // skip marker + u32 + hash + u32

	if off+20*8 > len(p.data) {
		return i + 3
	}

	p.currentParams = RailSegmentParams{
		Speed:            readF64(p.data, off),
		BridgeCorrection: readF64(p.data, off+9*8),
		TrackHeight:      readF64(p.data, off+18*8),
	}

	if p.current != nil {
		p.flushSegment()
	}

	return off + 20*8
}

func (p *railParser) flushSegment() {
	if len(p.currentPoints) == 0 {
		return
	}

	p.current.Segments = append(p.current.Segments, RailSegment{
		Points: p.currentPoints,
		Params: p.currentParams,
	})

	p.currentPoints = nil
}

func readF64(data []byte, off int) float64 {
	return math.Float64frombits(binary.LittleEndian.Uint64(data[off:]))
}
