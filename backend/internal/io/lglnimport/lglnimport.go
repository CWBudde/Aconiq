// Package lglnimport finds and downloads the LoD2 CityGML tiles the LGLN
// (Landesamt für Geoinformation und Landesvermessung Niedersachsen) publishes
// as open data, for a WGS84 bounding box.
//
// Search asks the LGLN STAC API which 1 km tiles intersect the box; FetchTile
// streams one tile into a disk cache. Parsing the CityGML is not this
// package's job — the caller hands the cached file to citygmlimport.
//
// Every URL the package requests — the STAC search, its next links, the tile
// assets and any redirect — must be https and on an allowlisted host. The STAC
// response names the asset URLs, so without the allowlist a search result
// could point the downloader anywhere.
package lglnimport

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/aconiq/backend/internal/buildinfo"
)

const (
	// DefaultSTACURL is the LGLN STAC API root.
	DefaultSTACURL = "https://lod.stac.lgln.niedersachsen.de"
	// stacHost and tileHost are the hosts NewClient allows by default: the
	// STAC API and the object store its lod2-gml assets live on.
	stacHost = "lod.stac.lgln.niedersachsen.de"
	tileHost = "lod2.s3.eu-de.cloud-object-storage.appdomain.cloud"

	// collectionID is the STAC collection holding the LoD2 tiles.
	collectionID = "lod2"
	// gmlAssetKey names the CityGML asset of a lod2 item; its sibling
	// lod2-shp is the ESRI shape export and is never fetched.
	gmlAssetKey = "lod2-gml"

	// MaxTiles is the most tiles one Search may return. A city-centre tile
	// is around 50 MB of CityGML, so nine of them (a 3 km × 3 km block) is
	// already a large import.
	MaxTiles = 9
	// MaxTileBytes caps one tile download. The largest tiles are a quarter
	// of it; a response past it is not a tile.
	MaxTileBytes = 200 << 20

	// searchPageLimit is the page size asked of the STAC search. It exceeds
	// MaxTiles, so a search that is going to succeed needs one page.
	searchPageLimit = 100
	// maxSearchPages bounds how many next links Search follows.
	maxSearchPages = 20
	// maxSearchPageBytes caps the body of one STAC search page.
	maxSearchPageBytes = 16 << 20

	searchTimeout = 30 * time.Second
	fetchTimeout  = 10 * time.Minute

	contactURL = "https://github.com/CWBudde/Aconiq"
)

var (
	// ErrInvalidBBox reports a bounding box that is not finite, not ordered
	// (west < east, south < north) or not within WGS84 range.
	ErrInvalidBBox = errors.New("invalid bounding box")
	// ErrOutsideCoverage reports a bounding box that does not touch Lower
	// Saxony's extent at all, so no LGLN tile can intersect it.
	ErrOutsideCoverage = errors.New("bounding box is outside the LGLN LoD2 coverage")
	// ErrTooManyTiles reports a search that intersects more than MaxTiles
	// tiles. The returned error is a *TooManyTilesError.
	ErrTooManyTiles = errors.New("too many LGLN tiles")
	// ErrUnavailable reports that the LGLN service could not be reached or
	// answered with something other than a usable response.
	ErrUnavailable = errors.New("LGLN service unavailable")
	// ErrHostNotAllowed reports a URL — the STAC base, a next link, a tile
	// asset or a redirect — that is not https on an allowlisted host.
	ErrHostNotAllowed = errors.New("host not allowed")
	// ErrTileTooLarge reports a tile download that exceeded MaxTileBytes.
	ErrTileTooLarge = errors.New("LGLN tile exceeds the size limit")
	// ErrInvalidTile reports a Tile whose ID or date cannot name a cache
	// file — one that did not come from Search.
	ErrInvalidTile = errors.New("invalid LGLN tile")
)

// TooManyTilesError is the error Search returns when the box intersects more
// than Max tiles. Found is a lower bound: Search stops paging once it is past
// the limit. It matches ErrTooManyTiles under errors.Is.
type TooManyTilesError struct {
	Found, Max int
}

func (e *TooManyTilesError) Error() string {
	return fmt.Sprintf("bounding box intersects at least %d LGLN tiles, more than the limit of %d", e.Found, e.Max)
}

// Is makes errors.Is(err, ErrTooManyTiles) hold.
func (e *TooManyTilesError) Is(target error) bool {
	return target == ErrTooManyTiles
}

// coverage is the lod2 collection's published spatial extent, rounded
// outward. It is a sanity bound only: a box inside it may still hit no tile.
var coverage = BBox{West: 6.6, South: 51.3, East: 11.6, North: 53.9}

// BBox is a geographic bounding box in WGS84 degrees.
type BBox struct {
	West, South, East, North float64
}

