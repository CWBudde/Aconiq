package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aconiq/backend/internal/atomicfile"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/jsonio"
)

func copyFileIfExists(srcPath string, dstPath string) (copied bool, err error) {
	_, err = os.Stat(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, fmt.Errorf("stat export source %s: %w", srcPath, err)
	}

	err = os.MkdirAll(filepath.Dir(dstPath), 0o750)
	if err != nil {
		return false, fmt.Errorf("create export bundle directory %s: %w", filepath.Dir(dstPath), err)
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return false, fmt.Errorf("open export source %s: %w", srcPath, err)
	}
	defer func() { _ = src.Close() }()

	dst, err := os.Create(dstPath)
	if err != nil {
		return false, fmt.Errorf("create export bundle file %s: %w", dstPath, err)
	}
	defer func() {
		if cerr := dst.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close export bundle file %s: %w", dstPath, cerr)
		}
	}()

	_, err = io.Copy(dst, src)
	if err != nil {
		return false, fmt.Errorf("copy %s to export bundle: %w", srcPath, err)
	}

	return true, nil
}

func copyRunResultArtifactsToBundle(projectRoot string, bundleDir string, artifacts []project.ArtifactRef, runID string) (copiedRunResults, error) {
	filtered := make([]project.ArtifactRef, 0, len(artifacts))
	for _, artifact := range artifacts {
		if artifact.RunID != runID {
			continue
		}

		if !strings.HasPrefix(artifact.Kind, project.ArtifactKindRunResultPrefix) {
			continue
		}

		filtered = append(filtered, artifact)
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Kind == filtered[j].Kind {
			return filtered[i].Path < filtered[j].Path
		}

		return filtered[i].Kind < filtered[j].Kind
	})

	out := copiedRunResults{
		CopiedFiles:        make([]string, 0, len(filtered)),
		RasterMetadataList: make([]string, 0, len(filtered)),
	}

	usedTargets := make(map[string]struct{}, len(filtered))
	for _, artifact := range filtered {
		destRel := destinationPathForRunArtifact(artifact)
		destRel = ensureUniqueDestination(destRel, usedTargets)
		usedTargets[destRel] = struct{}{}

		srcPath := filepath.Join(projectRoot, filepath.FromSlash(artifact.Path))
		dstPath := filepath.Join(bundleDir, filepath.FromSlash(destRel))

		copied, err := copyFileIfExists(srcPath, dstPath)
		if err != nil {
			return copiedRunResults{}, err
		}

		if !copied {
			continue
		}

		out.CopiedFiles = append(out.CopiedFiles, filepath.ToSlash(destRel))

		switch artifact.Kind {
		case project.ArtifactKindRunResultReceiverTableJSON:
			out.ReceiverTableJSON = dstPath
		case project.ArtifactKindRunResultSummary:
			out.RunSummary = dstPath
		case project.ArtifactKindRunResultRasterMetadata:
			out.RasterMetadataList = append(out.RasterMetadataList, dstPath)
		}
	}

	out.CopiedFiles = dedupeAndSort(out.CopiedFiles)
	sort.Strings(out.RasterMetadataList)

	return out, nil
}

func copyModelDumpToBundle(projectRoot string, bundleDir string, artifacts []project.ArtifactRef) (string, string, error) {
	modelDumpPath := ""

	var latestAt time.Time

	for _, artifact := range artifacts {
		if artifact.Kind != project.ArtifactKindModelDumpJSON {
			continue
		}

		if modelDumpPath == "" || artifact.CreatedAt.After(latestAt) {
			modelDumpPath = artifact.Path
			latestAt = artifact.CreatedAt
		}
	}

	if modelDumpPath == "" {
		return "", "", nil
	}

	srcPath := filepath.Join(projectRoot, filepath.FromSlash(modelDumpPath))
	destRel := filepath.ToSlash(filepath.Join("model", "model.dump.json"))
	dstPath := filepath.Join(bundleDir, filepath.FromSlash(destRel))

	copied, err := copyFileIfExists(srcPath, dstPath)
	if err != nil {
		return "", "", err
	}

	if !copied {
		return "", "", nil
	}

	return dstPath, destRel, nil
}

// reprojectModelGeoJSON reads a normalized GeoJSON file, re-normalizes it from
// the project CRS into a target CRS, and overwrites the file in place.
func reprojectModelGeoJSON(geojsonPath string, projectCRS string, targetCRS string) error {
	data, err := os.ReadFile(geojsonPath)
	if err != nil {
		return fmt.Errorf("read model GeoJSON: %w", err)
	}

	// Re-normalize: the file is in projectCRS, and we want targetCRS.
	// NormalizeWithCRS(data, targetCRS, projectCRS, ...) will transform from projectCRS → targetCRS.
	model, err := modelgeojson.NormalizeWithCRS(data, targetCRS, projectCRS, "export")
	if err != nil {
		return fmt.Errorf("re-project %s -> %s: %w", projectCRS, targetCRS, err)
	}

	fc := model.ToFeatureCollection()

	out, err := jsonio.Marshal(fc)
	if err != nil {
		return fmt.Errorf("marshal re-projected GeoJSON: %w", err)
	}

	if err := atomicfile.WriteFile(geojsonPath, out); err != nil {
		return fmt.Errorf("write re-projected GeoJSON %s: %w", geojsonPath, err)
	}

	return nil
}

func copyModelGeoJSONToBundle(projectRoot string, bundleDir string, artifacts []project.ArtifactRef) (string, string, error) {
	modelGeoJSONPath := ""

	var latestAt time.Time

	for _, artifact := range artifacts {
		if artifact.Kind != project.ArtifactKindModelNormalizedGeoJSON {
			continue
		}

		if modelGeoJSONPath == "" || artifact.CreatedAt.After(latestAt) {
			modelGeoJSONPath = artifact.Path
			latestAt = artifact.CreatedAt
		}
	}

	if modelGeoJSONPath == "" {
		return "", "", nil
	}

	srcPath := filepath.Join(projectRoot, filepath.FromSlash(modelGeoJSONPath))
	destRel := filepath.ToSlash(filepath.Join("model", "model.normalized.geojson"))
	dstPath := filepath.Join(bundleDir, filepath.FromSlash(destRel))

	copied, err := copyFileIfExists(srcPath, dstPath)
	if err != nil {
		return "", "", err
	}

	if !copied {
		return "", "", nil
	}

	return dstPath, destRel, nil
}

func destinationPathForRunArtifact(artifact project.ArtifactRef) string {
	switch artifact.Kind {
	case project.ArtifactKindRunResultReceiverTableJSON:
		return filepath.ToSlash(filepath.Join("results", "receivers.json"))
	case project.ArtifactKindRunResultReceiverTableCSV:
		return filepath.ToSlash(filepath.Join("results", "receivers.csv"))
	case project.ArtifactKindRunResultSummary:
		return filepath.ToSlash(filepath.Join("results", "run-summary.json"))
	default:
		return filepath.ToSlash(filepath.Join("results", filepath.Base(artifact.Path)))
	}
}

func ensureUniqueDestination(destRel string, used map[string]struct{}) string {
	if _, exists := used[destRel]; !exists {
		return destRel
	}

	ext := filepath.Ext(destRel)

	base := strings.TrimSuffix(destRel, ext)
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", base, i, ext)
		if _, exists := used[candidate]; !exists {
			return candidate
		}
	}
}

func dedupeAndSort(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}

	seen := make(map[string]struct{}, len(values))

	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}

		normalized := filepath.ToSlash(trimmed)
		if _, ok := seen[normalized]; ok {
			continue
		}

		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}

	sort.Strings(out)

	return out
}
