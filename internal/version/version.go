// Package version exposes build-time metadata that gets baked into the
// binary via -ldflags -X during the release workflow.
//
// Local builds (e.g. `go build .`) report "dev" / "none" / "unknown".
// Release builds (from a tag push) report the real version, commit, and
// build date. See .github/workflows/release.yml for the injection.
//
// Typical output:
//	ytmgo v0.1.1 (abc1234, 2026-06-01T12:34:56Z)
package version

import "fmt"

// Set at link time — do not edit these directly except to change the
// "default" fallback (e.g. for `go install` users who don't rebuild).
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

// Full returns a human-readable version string for `--version` output.
func Full() string {
	return fmt.Sprintf("%s (%s, %s)", Version, Commit, BuildDate)
}
