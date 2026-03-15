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

func parseGeoRailData(data []byte) ([]RailTrack, error) {
	var tracks []RailTrack
	var current *RailTrack
	var currentPoints []TrackPoint
	var currentParams RailSegmentParams

	i := 0

	for i < len(data)-6 {
		if data[i] != ':' {
			i++

			continue
		}

		tag := string(data[i+1 : i+3])

		switch {
		case tag == "O&":
			// Object group start — flush previous track's last segment.
			if current != nil {
				flushSegment(current, &currentPoints, currentParams)
				tracks = append(tracks, *current)
			}

			current = &RailTrack{}
			currentPoints = nil
			currentParams = RailSegmentParams{}
			i += 3

		case tag == "G ":
			// Coordinate point: 3 marker + 3 padding + 4×float64.
			recEnd := i + 6 + 32
			if recEnd > len(data) {
				break
			}

			off := i + 6
			pt := TrackPoint{
				X:       readF64(data, off),
				Y:       readF64(data, off+8),
				ZTrack:  readF64(data, off+16),
				ZGround: readF64(data, off+24),
			}
			currentPoints = append(currentPoints, pt)
			i = recEnd

		case tag == "D1":
			// Name record: 3 marker + 3 padding + 4 hash + 4 unknown + 1 strLen + string.
			off := i + 6
			if off+9 > len(data) {
				i += 3

				continue
			}

			off += 8 // skip hash + unknown u32
			strLen := int(data[off])
			off++

			if strLen > 0 && off+strLen <= len(data) && current != nil {
				current.Name = string(data[off : off+strLen])
			}

			i = off + strLen

		case data[i+1] == 'D' && data[i+2] == '=':
			// Rail parameter block: 3 marker + 4 + 4 hash + 4 + N×float64.
			// Parameters apply to the preceding coordinate points.
			off := i + 3 + 12 // skip marker + u32 + hash + u32
			if off+20*8 > len(data) {
				i += 3

				continue
			}

			currentParams = RailSegmentParams{
				Speed:            readF64(data, off),
				BridgeCorrection: readF64(data, off+9*8),
				TrackHeight:      readF64(data, off+18*8),
			}

			// Flush current points with these params.
			if current != nil {
				flushSegment(current, &currentPoints, currentParams)
			}

			i = off + 20*8

		case tag == "DL":
			// Data link end — skip.
			i += 3

		default:
			i++
		}
	}

	// Flush last track.
	if current != nil {
		flushSegment(current, &currentPoints, currentParams)
		tracks = append(tracks, *current)
	}

	return tracks, nil
}

func flushSegment(track *RailTrack, points *[]TrackPoint, params RailSegmentParams) {
	if len(*points) == 0 {
		return
	}

	track.Segments = append(track.Segments, RailSegment{
		Points: *points,
		Params: params,
	})

	*points = nil
}

func readF64(data []byte, off int) float64 {
	return math.Float64frombits(binary.LittleEndian.Uint64(data[off:]))
}
