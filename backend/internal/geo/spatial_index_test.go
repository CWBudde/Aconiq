package geo

import "testing"

func TestGridSpatialIndexQuery(t *testing.T) {
	t.Parallel()

	index, err := NewGridSpatialIndex(10)
	if err != nil {
		t.Fatalf("create index: %v", err)
	}

	err = index.Insert(IndexedItem{ID: "a", BBox: BBox{MinX: 0, MinY: 0, MaxX: 5, MaxY: 5}})
	if err != nil {
		t.Fatalf("insert a: %v", err)
	}

	err = index.Insert(IndexedItem{ID: "b", BBox: BBox{MinX: 20, MinY: 20, MaxX: 25, MaxY: 25}})
	if err != nil {
		t.Fatalf("insert b: %v", err)
	}

	items, err := index.Query(BBox{MinX: -1, MinY: -1, MaxX: 6, MaxY: 6})
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	if len(items) != 1 || items[0].ID != "a" {
		t.Fatalf("unexpected query results %#v", items)
	}
}

func TestGridSpatialIndexValidationAndHelpers(t *testing.T) {
	t.Parallel()

	_, err := NewGridSpatialIndex(0)
	if err == nil {
		t.Fatal("expected invalid cell size error")
	}

	var nilIndex *GridSpatialIndex
	if got := nilIndex.Len(); got != 0 {
		t.Fatalf("expected nil index len 0, got %d", got)
	}

	err = nilIndex.Insert(IndexedItem{ID: "a", BBox: BBox{MinX: 0, MinY: 0, MaxX: 1, MaxY: 1}})
	if err == nil {
		t.Fatal("expected nil index insert error")
	}

	_, err = nilIndex.Query(BBox{MinX: 0, MinY: 0, MaxX: 1, MaxY: 1})
	if err == nil {
		t.Fatal("expected nil index query error")
	}

	index, err := NewGridSpatialIndex(10)
	if err != nil {
		t.Fatalf("create index: %v", err)
	}

	if index.Len() != 0 {
		t.Fatalf("expected empty index, got %d", index.Len())
	}

	err = index.Insert(IndexedItem{})
	if err == nil {
		t.Fatal("expected missing id error")
	}

	err = index.Insert(IndexedItem{ID: "bad", BBox: BBox{MinX: 2, MinY: 0, MaxX: 1, MaxY: 1}})
	if err == nil {
		t.Fatal("expected invalid bbox error")
	}

	item := IndexedItem{ID: "a", BBox: BBox{MinX: 0, MinY: 0, MaxX: 15, MaxY: 15}}

	err = index.Insert(item)
	if err != nil {
		t.Fatalf("insert item: %v", err)
	}

	if index.Len() != 1 {
		t.Fatalf("expected len 1, got %d", index.Len())
	}

	err = index.Insert(item)
	if err == nil {
		t.Fatal("expected duplicate insert error")
	}

	results, err := index.Query(BBox{MinX: 9, MinY: 9, MaxX: 11, MaxY: 11})
	if err != nil {
		t.Fatalf("query spanning duplicate buckets: %v", err)
	}

	if len(results) != 1 || results[0].ID != "a" {
		t.Fatalf("unexpected deduplicated results %#v", results)
	}

	_, err = index.Query(BBox{MinX: 3, MinY: 0, MaxX: 2, MaxY: 1})
	if err == nil {
		t.Fatal("expected invalid query bbox error")
	}
}
