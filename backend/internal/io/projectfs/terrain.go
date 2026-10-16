package projectfs

import (
	"crypto/sha256"
	"encoding/hex"
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
// it. An uninitialised folder is refused before anything is written, for the
// reason SaveModelIf gives.
func (s Store) SaveTerrain(proj *project.Project, data []byte, expected string) error {
	_, err := s.Load()
	if err != nil {
		return err
	}

	err = os.MkdirAll(s.modelDir(), modelDirMode)
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

	// Loaded before the commit, so a manifest that cannot be read fails the
	// replace with the previous terrain still in place.
	current, err := s.Load()
	if err != nil {
		return err
	}

	err = checkExpectedHash("terrain", path, expected)
	if err != nil {
		return err
	}

	err = staged.Commit()
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "projectfs.SaveTerrain", "replace terrain file", err)
	}

	// The digest of what was just committed is known, so the status reports
	// it from the next read on rather than rehashing 50 MB to find it.
	info, err := os.Stat(path)
	if err == nil {
		sum := sha256.Sum256(data)
		terrainHashes.remember(path, info, hex.EncodeToString(sum[:]))
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
// per file. Size and modification time alone do not identify one — two
// same-sized terrains saved within one timestamp tick share both — so the
// remembered entry must also be the same file (os.SameFile), which a
// SaveTerrain rename never is: the staged file was created while the old one
// still existed. A stale answer could only make a client's expected_hash fail
// to match — SaveTerrain's own check always hashes the file itself — so a miss
// costs a 409, never a wrong overwrite.
func (s Store) TerrainHash() (string, error) {
	path := s.TerrainPath()

	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", domainerrors.New(domainerrors.KindNotFound, "projectfs.TerrainHash", "no terrain has been imported", err)
		}

		return "", domainerrors.New(domainerrors.KindInternal, "projectfs.TerrainHash", "stat terrain", err)
	}

	if sum, ok := terrainHashes.lookup(path, info); ok {
		return sum, nil
	}

	sum, err := hashFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", domainerrors.New(domainerrors.KindNotFound, "projectfs.TerrainHash", "no terrain has been imported", err)
		}

		return "", domainerrors.New(domainerrors.KindInternal, "projectfs.TerrainHash", "hash terrain", err)
	}

	// Remembered only if the file was not replaced while it was hashed:
	// otherwise the bytes read may be the new file's under the old one's
	// identity, or the reverse.
	after, err := os.Stat(path)
	if err == nil && sameFileVersion(info, after) {
		terrainHashes.remember(path, info, sum)
	}

	return sum, nil
}

// sameFileVersion reports whether two stats describe the same version of the
// same file: the same file, not merely the same name, and unchanged in size and
// modification time.
func sameFileVersion(a, b os.FileInfo) bool {
	return os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

// fileHashCache holds the last hash computed per path: one entry per project
// that was asked, replaced rather than added to when the file changes.
type fileHashCache struct {
	mu      sync.Mutex
	entries map[string]fileHashEntry
}

type fileHashEntry struct {
	info os.FileInfo
	sum  string
}

var terrainHashes = &fileHashCache{entries: map[string]fileHashEntry{}}

func (c *fileHashCache) lookup(path string, info os.FileInfo) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[path]
	if !ok || !sameFileVersion(entry.info, info) {
		return "", false
	}

	return entry.sum, true
}

func (c *fileHashCache) remember(path string, info os.FileInfo, sum string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[path] = fileHashEntry{info: info, sum: sum}
}
