//go:build !windows

package deps

// pins is empty off Windows: install.sh and the package manager provide
// the tools there, so Fetch has nothing to download.
var pins = map[string]pin{}
