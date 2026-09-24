package lglnimport

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// FetchTile returns the path of t's CityGML in cacheDir, downloading it when
// it is not there yet. The cache file is <ID>_<Updated>.gml, so a tile the
// LGLN has updated since is fetched afresh; the older dates of the same tile
// are removed. The download streams into a temporary file that is renamed
// into place only when complete, so the cache never holds a partial tile.
// A download past MaxTileBytes fails with ErrTileTooLarge.
func (c *Client) FetchTile(ctx context.Context, t Tile, cacheDir string) (path string, cached bool, err error) {
	if _, _, ok := parseTileID(t.ID); !ok {
		return "", false, fmt.Errorf("%w: id %q", ErrInvalidTile, t.ID)
	}

	if _, ok := parseDate(t.Updated); !ok {
		return "", false, fmt.Errorf("%w: tile %s has date %q, want YYYY-MM-DD", ErrInvalidTile, t.ID, t.Updated)
	}

	if _, err := c.allowedURL(t.GMLURL); err != nil {
		return "", false, fmt.Errorf("tile %s: %w", t.ID, err)
	}

	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		return "", false, fmt.Errorf("create tile cache: %w", err)
	}

	path = filepath.Join(cacheDir, t.ID+"_"+t.Updated+".gml")

	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
		removeStale(cacheDir, t.ID, path)
		return path, true, nil
	}

	if err := c.download(ctx, t, cacheDir, path); err != nil {
		return "", false, err
	}

	removeStale(cacheDir, t.ID, path)

	return path, false, nil
}

func (c *Client) download(ctx context.Context, t Tile, cacheDir, path string) error {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.GMLURL, nil)
	if err != nil {
		return fmt.Errorf("tile %s request: %w", t.ID, err)
	}

	req.Header.Set("User-Agent", userAgent())

	resp, err := c.http.Do(req)
	if err != nil {
		return transportError(ctx, "tile "+t.ID, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: tile %s answered %s", ErrUnavailable, t.ID, resp.Status)
	}

	if resp.ContentLength > c.maxTileBytes {
		return fmt.Errorf("%w: tile %s announces %d bytes, limit %d", ErrTileTooLarge, t.ID, resp.ContentLength, c.maxTileBytes)
	}

	return c.store(ctx, t.ID, resp.Body, cacheDir, path)
}

// store streams body into a temporary file in cacheDir and renames it to
// path once the whole body is in, within the size cap. On any failure the
// temporary file is removed.
func (c *Client) store(ctx context.Context, id string, body io.Reader, cacheDir, path string) error {
	tmp, err := os.CreateTemp(cacheDir, id+"-*.part")
	if err != nil {
		return fmt.Errorf("create tile temp file: %w", err)
	}

	committed := false

	defer func() {
		if !committed {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()

	n, err := io.Copy(tmp, io.LimitReader(body, c.maxTileBytes+1))
	if err != nil {
		return transportError(ctx, "tile "+id+" body", err)
	}

	if n > c.maxTileBytes {
		return fmt.Errorf("%w: tile %s is larger than %d bytes", ErrTileTooLarge, id, c.maxTileBytes)
	}

	if n == 0 {
		return fmt.Errorf("%w: tile %s is empty", ErrUnavailable, id)
	}

	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync tile %s: %w", id, err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close tile %s: %w", id, err)
	}

	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("store tile %s: %w", id, err)
	}

	committed = true

	return nil
}

// removeStale deletes cached copies of tile id other than keep: the dates a
// newer Updated has superseded. It is best-effort; a leftover costs disk,
// not correctness, because the file name carries the date.
func removeStale(cacheDir, id, keep string) {
	matches, err := filepath.Glob(filepath.Join(cacheDir, id+"_*.gml"))
	if err != nil {
		return
	}

	for _, m := range matches {
		if m == keep {
			continue
		}

		_ = os.Remove(m)
	}
}
