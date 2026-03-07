package geo

import (
	"errors"
	"fmt"
	"math"
)

// IndexedItem represents one spatial candidate with an ID and bbox.
type IndexedItem struct {
	ID   string
	BBox BBox
}

// SpatialIndex provides candidate queries using bounding boxes.
type SpatialIndex interface {
	Insert(item IndexedItem) error
	Query(query BBox) ([]IndexedItem, error)
	Len() int
}

// GridSpatialIndex is a deterministic grid-bucket spatial index.
// It is an R-tree equivalent candidate index for Phase 5.
type GridSpatialIndex struct {
	cellSize float64
	cells    map[cellKey][]IndexedItem
	items    map[string]IndexedItem
}

type cellKey struct {
	X int64
	Y int64
}

func NewGridSpatialIndex(cellSize float64) (*GridSpatialIndex, error) {
	if cellSize <= 0 || math.IsNaN(cellSize) || math.IsInf(cellSize, 0) {
		return nil, errors.New("cell size must be a finite number > 0")
	}

	return &GridSpatialIndex{
		cellSize: cellSize,
		cells:    make(map[cellKey][]IndexedItem),
		items:    make(map[string]IndexedItem),
	}, nil
}

func (s *GridSpatialIndex) Len() int {
	if s == nil {
		return 0
	}

	return len(s.items)
}

func (s *GridSpatialIndex) Insert(item IndexedItem) error {
	if s == nil {
		return errors.New("spatial index is nil")
	}

	if item.ID == "" {
		return errors.New("indexed item id is required")
	}

	if !item.BBox.IsFinite() || !item.BBox.IsValid() {
		return errors.New("indexed item bbox is invalid")
	}

	if _, exists := s.items[item.ID]; exists {
		return fmt.Errorf("indexed item %q already exists", item.ID)
	}

	s.items[item.ID] = item
	for _, key := range s.coveredCells(item.BBox) {
		s.cells[key] = append(s.cells[key], item)
	}

	return nil
}

func (s *GridSpatialIndex) Query(query BBox) ([]IndexedItem, error) {
	if s == nil {
		return nil, errors.New("spatial index is nil")
	}

	if !query.IsFinite() || !query.IsValid() {
		return nil, errors.New("query bbox is invalid")
	}

	seen := make(map[string]struct{})
	results := make([]IndexedItem, 0)

	for _, key := range s.coveredCells(query) {
		items := s.cells[key]
		for _, item := range items {
			if _, ok := seen[item.ID]; ok {
				continue
			}

			if !item.BBox.Intersects(query) {
				continue
			}

			seen[item.ID] = struct{}{}
			results = append(results, item)
		}
	}

	return results, nil
}

func (s *GridSpatialIndex) coveredCells(b BBox) []cellKey {
	minX := int64(math.Floor(b.MinX / s.cellSize))
	maxX := int64(math.Floor(b.MaxX / s.cellSize))
	minY := int64(math.Floor(b.MinY / s.cellSize))
	maxY := int64(math.Floor(b.MaxY / s.cellSize))

	keys := make([]cellKey, 0, (maxX-minX+1)*(maxY-minY+1))
	for x := minX; x <= maxX; x++ {
		for y := minY; y <= maxY; y++ {
			keys = append(keys, cellKey{X: x, Y: y})
		}
	}

	return keys
}
