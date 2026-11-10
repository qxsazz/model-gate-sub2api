package service

import (
	"os"
	"os/exec"
	"regexp"
	"testing"
)

// runAllocationTestInSubprocess returns true when the caller's test has finished
// in a child process. AllocsPerRun counts process-wide Mallocs, so background
// work from unrelated service tests must not share the measurement process.
func runAllocationTestInSubprocess(t *testing.T) bool {
	t.Helper()
	const subprocessEnv = "SUB2API_SERVICE_ALLOCATION_TEST_SUBPROCESS"
	if os.Getenv(subprocessEnv) == t.Name() {
		return false
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("locate allocation test executable: %v", err)
	}
	cmd := exec.CommandContext(t.Context(), executable, "-test.run=^"+regexp.QuoteMeta(t.Name())+"$", "-test.count=1")
	cmd.Env = append(os.Environ(), subprocessEnv+"="+t.Name())
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("allocation subprocess failed: %v\n%s", err, output)
	}
	return true
}
