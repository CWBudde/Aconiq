package projectfs

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
)

const (
	toolName    = "noise"
	toolVersion = "dev"
)

// Store persists project metadata and run provenance in the local project folder.
type Store struct {
	root string
}

// CreateRunSpec describes metadata for appending a run record and provenance file.
type CreateRunSpec struct {
	ScenarioID    string
	Standard      project.StandardRef
	ReceiverMode  string
	ReceiverSetID string
	Parameters    map[string]string
	Metadata      map[string]string
	InputPaths    []string
	Status        string
	LogLines      []string
}

// New returns a store rooted at projectPath.
func New(projectPath string) (Store, error) {
	if projectPath == "" {
		return Store{}, domainerrors.New(domainerrors.KindUserInput, "projectfs.New", "project path is required", nil)
	}

	abs, err := filepath.Abs(projectPath)
	if err != nil {
		return Store{}, domainerrors.New(domainerrors.KindUserInput, "projectfs.New", "resolve absolute project path", err)
	}

	return Store{root: abs}, nil
}

func (s Store) Root() string {
	return s.root
}

func (s Store) ManifestPath() string {
	return s.manifestPath()
}

// Exists reports whether the project manifest already exists.
func (s Store) Exists() (bool, error) {
	_, err := os.Stat(s.manifestPath())
	if err == nil {
		return true, nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return false, domainerrors.New(domainerrors.KindInternal, "projectfs.Exists", "stat project manifest", err)
}

// Init creates a v1 project manifest and required local folders.
func (s Store) Init(name string, crs string) (project.Project, error) {
	exists, err := s.Exists()
	if err != nil {
		return project.Project{}, err
	}

	if exists {
		return project.Project{}, domainerrors.New(domainerrors.KindUserInput, "projectfs.Init", "project already initialized: "+s.manifestPath(), nil)
	}

	err = os.MkdirAll(s.root, 0o755)
	if err != nil {
		return project.Project{}, domainerrors.New(domainerrors.KindInternal, "projectfs.Init", "create project directory", err)
	}

	err = os.MkdirAll(s.runsDir(), 0o755)
	if err != nil {
		return project.Project{}, domainerrors.New(domainerrors.KindInternal, "projectfs.Init", "create runs directory", err)
	}

	err = os.MkdirAll(s.artifactsDir(), 0o755)
	if err != nil {
		return project.Project{}, domainerrors.New(domainerrors.KindInternal, "projectfs.Init", "create artifacts directory", err)
	}

	err = os.MkdirAll(s.logsDir(), 0o755)
	if err != nil {
		return project.Project{}, domainerrors.New(domainerrors.KindInternal, "projectfs.Init", "create logs directory", err)
	}

	if name == "" {
		name = filepath.Base(s.root)
	}

	if crs == "" {
		crs = "EPSG:4326"
	}

	now := time.Now().UTC()

	proj := project.Project{
		ManifestVersion: project.CurrentManifestVersion,
		ProjectID:       buildID("proj"),
		Name:            name,
		CRS:             crs,
		Storage: project.StorageConfig{
			Kind:  project.StorageKindJSONV1,
			Notes: "Phase 3 choice: JSON-only metadata; SQLite may be introduced later.",
		},
		Scenarios: []project.Scenario{
			{
				ID:   "default",
				Name: "Default Scenario",
				Standard: project.StandardRef{
					ID:      "unassigned",
					Version: "v0",
					Profile: "default",
				},
				CreatedAt: now,
			},
		},
		Runs:      make([]project.Run, 0),
		Artifacts: make([]project.ArtifactRef, 0),
		Migrations: project.MigrationState{
			CurrentVersion:         project.CurrentManifestVersion,
			LatestSupportedVersion: project.CurrentManifestVersion,
			History:                make([]project.MigrationRecord, 0),
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	err = s.Save(proj)
	if err != nil {
		return project.Project{}, err
	}

	return proj, nil
}

// Load reads and decodes the project manifest.
func (s Store) Load() (project.Project, error) {
	payload, err := os.ReadFile(s.manifestPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return project.Project{}, domainerrors.New(domainerrors.KindNotFound, "projectfs.Load", fmt.Sprintf("project is not initialized (%s missing)", s.manifestPath()), err)
		}

		return project.Project{}, domainerrors.New(domainerrors.KindInternal, "projectfs.Load", "read project manifest", err)
	}

	var proj project.Project

	err = json.Unmarshal(payload, &proj)
	if err != nil {
		return project.Project{}, domainerrors.New(domainerrors.KindValidation, "projectfs.Load", "decode project manifest", err)
	}

	if proj.ManifestVersion <= 0 {
		return project.Project{}, domainerrors.New(domainerrors.KindValidation, "projectfs.Load", "manifest version must be set", nil)
	}

	return proj, nil
}

// Save writes the project manifest atomically.
func (s Store) Save(proj project.Project) error {
	proj.UpdatedAt = time.Now().UTC()

	serialized, err := json.MarshalIndent(proj, "", "  ")
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "projectfs.Save", "encode project manifest", err)
	}

	serialized = append(serialized, '\n')

	err = os.MkdirAll(filepath.Dir(s.manifestPath()), 0o755)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "projectfs.Save", "create manifest directory", err)
	}

	tmpPath := s.manifestPath() + ".tmp"

	err = os.WriteFile(tmpPath, serialized, 0o600)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "projectfs.Save", "write temporary project manifest", err)
	}

	err = os.Rename(tmpPath, s.manifestPath())
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "projectfs.Save", "replace project manifest", err)
	}

	return nil
}

