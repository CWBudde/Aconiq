package lglnimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/aconiq/backend/internal/buildinfo"
)

var hannover = BBox{West: 9.735, South: 52.372, East: 9.745, North: 52.378}

// fixture serves STAC search pages and tile bodies from two TLS servers, so
// the asset host and the STAC host differ as they do in production.
type fixture struct {
	stac, tiles *httptest.Server
	tileHits    atomic.Int32
}

func newFixture(t *testing.T, stac http.HandlerFunc, tiles http.HandlerFunc) *fixture {
	t.Helper()

	f := &fixture{}
	f.stac = httptest.NewTLSServer(stac)
	t.Cleanup(f.stac.Close)

	f.tiles = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.tileHits.Add(1)
		tiles(w, r)
	}))
	t.Cleanup(f.tiles.Close)

	return f
}

// client trusts both servers: httptest signs every TLS server with the same
// test certificate.
func (f *fixture) client() *Client {
	return NewClient(
		WithSTACURL(f.stac.URL),
		WithHTTPClient(f.stac.Client()),
		WithAllowedHosts(host(f.stac.URL), host(f.tiles.URL)),
	)
}

func host(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}

	return u.Host
}

func item(id, date, href string) map[string]any {
	return map[string]any{
		"id":         id,
		"type":       "Feature",
		"properties": map[string]any{"datetime": date + "T00:00:00Z", "letzte_aenderung": date},
		"assets": map[string]any{
			"lod2-gml": map[string]any{"href": href, "type": "text/xml"},
			"lod2-shp": map[string]any{"href": href + ".zip"},
		},
	}
}

func writePage(t *testing.T, w http.ResponseWriter, features []map[string]any, next string) {
	t.Helper()

	links := []map[string]any{{"rel": "self", "href": "https://example.invalid/search"}}
	if next != "" {
		links = append(links, map[string]any{"rel": "next", "href": next})
	}

	w.Header().Set("Content-Type", "application/geo+json")

	if err := json.NewEncoder(w).Encode(map[string]any{
		"type":     "FeatureCollection",
		"features": features,
		"links":    links,
	}); err != nil {
		t.Errorf("encode page: %v", err)
	}
}

func notCalled(t *testing.T) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, _ *http.Request) {
		t.Error("tile server called unexpectedly")
		w.WriteHeader(http.StatusTeapot)
	}
}

func TestSearchFollowsNextLinksDedupsAndOrders(t *testing.T) {
	var f *fixture

	f = newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/search" || q.Get("collections") != "lod2" || q.Get("bbox") != "9.735,52.372,9.745,52.378" {
			t.Errorf("unexpected search request %s", r.URL)
		}

		if ua := r.Header.Get("User-Agent"); !strings.HasPrefix(ua, buildinfo.Name+"/") {
			t.Errorf("User-Agent = %q, want the %s agent", ua, buildinfo.Name)
		}

		tileURL := func(id string) string { return f.tiles.URL + "/" + id + ".gml" }

		switch q.Get("token") {
		case "":
			writePage(t, w, []map[string]any{
				item("LoD2_32_551_5803_1_ni", "2024-06-12", tileURL("LoD2_32_551_5803_1_ni")),
				item("LoD2_32_550_5803_1_ni", "2024-06-01", tileURL("LoD2_32_550_5803_1_ni")),
			}, f.stac.URL+"/search?"+q.Encode()+"&token=next%3Alod2%3Ap2")
		case "next:lod2:p2":
			writePage(t, w, []map[string]any{
				item("LoD2_32_550_5802_1_ni", "2023-01-02", tileURL("LoD2_32_550_5802_1_ni")),
				item("LoD2_32_550_5803_1_ni", "2024-06-12", tileURL("LoD2_32_550_5803_1_ni")),
			}, "")
		default:
			t.Errorf("unexpected token %q", q.Get("token"))
		}
	}, notCalled(t))

	tiles, err := f.client().Search(context.Background(), hannover)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	got := make([]string, 0, len(tiles))
	for _, tile := range tiles {
		got = append(got, fmt.Sprintf("%s@%s(%d,%d)", tile.ID, tile.Updated, tile.EastingKM, tile.NorthingKM))
	}

	want := []string{
		"LoD2_32_550_5802_1_ni@2023-01-02(550,5802)",
		"LoD2_32_550_5803_1_ni@2024-06-12(550,5803)",
		"LoD2_32_551_5803_1_ni@2024-06-12(551,5803)",
	}

	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("tiles = %v, want %v", got, want)
	}

	if tiles[0].GMLURL != f.tiles.URL+"/LoD2_32_550_5802_1_ni.gml" {
		t.Errorf("GMLURL = %q", tiles[0].GMLURL)
	}
}

