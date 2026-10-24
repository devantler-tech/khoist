//go:build browser && !unix

package browser_test

import "os/exec"

// ownProcessGroup keeps the default cancellation, which kills only the started process.
func ownProcessGroup(*exec.Cmd) {}