// CreateRun appends a run entry, creates a log file, and writes a provenance manifest.
func (s Store) CreateRun(spec CreateRunSpec) (project.Run, project.ProvenanceManifest, error) {
	proj, err := s.Load()
	if err != nil {
		return project.Run{}, project.ProvenanceManifest{}, err
	}

	scenarioID := spec.ScenarioID
	if scenarioID == "" {
		scenarioID = "default"
	}

	scenario, ok := findScenarioByID(proj.Scenarios, scenarioID)
	if !ok {
		return project.Run{}, project.ProvenanceManifest{}, domainerrors.New(domainerrors.KindUserInput, "projectfs.CreateRun", fmt.Sprintf("scenario %q does not exist", scenarioID), nil)
	}

	std := spec.Standard
	if std.ID == "" {
		std = scenario.Standard
	}

	if std.ID == "" || std.ID == "unassigned" {
		return project.Run{}, project.ProvenanceManifest{}, domainerrors.New(domainerrors.KindUserInput, "projectfs.CreateRun", "standard ID is required", nil)
	}

	if std.Version == "" {
		std.Version = "v0"
	}

	if std.Profile == "" {
		std.Profile = "default"
	}

	return s.persistRun(proj, spec, scenarioID, std)
}

func (s Store) persistRun(proj project.Project, spec CreateRunSpec, scenarioID string, std project.StandardRef) (project.Run, project.ProvenanceManifest, error) {
	now := time.Now().UTC()
	runID := buildID("run")
	runDir := filepath.Join(s.runsDir(), runID)

	err := os.MkdirAll(runDir, 0o755)
	if err != nil {
		return project.Run{}, project.ProvenanceManifest{}, domainerrors.New(domainerrors.KindInternal, "projectfs.CreateRun", "create run directory", err)
	}

	status := spec.Status
	if status == "" {
		status = project.RunStatusCompleted
	}

	logRelPath := filepath.ToSlash(filepath.Join(".noise", "runs", runID, "run.log"))
	provRelPath := filepath.ToSlash(filepath.Join(".noise", "runs", runID, "provenance.json"))

	err = writeRunLog(filepath.Join(s.root, filepath.FromSlash(logRelPath)), spec.LogLines)
	if err != nil {
		return project.Run{}, project.ProvenanceManifest{}, err
	}

	inputHashes, err := s.hashInputs(spec.InputPaths)
	if err != nil {
		return project.Run{}, project.ProvenanceManifest{}, err
	}

	provenance := project.ProvenanceManifest{
		RunID:         runID,
		ScenarioID:    scenarioID,
		Standard:      std,
		ReceiverMode:  spec.ReceiverMode,
		ReceiverSetID: spec.ReceiverSetID,
		Parameters:    cloneStringMap(spec.Parameters),
		Metadata:      cloneStringMap(spec.Metadata),
		InputHashes:   inputHashes,
		GeneratedAt:   now,
		ToolName:      toolName,
		ToolVersion:   toolVersion,
	}

	err = writeJSONFile(filepath.Join(s.root, filepath.FromSlash(provRelPath)), provenance)
	if err != nil {
		return project.Run{}, project.ProvenanceManifest{}, err
	}

	run := project.Run{
		ID:             runID,
		ScenarioID:     scenarioID,
		Standard:       std,
		ReceiverMode:   spec.ReceiverMode,
		ReceiverSetID:  spec.ReceiverSetID,
		Status:         status,
		LogPath:        logRelPath,
		ProvenancePath: provRelPath,
		StartedAt:      now,
		FinishedAt:     now,
	}

	proj.Runs = append(proj.Runs, run)

	err = s.Save(proj)
	if err != nil {
		return project.Run{}, project.ProvenanceManifest{}, err
	}

	provenance.ManifestPath = provRelPath

	return run, provenance, nil
}

