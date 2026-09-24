package lglnimport

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// stacPage is the part of a STAC search response Search reads.
type stacPage struct {
	Features []stacItem `json:"features"`
	Links    []stacLink `json:"links"`
}

type stacLink struct {
	Rel  string `json:"rel"`
	Href string `json:"href"`
}

type stacItem struct {
	ID         string               `json:"id"`
	Properties stacProperties       `json:"properties"`
	Assets     map[string]stacAsset `json:"assets"`
}

type stacProperties struct {
	DateTime        string `json:"datetime"`
	LetzteAenderung string `json:"letzte_aenderung"`
}

type stacAsset struct {
	Href string `json:"href"`
}

// Search lists the LoD2 tiles intersecting bb, ordered by (NorthingKM,
// EastingKM). It follows the search's next links and drops duplicate items.
// It fails with ErrTooManyTiles past MaxTiles tiles and with
// ErrHostNotAllowed when an item's CityGML asset is off the allowlist.
// A box that intersects no tile yields an empty slice and no error.
func (c *Client) Search(ctx context.Context, bb BBox) ([]Tile, error) {
	if err := bb.Validate(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, searchTimeout)
	defer cancel()

	next, err := c.searchURL(bb)
	if err != nil {
		return nil, err
	}

	byID := map[string]Tile{}
	visited := map[string]bool{}

	for page := 0; next != ""; page++ {
		if page >= maxSearchPages {
			return nil, fmt.Errorf("%w: search did not end after %d pages", ErrUnavailable, maxSearchPages)
		}

		visited[next] = true

		p, err := c.fetchPage(ctx, next)
		if err != nil {
			return nil, err
		}

		for _, item := range p.Features {
			tile, err := c.tileFromItem(item)
			if err != nil {
				return nil, err
			}

			if prev, ok := byID[tile.ID]; !ok || tile.Updated > prev.Updated {
				byID[tile.ID] = tile
			}
		}

		if len(byID) > MaxTiles {
			return nil, &TooManyTilesError{Found: len(byID), Max: MaxTiles}
		}

		next, err = c.nextLink(p.Links, visited)
		if err != nil {
			return nil, err
		}
	}

	tiles := make([]Tile, 0, len(byID))
	for _, t := range byID {
		tiles = append(tiles, t)
	}

	slices.SortFunc(tiles, func(a, b Tile) int {
		return cmp.Or(
			cmp.Compare(a.NorthingKM, b.NorthingKM),
			cmp.Compare(a.EastingKM, b.EastingKM),
			strings.Compare(a.ID, b.ID),
		)
	})

	return tiles, nil
}

func (c *Client) searchURL(bb BBox) (string, error) {
	base, err := c.allowedURL(c.stacURL + "/search")
	if err != nil {
		return "", err
	}

	coords := make([]string, 0, 4)
	for _, v := range []float64{bb.West, bb.South, bb.East, bb.North} {
		coords = append(coords, strconv.FormatFloat(v, 'f', -1, 64))
	}

	q := url.Values{
		"collections": {collectionID},
		"bbox":        {strings.Join(coords, ",")},
		"limit":       {strconv.Itoa(searchPageLimit)},
	}
	base.RawQuery = q.Encode()

	return base.String(), nil
}

// nextLink returns the page's rel=next href, or "" on the last page. A next
// link off the allowlist is refused; one already visited ends the search
// with an error rather than looping.
func (c *Client) nextLink(links []stacLink, visited map[string]bool) (string, error) {
	for _, l := range links {
		if l.Rel != "next" || l.Href == "" {
			continue
		}

		if _, err := c.allowedURL(l.Href); err != nil {
			return "", fmt.Errorf("STAC next link: %w", err)
		}

		if visited[l.Href] {
			return "", fmt.Errorf("%w: STAC next link %q repeats a page", ErrUnavailable, l.Href)
		}

		return l.Href, nil
	}

	return "", nil
}

func (c *Client) fetchPage(ctx context.Context, pageURL string) (stacPage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return stacPage{}, fmt.Errorf("STAC request: %w", err)
	}

	req.Header.Set("Accept", "application/geo+json")
	req.Header.Set("User-Agent", userAgent())

	resp, err := c.http.Do(req)
	if err != nil {
		return stacPage{}, transportError(ctx, "STAC search", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return stacPage{}, fmt.Errorf("%w: STAC search answered %s", ErrUnavailable, resp.Status)
	}

	var p stacPage
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxSearchPageBytes)).Decode(&p); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return stacPage{}, fmt.Errorf("STAC search: %w", ctxErr)
		}

		return stacPage{}, fmt.Errorf("%w: STAC search response is not valid JSON: %w", ErrUnavailable, err)
	}

	return p, nil
}

// transportError classifies a failed request: the caller's own cancellation
// or deadline stays a context error, everything else is the service's.
func transportError(ctx context.Context, what string, err error) error {
	if errors.Is(err, ErrHostNotAllowed) {
		return fmt.Errorf("%s: %w", what, err)
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("%s: %w", what, ctxErr)
	}

	return fmt.Errorf("%w: %s: %w", ErrUnavailable, what, err)
}

func (c *Client) tileFromItem(item stacItem) (Tile, error) {
	e, n, ok := parseTileID(item.ID)
	if !ok {
		return Tile{}, fmt.Errorf("%w: unrecognised STAC item id %q", ErrUnavailable, item.ID)
	}

	updated, ok := itemDate(item.Properties)
	if !ok {
		return Tile{}, fmt.Errorf("%w: STAC item %q carries no usable date", ErrUnavailable, item.ID)
	}

	asset, ok := item.Assets[gmlAssetKey]
	if !ok || asset.Href == "" {
		return Tile{}, fmt.Errorf("%w: STAC item %q has no %s asset", ErrUnavailable, item.ID, gmlAssetKey)
	}

	if _, err := c.allowedURL(asset.Href); err != nil {
		return Tile{}, fmt.Errorf("STAC item %q: %w", item.ID, err)
	}

	return Tile{ID: item.ID, EastingKM: e, NorthingKM: n, Updated: updated, GMLURL: asset.Href}, nil
}

// parseTileID parses LoD2_32_<eastingKm>_<northingKm>_1_ni. Only the
// canonical form is accepted: the ID becomes part of a cache file name.
func parseTileID(id string) (eastingKM, northingKM int, ok bool) {
	parts := strings.Split(id, "_")
	if len(parts) != 6 || parts[0] != "LoD2" || parts[1] != "32" || parts[4] != "1" || parts[5] != "ni" {
		return 0, 0, false
	}

	e, errE := strconv.Atoi(parts[2])
	n, errN := strconv.Atoi(parts[3])

	if errE != nil || errN != nil || e < 0 || n < 0 ||
		strconv.Itoa(e) != parts[2] || strconv.Itoa(n) != parts[3] {
		return 0, 0, false
	}

	return e, n, true
}

const dateLayout = "2006-01-02"

// itemDate prefers letzte_aenderung and falls back to the date of datetime.
func itemDate(p stacProperties) (string, bool) {
	if d, ok := parseDate(p.LetzteAenderung); ok {
		return d, true
	}

	if t, err := time.Parse(time.RFC3339, p.DateTime); err == nil {
		return t.UTC().Format(dateLayout), true
	}

	return "", false
}

// parseDate returns s when it is a valid YYYY-MM-DD date.
func parseDate(s string) (string, bool) {
	t, err := time.Parse(dateLayout, s)
	if err != nil || t.Format(dateLayout) != s {
		return "", false
	}

	return s, true
}
