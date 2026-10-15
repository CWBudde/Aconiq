package projectfs

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aconiq/backend/internal/atomicfile"
	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
)

// terrainFileName is the project DTM under .noise/model/. Like the model file
// names it is part of the project format: `aconiq run` finds the terrain
// through the artifact ref, and the ref names this path.
const terrainFileName = "terrain.tif"

// TerrainPath returns the absolute path of the project DTM, derived from the
// store root and never from a manifest ref, for the reason ReadModel gives.
func (s Store) TerrainPath() string {
	return filepath.Join(s.modelDir(), terrainFileName)
}

// SaveTerrain replaces the project DTM with data and registers it as the
// terrain artifact, under the same compare-and-write SaveModelIf has: expected
// is the current terrain's hash (TerrainHash), ExpectNone, or empty for no
// precondition. A refusal is a *HashMismatchError and writes nothing.
//
// It is the one persistence path for a terrain, shared by `aconiq import
// --terrain` and `POST /api/v1/import/terrain`. Callers validate the GeoTIFF
// first; the store persists what it is given.
//
// The file is replaced through a rename, not rewritten, because an `aconiq run`
// may be reading the previous DTM and takes no lock of ours. *proj is replaced
// with the manifest as saved. The lock is taken here, so a caller must not hold
// it.
func (s Store) SaveTerrain(proj *project.Project, data []byte, expected string) error {
	err := os.MkdirAll(s.modelDir(), modelDirMode)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "projectfs.SaveTerrain", "create model directory", err)
	}

	path := s.TerrainPath()

	staged, err := atomicfile.Stage(path, data)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "projectfs.SaveTerrain", "write terrain file", err)
	}
	defer staged.Discard()

	unlock, err := s.LockManifest()
	if err != nil {
		return err
	}
	defer unlock()

	err = checkExpectedHash("terrain", path, expected)
	if err != nil {
		return err
	}

	err = staged.Commit()
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "projectfs.SaveTerrain", "replace terrain file", err)
	}

	current, err := s.Load()
	if err != nil {
		return err
	}

	current.Artifacts = upsertArtifactRef(current.Artifacts, project.ArtifactRef{
		ID:        project.ArtifactIDTerrain,
		Kind:      project.ArtifactKindTerrainGeoTIFF,
		Path:      s.RelativePath(path),
		CreatedAt: time.Now().UTC(),
	})

	err = s.Save(current)
	if err != nil {
		return err
	}

	*proj = current

	return nil
}

// TerrainHash returns the SHA-256 of the project DTM as bare lowercase hex,
// the spelling ModelHash uses, or a NotFound error when there is no terrain.
//
// It is what the project status reports, and the status is rebuilt every two
// seconds for every open event stream: rehashing a DTM of up to 50 MB on each
// of those is work with the same answer every time. So the hash is remembered
// per file, keyed on its size and modification time, which a SaveTerrain
// rename always changes. A stale answer could only make a client's
// expected_hash fail to match — SaveTerrain's own check always hashes the file
// itself — so a miss costs a 409, never a wrong overwrite.
func (s Store) TerrainHash() (string, error) {
	path := s.TerrainPath()

	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", domainerrors.New(domainerrors.KindNotFound, "projectfs.TerrainHash", "no terrain has been imported", err)
		}

		return "", domainerrors.New(domainerrors.KindInternal, "projectfs.TerrainHash", "stat terrain", err)
	}

	key := fileHashKey{size: info.Size(), modTime: info.ModTime().UnixNano()}

	if sum, ok := terrainHashes.lookup(path, key); ok {
		return sum, nil
	}

	sum, err := hashFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", domainerrors.New(domainerrors.KindNotFound, "projectfs.TerrainHash", "no terrain has been imported", err)
		}

		return "", domainerrors.New(domainerrors.KindInternal, "projectfs.TerrainHash", "hash terrain", err)
	}

	terrainHashes.remember(path, key, sum)

	return sum, nil
}

// fileHashKey identifies one version of a file well enough to reuse its hash.
type fileHashKey struct {
	size    int64
	modTime int64
}

// fileHashCache holds the last hash computed per path: one entry per project
// that was asked, replaced rather than added to when the file changes.
type fileHashCache struct {
	mu      sync.Mutex
	entries map[string]fileHashEntry
}

type fileHashEntry struct {
	key fileHashKey
	sum string
}

var terrainHashes = &fileHashCache{entries: map[string]fileHashEntry{}}

func (c *fileHashCache) lookup(path string, key fileHashKey) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[path]
	if !ok || entry.key != key {
		return "", false
	}

	return entry.sum, true
}

func (c *fileHashCache) remember(path string, key fileHashKey, sum string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[path] = fileHashEntry{key: key, sum: sum}
}