func TestSearchEmptyResult(t *testing.T) {
	f := newFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		writePage(t, w, nil, "")
	}, notCalled(t))

	tiles, err := f.client().Search(context.Background(), hannover)
	if err != nil || len(tiles) != 0 {
		t.Fatalf("Search = %v, %v; want no tiles and no error", tiles, err)
	}
}

func TestSearchRefusals(t *testing.T) {
	cases := []struct {
		name    string
		page    func(f *fixture) ([]map[string]any, string)
		status  int
		raw     string
		wantErr error
	}{
		{
			name: "asset host off the allowlist",
			page: func(*fixture) ([]map[string]any, string) {
				return []map[string]any{item("LoD2_32_550_5803_1_ni", "2024-06-12", "https://evil.example/x.gml")}, ""
			},
			wantErr: ErrHostNotAllowed,
		},
		{
			name: "asset over plain http",
			page: func(f *fixture) ([]map[string]any, string) {
				return []map[string]any{item("LoD2_32_550_5803_1_ni", "2024-06-12", "http://"+host(f.tiles.URL)+"/x.gml")}, ""
			},
			wantErr: ErrHostNotAllowed,
		},
		{
			name: "next link off the allowlist",
			page: func(*fixture) ([]map[string]any, string) {
				return nil, "https://evil.example/search?token=x"
			},
			wantErr: ErrHostNotAllowed,
		},
		{
			name: "next link repeats the page",
			page: func(f *fixture) ([]map[string]any, string) {
				return nil, f.stac.URL + "/search?bbox=9.735%2C52.372%2C9.745%2C52.378&collections=lod2&limit=100"
			},
			wantErr: ErrUnavailable,
		},
		{
			name: "too many tiles",
			page: func(f *fixture) ([]map[string]any, string) {
				features := make([]map[string]any, 0, MaxTiles+1)

				for i := range MaxTiles + 1 {
					id := fmt.Sprintf("LoD2_32_%d_5803_1_ni", 540+i)
					features = append(features, item(id, "2024-06-12", f.tiles.URL+"/"+id+".gml"))
				}

				return features, ""
			},
			wantErr: ErrTooManyTiles,
		},
		{
			name: "unrecognised item id",
			page: func(f *fixture) ([]map[string]any, string) {
				return []map[string]any{item("LoD2_32_../x_5803_1_ni", "2024-06-12", f.tiles.URL+"/x.gml")}, ""
			},
			wantErr: ErrUnavailable,
		},
		{
			name: "item without a date",
			page: func(f *fixture) ([]map[string]any, string) {
				it := item("LoD2_32_550_5803_1_ni", "", f.tiles.URL+"/x.gml")
				it["properties"] = map[string]any{}

				return []map[string]any{it}, ""
			},
			wantErr: ErrUnavailable,
		},
		{
			name: "item without a CityGML asset",
			page: func(f *fixture) ([]map[string]any, string) {
				it := item("LoD2_32_550_5803_1_ni", "2024-06-12", f.tiles.URL+"/x.gml")
				it["assets"] = map[string]any{}

				return []map[string]any{it}, ""
			},
			wantErr: ErrUnavailable,
		},
		{name: "server error", status: http.StatusBadGateway, wantErr: ErrUnavailable},
		{name: "not JSON", raw: "<html>", wantErr: ErrUnavailable},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var f *fixture

			f = newFixture(t, func(w http.ResponseWriter, _ *http.Request) {
				switch {
				case tc.status != 0:
					w.WriteHeader(tc.status)
				case tc.raw != "":
					_, _ = w.Write([]byte(tc.raw))
				default:
					features, next := tc.page(f)
					writePage(t, w, features, next)
				}
			}, notCalled(t))

			tiles, err := f.client().Search(context.Background(), hannover)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Search = %v, %v; want %v", tiles, err, tc.wantErr)
			}
		})
	}
}

