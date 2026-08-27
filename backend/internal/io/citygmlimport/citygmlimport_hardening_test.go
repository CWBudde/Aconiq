package citygmlimport

import (
	"strings"
	"testing"
)

// Go's xml.Decoder is safe against XXE and entity expansion but has no depth
// limit: it keeps one stack entry per open element, so a document that is
// nothing but opening tags is a compact way to make the parser allocate.
func TestReadWithCRS_RejectsDeeplyNestedDocument(t *testing.T) {
	var b strings.Builder

	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)

	const depth = maxXMLNestingDepth + 50

	for range depth {
		b.WriteString("<a>")
	}

	for range depth {
		b.WriteString("</a>")
	}

	doc := b.String()
	if len(doc) > 4096 {
		t.Fatalf("proof-of-concept document grew to %d bytes", len(doc))
	}

	_, err := ReadWithCRS([]byte(doc))
	if err == nil {
		t.Fatal("expected an error for a document nested past the depth limit")
	}

	if !strings.Contains(err.Error(), "nesting") {
		t.Fatalf("error %q does not report the nesting depth", err)
	}
}

// A document nested within the limit must reach the CityGML parser unchanged.
func TestCheckXMLNestingDepth_AcceptsRealisticNesting(t *testing.T) {
	var b strings.Builder

	const depth = 24

	for range depth {
		b.WriteString("<a>")
	}

	for range depth {
		b.WriteString("</a>")
	}

	err := checkXMLNestingDepth([]byte(b.String()))
	if err != nil {
		t.Fatalf("depth %d rejected: %v", depth, err)
	}
}

// Malformed XML is the CityGML parser's business to report, not the depth
// pre-scan's.
func TestCheckXMLNestingDepth_IgnoresMalformedXML(t *testing.T) {
	err := checkXMLNestingDepth([]byte("<a><b></a>"))
	if err != nil {
		t.Fatalf("pre-scan reported a parse error it should have left alone: %v", err)
	}
}

func FuzzReadWithCRS(f *testing.F) {
	f.Add([]byte(`<?xml version="1.0"?><core:CityModel xmlns:core="http://www.opengis.net/citygml/2.0"></core:CityModel>`))
	f.Add([]byte(strings.Repeat("<a>", 400)))
	f.Add([]byte("<a><b></a>"))
	f.Add([]byte(""))

	f.Fuzz(func(_ *testing.T, data []byte) {
		// Any error is acceptable; panics and runaway allocations are not.
		_, _ = ReadWithCRS(data)
	})
}
