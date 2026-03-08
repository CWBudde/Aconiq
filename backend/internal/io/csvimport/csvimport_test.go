package csvimport

import (
	"strings"
	"testing"
)

func TestReadTable_HappyPath(t *testing.T) {
	input := "feature_id,name,count\nf1,road A,42\nf2,road B,7\n"

	records, err := ReadTable(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	if records[0].FeatureID != "f1" {
		t.Errorf("expected feature_id 'f1', got %q", records[0].FeatureID)
	}

	if records[0].Properties["name"] != "road A" {
		t.Errorf("expected name 'road A', got %v", records[0].Properties["name"])
	}

	if records[0].Properties["count"] != float64(42) {
		t.Errorf("expected count 42.0, got %v", records[0].Properties["count"])
	}
}

func TestReadTable_MissingFeatureIDColumn(t *testing.T) {
	input := "id,name\nf1,road A\n"

	_, err := ReadTable(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for missing feature_id column, got nil")
	}
}

func TestReadTable_SkipsBlankFeatureID(t *testing.T) {
	input := "feature_id,name\nf1,road A\n,road B\n   ,road C\nf4,road D\n"

	records, err := ReadTable(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 records (blank fid rows skipped), got %d", len(records))
	}
}

func TestReadTable_TypeInference(t *testing.T) {
	input := "feature_id,num,flag,label\nf1,3.14,true,hello\n"

	records, err := ReadTable(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	props := records[0].Properties

	if props["num"] != float64(3.14) {
		t.Errorf("expected num=3.14, got %v (%T)", props["num"], props["num"])
	}

	if props["flag"] != true {
		t.Errorf("expected flag=true, got %v (%T)", props["flag"], props["flag"])
	}

	if props["label"] != "hello" {
		t.Errorf("expected label='hello', got %v (%T)", props["label"], props["label"])
	}
}

func TestReadTable_EmptyReader(t *testing.T) {
	// Header only → empty slice, no error.
	input := "feature_id,name\n"

	records, err := ReadTable(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) != 0 {
		t.Errorf("expected 0 records for header-only CSV, got %d", len(records))
	}
}

func TestReadTable_CaseInsensitiveFeatureIDHeader(t *testing.T) {
	input := "Feature_ID,name\nf1,road\n"

	records, err := ReadTable(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) != 1 || records[0].FeatureID != "f1" {
		t.Errorf("expected 1 record with FeatureID 'f1', got %+v", records)
	}
}
