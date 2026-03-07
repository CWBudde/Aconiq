package geo

import "testing"

func TestGridSpatialIndexQuery(t *testing.T) {
	t.Parallel()

	index, err := NewGridSpatialIndex(10)
	if err != nil {
		t.Fatalf("create index: %v", err)

		err := index.Insert(IndexedItem{ID: "a", BBox: BBox{MinX: 0, MinY: 0, MaxX: 5, MaxY: 5}})
		if err != nil {
			t.Fatalf("insert a: %v", err)
		}

		err = index.Insert(IndexedItem{ID: "b", BBox: BBox{MinX: 20, MinY: 20, MaxX: 25, MaxY: 25}})
		if err != nil {
			t.Fatalf("insert b: %v", err)
		}
	}

	items, err := index.Query(BBox{MinX: -1, MinY: -1, MaxX: 6, MaxY: 6})
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	if len(items) != 1 || items[0].ID != "a" {
		t.Fatalf("unexpected query results %#v", items)
	}
}