func TestSearchTooManyTilesCarriesCount(t *testing.T) {
	var f *fixture

	f = newFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		features := make([]map[string]any, 0, 12)

		for i := range 12 {
			id := fmt.Sprintf("LoD2_32_550_%d_1_ni", 5800+i)
			features = append(features, item(id, "2024-06-12", f.tiles.URL+"/"+id+".gml"))
		}

		writePage(t, w, features, f.stac.URL+"/search?token=never-followed")
	}, notCalled(t))

	_, err := f.client().Search(context.Background(), hannover)

	var tooMany *TooManyTilesError
	if !errors.As(err, &tooMany) || tooMany.Found != 12 || tooMany.Max != MaxTiles {
		t.Fatalf("err = %v, want *TooManyTilesError{12, %d}", err, MaxTiles)
	}
}

func TestSearchRefusesStacURLOffAllowlist(t *testing.T) {
	c := NewClient(WithSTACURL("https://evil.example"))

	if _, err := c.Search(context.Background(), hannover); !errors.Is(err, ErrHostNotAllowed) {
		t.Fatalf("err = %v, want ErrHostNotAllowed", err)
	}
}

func TestSearchCancelled(t *testing.T) {
	release := make(chan struct{})

	t.Cleanup(func() { close(release) })

	f := newFixture(t, func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}, notCalled(t))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := f.client().Search(ctx, hannover)
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want context.Canceled and not ErrUnavailable", err)
	}
}

func TestBBoxValidate(t *testing.T) {
	cases := []struct {
		name string
		bb   BBox
		want error
	}{
		{"valid", hannover, nil},
		{"NaN", BBox{West: math.NaN(), South: 52, East: 10, North: 53}, ErrInvalidBBox},
		{"Inf", BBox{West: 9, South: 52, East: math.Inf(1), North: 53}, ErrInvalidBBox},
		{"west not less than east", BBox{West: 10, South: 52, East: 10, North: 53}, ErrInvalidBBox},
		{"south not less than north", BBox{West: 9, South: 53, East: 10, North: 52}, ErrInvalidBBox},
		{"out of WGS84 range", BBox{West: 9, South: 52, East: 10, North: 95}, ErrInvalidBBox},
		{"Munich", BBox{West: 11.5, South: 48.1, East: 11.6, North: 48.2}, ErrOutsideCoverage},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.bb.Validate()
			if !errors.Is(err, tc.want) || (tc.want == nil && err != nil) {
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}

			if tc.want != nil {
				if _, err := NewClient().Search(context.Background(), tc.bb); !errors.Is(err, tc.want) {
					t.Fatalf("Search() = %v, want %v", err, tc.want)
				}
			}
		})
	}
}

func TestFetchTileCachesAndReplacesStaleDates(t *testing.T) {
	body := "<CityModel/>"

	f := newFixture(t, func(http.ResponseWriter, *http.Request) {}, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/LoD2_32_550_5803_1_ni.gml" {
			t.Errorf("tile path = %s", r.URL.Path)
		}

		_, _ = w.Write([]byte(body))
	})

	dir := filepath.Join(t.TempDir(), "cache")
	c := f.client()
	tile := Tile{ID: "LoD2_32_550_5803_1_ni", EastingKM: 550, NorthingKM: 5803, Updated: "2024-06-12", GMLURL: f.tiles.URL + "/LoD2_32_550_5803_1_ni.gml"}

	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}

	stale := filepath.Join(dir, "LoD2_32_550_5803_1_ni_2020-01-01.gml")
	neighbour := filepath.Join(dir, "LoD2_32_551_5803_1_ni_2020-01-01.gml")

	for _, p := range []string{stale, neighbour} {
		if err := os.WriteFile(p, []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	path, cached, err := c.FetchTile(context.Background(), tile, dir)
	if err != nil || cached {
		t.Fatalf("first FetchTile = %q, %v, %v", path, cached, err)
	}

	if want := filepath.Join(dir, "LoD2_32_550_5803_1_ni_2024-06-12.gml"); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}

	if got, _ := os.ReadFile(path); string(got) != body {
		t.Fatalf("content = %q, want %q", got, body)
	}

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale date of the same tile survived: %v", err)
	}

	if _, err := os.Stat(neighbour); err != nil {
		t.Errorf("another tile's cache file was removed: %v", err)
	}

	path2, cached, err := c.FetchTile(context.Background(), tile, dir)
	if err != nil || !cached || path2 != path {
		t.Fatalf("second FetchTile = %q, %v, %v; want cached %q", path2, cached, err, path)
	}

	if hits := f.tileHits.Load(); hits != 1 {
		t.Fatalf("tile server hits = %d, want 1", hits)
	}
}