// Validate checks that the box is finite, ordered, within WGS84 range and
// touches the LGLN coverage.
func (b BBox) Validate() error {
	for _, v := range []float64{b.West, b.South, b.East, b.North} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("%w: coordinates must be finite", ErrInvalidBBox)
		}
	}

	if b.West >= b.East {
		return fmt.Errorf("%w: west must be less than east", ErrInvalidBBox)
	}

	if b.South >= b.North {
		return fmt.Errorf("%w: south must be less than north", ErrInvalidBBox)
	}

	if b.South < -90 || b.North > 90 || b.West < -180 || b.East > 180 {
		return fmt.Errorf("%w: coordinates out of WGS84 range (lat: -90..90, lon: -180..180)", ErrInvalidBBox)
	}

	if b.East < coverage.West || b.West > coverage.East || b.North < coverage.South || b.South > coverage.North {
		return ErrOutsideCoverage
	}

	return nil
}

// Tile is one 1 km LoD2 tile in EPSG:25832, as the STAC search lists it.
type Tile struct {
	// ID is the STAC item ID, LoD2_32_<eastingKm>_<northingKm>_1_ni.
	ID string
	// EastingKM and NorthingKM are the tile's lower-left corner in km.
	EastingKM, NorthingKM int
	// Updated is the tile's last change as YYYY-MM-DD: letzte_aenderung,
	// or the date part of datetime when that is absent.
	Updated string
	// GMLURL is the CityGML asset's URL.
	GMLURL string
}

// Client talks to the LGLN STAC API and its tile store.
type Client struct {
	stacURL      string
	http         *http.Client
	allowedHosts []string
	maxTileBytes int64
}

// Option configures a Client.
type Option func(*Client)

// WithSTACURL points the client at a different STAC root. Its host must be
// allowlisted as well.
func WithSTACURL(u string) Option {
	return func(c *Client) { c.stacURL = strings.TrimRight(u, "/") }
}

// WithHTTPClient sets the HTTP client. The client is copied, and the copy's
// CheckRedirect replaced so that a redirect cannot leave the allowlist.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.http = hc
		}
	}
}

// WithAllowedHosts replaces the host allowlist. An entry is compared,
// case-insensitively, against a URL's host including any port.
func WithAllowedHosts(hosts ...string) Option {
	return func(c *Client) { c.allowedHosts = slices.Clone(hosts) }
}

// NewClient returns a client for the public LGLN service, allowing the STAC
// host and the tile store's host.
func NewClient(opts ...Option) *Client {
	c := &Client{
		stacURL:      DefaultSTACURL,
		http:         defaultHTTPClient(),
		allowedHosts: []string{stacHost, tileHost},
		maxTileBytes: MaxTileBytes,
	}

	for _, opt := range opts {
		opt(c)
	}

	guarded := *c.http
	guarded.CheckRedirect = c.checkRedirect
	c.http = &guarded

	return c
}

// defaultHTTPClient bounds the connection phases but not the body: a tile
// takes as long as it takes, and the caller's context and fetchTimeout
// bound the whole transfer.
func defaultHTTPClient() *http.Client {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Client{}
	}

	t := transport.Clone()
	t.TLSHandshakeTimeout = 15 * time.Second
	t.ResponseHeaderTimeout = 30 * time.Second

	return &http.Client{Transport: t}
}

const maxRedirects = 10

func (c *Client) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("%w: stopped after %d redirects", ErrUnavailable, maxRedirects)
	}

	return c.checkURL(req.URL)
}

// allowedURL parses raw and checks it against the allowlist.
func (c *Client) allowedURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %q is not a URL", ErrHostNotAllowed, raw)
	}

	if err := c.checkURL(u); err != nil {
		return nil, err
	}

	return u, nil
}

func (c *Client) checkURL(u *url.URL) error {
	if u.Scheme != "https" || u.Host == "" || u.User != nil {
		return fmt.Errorf("%w: %q must be an absolute https URL", ErrHostNotAllowed, u.Redacted())
	}

	if !slices.ContainsFunc(c.allowedHosts, func(h string) bool { return strings.EqualFold(h, u.Host) }) {
		return fmt.Errorf("%w: %q is not an allowed LGLN host", ErrHostNotAllowed, u.Host)
	}

	return nil
}

// userAgent names the caller on every request, as osmimport does.
func userAgent() string {
	return buildinfo.Name + "/" + buildinfo.Version() + " (+" + contactURL + ")"
}

// Attribution returns the source note the LGLN open-data licence (CC BY 4.0)
// requires: the rights holder and a year in parentheses. The year is the
// latest Updated year among tiles. It returns "" for no tiles.
func Attribution(tiles []Tile) string {
	year := ""

	for _, t := range tiles {
		if len(t.Updated) >= 4 && t.Updated[:4] > year {
			year = t.Updated[:4]
		}
	}

	if year == "" {
		return ""
	}

	return fmt.Sprintf(AttributionFormat, year)
}

const (
	// AttributionFormat is the source note, with the year as its only verb.
	// The LGLN metadata asks for "LGLN" as rights holder and provider,
	// followed by the year in parentheses, under CC BY 4.0.
	AttributionFormat = "Quelle: LGLN (%s), " + LicenseName
	// LicenseName and LicenseURL name the licence the LGLN publishes its
	// open geodata under.
	LicenseName = "CC BY 4.0"
	LicenseURL  = "https://creativecommons.org/licenses/by/4.0/"
)
