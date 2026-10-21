package cli

import (
	"os"
	"testing"
)

// cliStandInEnv makes the test binary stand in for `aconiq`. The API's run
// and estimate executors spawn os.Executable(), which under `go test` is this
// binary; with the variable set, it runs the CLI on its arguments instead of
// the tests, so a run started through `aconiq mcp` or `aconiq serve` in a
// test computes in a real `aconiq run` child.
const cliStandInEnv = "ACONIQ_TEST_AS_CLI"

func TestMain(m *testing.M) {
	if os.Getenv(cliStandInEnv) == "1" {
		os.Exit(Execute(os.Args[1:]))
	}

	os.Exit(m.Run())
}
