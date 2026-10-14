package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"testing"
)

// freeLoopbackAddr returns a loopback address with a port nothing listens on.
func freeLoopbackAddr(t *testing.T) string {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find a free port: %v", err)
	}

	addr := listener.Addr().String()

	err = listener.Close()
	if err != nil {
		t.Fatalf("release port: %v", err)
	}

	return addr
}

// `aconiq --json serve` runs until it is stopped, so its stdout is read as it
// comes: one JSON line naming where it listens, and nothing after it, through
// a graceful stop included. The log is on stderr.
func TestJSONServeAnnouncesItselfInOneDocument(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("stopping the server gracefully needs os.Interrupt")
	}

	projectDir := t.TempDir()

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Serve", "--crs", "EPSG:25832")

	addr := freeLoopbackAddr(t)

	encoded, err := json.Marshal([]string{"--project", projectDir, "--json", "serve", "--listen", addr})
	if err != nil {
		t.Fatalf("encode args: %v", err)
	}

	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestHelperProcessAconiq$")

	cmd.Env = append(os.Environ(), cliChildArgsEnv+"="+string(encoded))

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}

	err = cmd.Start()
	if err != nil {
		t.Fatalf("start aconiq serve: %v", err)
	}

	reader := bufio.NewReader(stdoutPipe)

	// The announcement is one line, so a client can read it with a line
	// read and know the server is up.
	line, err := reader.ReadString('\n')
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()

		t.Fatalf("read the announcement: %v\nstderr:\n%s", err, stderr.String())
	}

	err = cmd.Process.Signal(os.Interrupt)
	if err != nil {
		t.Fatalf("interrupt aconiq serve: %v", err)
	}

	rest, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read the rest of stdout: %v", err)
	}

	err = cmd.Wait()
	if err != nil {
		t.Fatalf("aconiq serve did not stop cleanly: %v\nstderr:\n%s", err, stderr.String())
	}

	document := assertOneJSONDocument(t, line+string(rest))

	if document["command"] != "serve" || document["url"] != "http://"+addr ||
		document["health"] != "http://"+addr+"/api/v1/health" {
		t.Fatalf("announcement %v does not name %s", document, addr)
	}

	records := jsonLogRecords(t, stderr.String())
	if !hasRecord(records, "serve started") {
		t.Fatalf("stderr is missing the serve log records:\n%s", stderr.String())
	}
}