func TestFetchTileFailuresLeaveNoFile(t *testing.T) {
	cases := []struct {
		name    string
		handler http.HandlerFunc
		want    error
	}{
		{
			name: "streamed body past the cap",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				for range 4 {
					_, _ = w.Write([]byte(strings.Repeat("x", 40)))
					w.(http.Flusher).Flush()
				}
			},
			want: ErrTileTooLarge,
		},
		{
			name: "announced length past the cap",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Length", "1000")
				_, _ = w.Write([]byte(strings.Repeat("x", 1000)))
			},
			want: ErrTileTooLarge,
		},
		{
			name:    "not found",
			handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) },
			want:    ErrUnavailable,
		},
		{
			name:    "empty body",
			handler: func(http.ResponseWriter, *http.Request) {},
			want:    ErrUnavailable,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, func(http.ResponseWriter, *http.Request) {}, tc.handler)
			c := f.client()
			c.maxTileBytes = 100

			dir := t.TempDir()
			tile := Tile{ID: "LoD2_32_550_5803_1_ni", Updated: "2024-06-12", GMLURL: f.tiles.URL + "/t.gml"}

			path, _, err := c.FetchTile(context.Background(), tile, dir)
			if !errors.Is(err, tc.want) {
				t.Fatalf("FetchTile = %q, %v; want %v", path, err, tc.want)
			}

			assertEmptyDir(t, dir)
		})
	}
}

func TestFetchTileCancelledMidBody(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})

	// The handler holds the body open until the client has cancelled, then
	// aborts the connection. Returning normally would end the chunked body
	// cleanly, and a client that read that end before noticing its own
	// cancellation would store a truncated tile.
	f := newFixture(t, func(http.ResponseWriter, *http.Request) {}, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<CityModel>"))
		w.(http.Flusher).Flush()
		close(started)

		select {
		case <-cancelled:
		case <-r.Context().Done():
		}

		panic(http.ErrAbortHandler)
	})

	ctx, cancel := context.WithCancel(context.Background())
	dir := t.TempDir()

	go func() {
		<-started
		cancel()
		close(cancelled)
	}()

	tile := Tile{ID: "LoD2_32_550_5803_1_ni", Updated: "2024-06-12", GMLURL: f.tiles.URL + "/t.gml"}

	_, _, err := f.client().FetchTile(ctx, tile, dir)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}

	assertEmptyDir(t, dir)
}

func TestFetchTileRefusesRedirectOffAllowlist(t *testing.T) {
	f := newFixture(t, func(http.ResponseWriter, *http.Request) {}, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://evil.example/t.gml", http.StatusFound)
	})

	dir := t.TempDir()
	tile := Tile{ID: "LoD2_32_550_5803_1_ni", Updated: "2024-06-12", GMLURL: f.tiles.URL + "/t.gml"}

	_, _, err := f.client().FetchTile(context.Background(), tile, dir)
	if !errors.Is(err, ErrHostNotAllowed) {
		t.Fatalf("err = %v, want ErrHostNotAllowed", err)
	}

	assertEmptyDir(t, dir)
}

