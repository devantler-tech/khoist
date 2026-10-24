//go:build browser && unix

package browser_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/utils"
)

// writeExecutable creates a shell script that stands in for a browser binary.
func writeExecutable(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "browser")
	writeScript(t, path, body)
	return path
}

// writeScript writes an executable shell script at path, creating its directory.
func writeScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create stand-in browser directory: %v", err)
	}
	//nolint:gosec // G306: a stand-in browser must be executable; only this user can read it.
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o700); err != nil {
		t.Fatalf("write stand-in browser: %v", err)
	}
}

func TestBrowserValidationRejectsABinaryWithoutHeadlessOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), resolutionDeadline)
	defer cancel()
	err := validateBrowser(ctx, writeExecutable(t, "echo 'not a browser'\n"))
	if err == nil || !strings.Contains(err.Error(), "headless blank page") {
		t.Fatalf("validate a non-browser: error = %v, want the headless-output diagnostic", err)
	}
}

// TestBrowserValidationStopsABlockedBinaryAndItsHelpers proves a stalled validation
// returns at the deadline and that a helper process the stand-in started is killed
// with it. The helper is first observed alive, so the check can detect a survivor.
func TestBrowserValidationStopsABlockedBinaryAndItsHelpers(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "helper.pid")
	bin := writeExecutable(t, fmt.Sprintf("sleep 600 &\necho $! > '%s'\nwait\n", pidFile))

	ctx, cancel := context.WithTimeout(t.Context(), resolutionDeadline)
	defer cancel()
	helper := make(chan int, 1)
	go func() { helper <- waitForLiveHelper(pidFile, resolutionDeadline) }()

	started := time.Now()
	err := validateBrowser(ctx, bin)
	elapsed := time.Since(started)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("validate a blocked binary: error = %v, want the deadline", err)
	}
	if limit := resolutionDeadline + browserExitTimeout; elapsed > limit {
		t.Fatalf("blocked binary returned after %s, want at most %s", elapsed, limit)
	}
	pid := <-helper
	if pid == 0 {
		t.Fatal("the stand-in's helper was never observed alive, so its cleanup cannot be judged")
	}
	gone := time.Now().Add(browserExitTimeout)
	for processAlive(pid) {
		if time.Now().After(gone) {
			t.Fatalf("helper process %d outlived the cancelled validation", pid)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestManagedBrowserWithAStalledCachedBinaryFailsAtTheDeadline reproduces a cached Rod
// browser that hangs while it is validated: resolution returns at the deadline with the
// selection hint, and the cached binary is left in place.
func TestManagedBrowserWithAStalledCachedBinaryFailsAtTheDeadline(t *testing.T) {
	managed := launcher.NewBrowser()
	managed.RootDir = t.TempDir()
	managed.Logger = utils.LoggerQuiet
	writeScript(t, managed.BinPath(), "exec sleep 600\n")

	ctx, cancel := context.WithTimeout(t.Context(), resolutionDeadline)
	defer cancel()
	started := time.Now()
	_, err := resolveManagedBrowser(ctx, managed)
	elapsed := time.Since(started)

	if !errors.Is(err, context.DeadlineExceeded) ||
		!strings.Contains(err.Error(), browserSelectionHint) {
		t.Fatalf(
			"stalled cached browser: error = %v, want the deadline and %q",
			err,
			browserSelectionHint,
		)
	}
	if limit := resolutionDeadline + browserExitTimeout; elapsed > limit {
		t.Fatalf("stalled cached browser returned after %s, want at most %s", elapsed, limit)
	}
	if _, err := os.Stat(managed.BinPath()); err != nil {
		t.Fatalf("cached browser was not left in place: %v", err)
	}
}

// waitForLiveHelper returns the helper's pid once it is recorded and running, or 0.
func waitForLiveHelper(pidFile string, within time.Duration) int {
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if raw, err := os.ReadFile(filepath.Clean(pidFile)); err == nil {
			if pid, err := strconv.Atoi(
				strings.TrimSpace(string(raw)),
			); err == nil &&
				processAlive(pid) {
				return pid
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	return 0
}

// processAlive reports whether pid names a process this user can signal.
func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}