func (s Store) controlDir() string {
	return filepath.Join(s.root, ".noise")
}

func (s Store) manifestPath() string {
	return filepath.Join(s.controlDir(), "project.json")
}

func (s Store) runsDir() string {
	return filepath.Join(s.controlDir(), "runs")
}

func (s Store) artifactsDir() string {
	return filepath.Join(s.controlDir(), "artifacts")
}

func (s Store) logsDir() string {
	return filepath.Join(s.controlDir(), "logs")
}

func (s Store) hashInputs(paths []string) (map[string]string, error) {
	if len(paths) == 0 {
		return map[string]string{}, nil
	}

	resolved := make([]string, 0, len(paths))
	for _, p := range paths {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}

		resolved = append(resolved, trimmed)
	}

	slices.Sort(resolved)

	hashes := make(map[string]string, len(resolved))
	for _, p := range resolved {
		absPath := p
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(s.root, p)
		}

		content, err := os.ReadFile(absPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, domainerrors.New(domainerrors.KindUserInput, "projectfs.hashInputs", "input file does not exist: "+p, err)
			}

			return nil, domainerrors.New(domainerrors.KindInternal, "projectfs.hashInputs", "read input file: "+p, err)
		}

		sum := sha256.Sum256(content)
		hashes[filepath.ToSlash(p)] = hex.EncodeToString(sum[:])
	}

	return hashes, nil
}

func writeRunLog(path string, lines []string) error {
	if len(lines) == 0 {
		lines = []string{"run executed"}
	}

	text := strings.Join(lines, "\n") + "\n"

	err := os.WriteFile(path, []byte(text), 0o600)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "projectfs.writeRunLog", "write run log", err)
	}

	return nil
}

func writeJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "projectfs.writeJSONFile", "encode json", err)
	}

	data = append(data, '\n')

	err = os.WriteFile(path, data, 0o600)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "projectfs.writeJSONFile", "write json file "+path, err)
	}

	return nil
}

func findScenarioByID(scenarios []project.Scenario, id string) (project.Scenario, bool) {
	for _, s := range scenarios {
		if s.ID == id {
			return s, true
		}
	}

	return project.Scenario{}, false
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}

	out := make(map[string]string, len(in))
	maps.Copy(out, in)

	return out
}

func buildID(prefix string) string {
	buf := make([]byte, 6)
	{
		_, err := rand.Read(buf)
		if err != nil {
			return fmt.Sprintf("%s-%d", prefix, time.Now().UTC().UnixNano())
		}
	}

	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(buf))
}