func TestFetchTileRefusesInvalidTiles(t *testing.T) {
	good := Tile{ID: "LoD2_32_550_5803_1_ni", Updated: "2024-06-12", GMLURL: "https://" + tileHost + "/LoD2_32_550_5803_1_ni.gml"}

	cases := []struct {
		name   string
		mutate func(*Tile)
		want   error
	}{
		{"path in id", func(t *Tile) { t.ID = "../LoD2_32_550_5803_1_ni" }, ErrInvalidTile},
		{"bad date", func(t *Tile) { t.Updated = "2024-6-12" }, ErrInvalidTile},
		{"date with path", func(t *Tile) { t.Updated = "../x" }, ErrInvalidTile},
		{"asset host", func(t *Tile) { t.GMLURL = "https://evil.example/x.gml" }, ErrHostNotAllowed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tile := good
			tc.mutate(&tile)

			dir := t.TempDir()
			if _, _, err := NewClient().FetchTile(context.Background(), tile, dir); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}

			assertEmptyDir(t, dir)
		})
	}
}

func TestParseTileID(t *testing.T) {
	cases := []struct {
		id   string
		e, n int
		ok   bool
	}{
		{"LoD2_32_551_5803_1_ni", 551, 5803, true},
		{"LoD2_32_551_5803_1_ni.gml", 0, 0, false},
		{"LoD1_32_551_5803_1_ni", 0, 0, false},
		{"LoD2_33_551_5803_1_ni", 0, 0, false},
		{"LoD2_32_0551_5803_1_ni", 0, 0, false},
		{"LoD2_32_-1_5803_1_ni", 0, 0, false},
		{"LoD2_32_x_5803_1_ni", 0, 0, false},
		{"LoD2_32_551_5803_2_ni", 0, 0, false},
		{"LoD2_32_551_5803_1_nw", 0, 0, false},
		{"", 0, 0, false},
	}

	for _, tc := range cases {
		e, n, ok := parseTileID(tc.id)
		if e != tc.e || n != tc.n || ok != tc.ok {
			t.Errorf("parseTileID(%q) = %d, %d, %v; want %d, %d, %v", tc.id, e, n, ok, tc.e, tc.n, tc.ok)
		}
	}
}

func TestItemDate(t *testing.T) {
	cases := []struct {
		props stacProperties
		want  string
		ok    bool
	}{
		{stacProperties{LetzteAenderung: "2024-06-12", DateTime: "2020-01-01T00:00:00Z"}, "2024-06-12", true},
		{stacProperties{DateTime: "2024-06-12T23:30:00+02:00"}, "2024-06-12", true},
		{stacProperties{LetzteAenderung: "12.06.2024", DateTime: "2024-06-11T00:00:00Z"}, "2024-06-11", true},
		{stacProperties{DateTime: "2024"}, "", false},
		{stacProperties{}, "", false},
	}

	for _, tc := range cases {
		got, ok := itemDate(tc.props)
		if got != tc.want || ok != tc.ok {
			t.Errorf("itemDate(%+v) = %q, %v; want %q, %v", tc.props, got, ok, tc.want, tc.ok)
		}
	}
}

func TestAttribution(t *testing.T) {
	if got := Attribution(nil); got != "" {
		t.Errorf("Attribution(nil) = %q, want empty", got)
	}

	got := Attribution([]Tile{{Updated: "2023-01-02"}, {Updated: "2024-06-12"}, {Updated: "2019-12-31"}})
	if want := "Quelle: LGLN (2024), CC BY 4.0"; got != want {
		t.Errorf("Attribution = %q, want %q", got, want)
	}
}

func TestNewClientDefaults(t *testing.T) {
	c := NewClient()

	u, err := c.searchURL(hannover)
	if err != nil {
		t.Fatalf("searchURL: %v", err)
	}

	want := "https://lod.stac.lgln.niedersachsen.de/search?bbox=9.735%2C52.372%2C9.745%2C52.378&collections=lod2&limit=100"
	if u != want {
		t.Errorf("searchURL = %q, want %q", u, want)
	}

	if _, err := c.allowedURL("https://" + tileHost + "/LoD2_32_550_5803_1_ni.gml"); err != nil {
		t.Errorf("tile host not allowed by default: %v", err)
	}

	if _, err := c.allowedURL("https://user@" + tileHost + "/x"); !errors.Is(err, ErrHostNotAllowed) {
		t.Errorf("URL with userinfo allowed: %v", err)
	}
}

func assertEmptyDir(t *testing.T, dir string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}

		t.Fatalf("cache dir holds %v, want nothing", names)
	}
}
