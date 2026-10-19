package proc

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// Output is written out by hand rather than inherited from exec.Cmd, so
// check it keeps exec's contract: stdout returned, and a failed exit's
// stderr on the *exec.ExitError — ytresolve shows that to the user.
func TestOutputKeepsStderrOnFailure(t *testing.T) {
	name, args := "sh", []string{"-c", "echo out; echo oops >&2; exit 3"}
	if runtime.GOOS == "windows" {
		name, args = "cmd", []string{"/c", "echo out& echo oops 1>&2& exit 3"}
	}
	out, err := CommandContext(context.Background(), name, args...).Output()
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("err = %v, want an *exec.ExitError", err)
	}
	if !strings.Contains(string(out), "out") {
		t.Errorf("stdout = %q, want it to contain %q", out, "out")
	}
	if !strings.Contains(string(ee.Stderr), "oops") {
		t.Errorf("ExitError.Stderr = %q, want it to contain %q", ee.Stderr, "oops")
	}
}
