package mcpserver

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// auditOutcomeOK is the outcome of a call the API accepted; a refused call
// records its error code instead.
const auditOutcomeOK = "ok"

// auditFileMode and auditDirMode keep the log to its owner, as the project's
// other logs are.
const (
	auditFileMode = 0o600
	auditDirMode  = 0o750
)

// auditEntry is one line of .noise/logs/mcp-audit.jsonl, as ADR-002 specifies
// it. The arguments are recorded only as a digest: a model payload can be
// large and can carry customer geometry, and the log has to be safe to hand
// to whoever reviews it.
type auditEntry struct {
	Time       string `json:"time"`
	Tool       string `json:"tool"`
	ArgsSHA256 string `json:"args_sha256"`
	// Outcome is "ok" or the error code the call was refused with — the API's,
	// or the MCP layer's own (invalid_arguments, confirm_required).
	Outcome string `json:"outcome"`
	// ModelHash is the model a successful save_model left in place.
	ModelHash string `json:"model_hash,omitempty"`
	// RunID is the run a run tool started or asked to stop, as the call named
	// it.
	RunID string `json:"run_id,omitempty"`
}

// affected is what a state-changing call touched, for its audit entry: the
// model a save left in place, or the run a run tool acted on.
type affected struct {
	modelHash string
	runID     string
}

// auditLog appends entries to one file. Each entry is one write of one line
// to a file opened O_APPEND, so no line is ever split, and the mutex keeps
// two calls of one session from interleaving their writes.
type auditLog struct {
	path  string
	clock func() time.Time
	mu    sync.Mutex
}

// pendingEntry is an audit log opened for the one entry a call will write.
type pendingEntry struct {
	log  *auditLog
	file *os.File
}

// open makes sure the entry for a call can be written before the call runs.
// A log that cannot be opened — a read-only disk, a permission, a path that
// is not a directory — refuses the write it would have recorded, so no write
// happens unaudited. A failure after this point, a full disk for one, can
// still lose the line; finish reports it.
func (a *auditLog) open() (*pendingEntry, error) {
	err := os.MkdirAll(filepath.Dir(a.path), auditDirMode)
	if err != nil {
		return nil, fmt.Errorf("create audit log directory: %w", err)
	}

	file, err := os.OpenFile(a.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, auditFileMode)
	if err != nil {
		return nil, fmt.Errorf("open audit log: %w", err)
	}

	return &pendingEntry{log: a, file: file}, nil
}

// finish writes the entry and closes the log. It is called exactly once per
// opened entry, whatever the call's outcome.
func (p *pendingEntry) finish(tool string, arguments json.RawMessage, outcome string, touched affected) error {
	line, err := json.Marshal(auditEntry{
		Time:       p.log.clock().UTC().Format(time.RFC3339),
		Tool:       tool,
		ArgsSHA256: argumentsDigest(arguments),
		Outcome:    outcome,
		ModelHash:  touched.modelHash,
		RunID:      touched.runID,
	})
	if err != nil {
		_ = p.file.Close()

		return fmt.Errorf("encode audit entry: %w", err)
	}

	p.log.mu.Lock()
	_, err = p.file.Write(append(line, '\n'))
	p.log.mu.Unlock()

	if err != nil {
		_ = p.file.Close()

		return fmt.Errorf("append audit entry: %w", err)
	}

	err = p.file.Close()
	if err != nil {
		return fmt.Errorf("close audit log: %w", err)
	}

	return nil
}

// argumentsDigest is the SHA-256 of the arguments in canonical form —
// compact, object keys sorted, numbers spelled as sent — so the same call
// digests the same however the client spaced or ordered it. Arguments that
// are not JSON are digested as received.
func argumentsDigest(arguments json.RawMessage) string {
	canonical := []byte(arguments)

	decoder := json.NewDecoder(bytes.NewReader(arguments))
	decoder.UseNumber()

	var value any

	if decoder.Decode(&value) == nil {
		encoded, err := json.Marshal(value)
		if err == nil {
			canonical = encoded
		}
	}

	sum := sha256.Sum256(canonical)

	return hex.EncodeToString(sum[:])
}
