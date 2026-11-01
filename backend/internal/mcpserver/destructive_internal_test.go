package mcpserver

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/api/httpv1"
)

// countingReader yields zero bytes up to its limit and counts what was read.
type countingReader struct {
	read  atomic.Int64
	limit int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	left := c.limit - c.read.Load()
	if left <= 0 {
		return 0, io.EOF
	}

	n := min(int64(len(p)), left)
	clear(p[:n])
	c.read.Add(n)

	return int(n), nil
}

// The in-process handler never looks at r.Context(), as the API's terrain
// handler does not, so a cancelled import must stop the stream itself: the
// handler's next read fails, the file is not read on to the cap, and the
// call answers the cancellation rather than the handler's refusal of a cut
// body.
func TestUploadTerrainStopsWhenTheCallIsCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var handlerRead atomic.Int64

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 32<<10)

		for {
			n, err := r.Body.Read(buf)
			if handlerRead.Add(int64(n)) > 0 {
				cancel()
			}

			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)

				return
			}
		}
	})

	file := &countingReader{limit: httpv1.MaxTerrainUploadBytes}
	tool := tools{api: apiClient{handler: handler}}

	done := make(chan error, 1)

	go func() {
		_, err := tool.uploadTerrain(ctx, file, "dtm.tif", "none", sha256.New())
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("a cancelled upload answered %v, want context.Canceled", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("a cancelled upload did not return")
	}

	if read := file.read.Load(); read >= httpv1.MaxTerrainUploadBytes {
		t.Fatalf("a cancelled upload read the whole file (%d bytes)", read)
	}
}
