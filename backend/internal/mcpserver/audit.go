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
	// or the MCP layer's own (confirm_required).
	Outcome string `json:"outcome"`
	// ModelHash is the model a successful save_model left in place.
	ModelHash string `json:"model_hash,omitempty"`
}

// auditLog appends entries to one file. Each entry is one write of one line
// to a file opened O_APPEND, so no line is ever split, and the mutex keeps two
// calls of one session from racing the directory creation and the open.
type auditLog struct {
	path  string
	clock func() time.Time
	mu    sync.Mutex
}

func (a *auditLog) record(tool string, arguments json.RawMessage, outcome string, modelHash string) error {
	line, err := json.Marshal(auditEntry{
		Time:       a.clock().UTC().Format(time.RFC3339),
		Tool:       tool,
		ArgsSHA256: argumentsDigest(arguments),
		Outcome:    outcome,
		ModelHash:  modelHash,
	})
	if err != nil {
		return fmt.Errorf("encode audit entry: %w", err)
	}

	line = append(line, '\n')

	a.mu.Lock()
	defer a.mu.Unlock()

	err = os.MkdirAll(filepath.Dir(a.path), auditDirMode)
	if err != nil {
		return fmt.Errorf("create audit log directory: %w", err)
	}

	file, err := os.OpenFile(a.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, auditFileMode)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}

	_, err = file.Write(line)
	if err != nil {
		_ = file.Close()

		return fmt.Errorf("append audit entry: %w", err)
	}

	err = file.Close()
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
