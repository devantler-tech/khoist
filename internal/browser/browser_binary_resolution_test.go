//go:build browser

package browser_test

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/utils"
)

// resolutionDeadline is short so the tests prove the bound rather than wait on it.
const resolutionDeadline = 2 * time.Second

func TestBrowserValidationRejectsAMissingBinaryPromptly(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), resolutionDeadline)
	defer cancel()
	started := time.Now()
	err := validateBrowser(ctx, filepath.Join(t.TempDir(), "missing-browser"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("validate a missing binary: error = %v, want not-exist", err)
	}
	if elapsed := time.Since(started); elapsed >= resolutionDeadline {
		t.Fatalf("missing binary took %s, want an immediate failure", elapsed)
	}
}

func TestManagedBrowserDownloadStopsAtTheDeadline(t *testing.T) {
	requested := make(chan struct{}, 1)
	stalled := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case requested <- struct{}{}:
		default:
		}
		<-r.Context().Done()
	}))
	defer stalled.Close()

	managed := launcher.NewBrowser()
	managed.RootDir = t.TempDir()
	managed.Logger = utils.LoggerQuiet
	managed.Hosts = []launcher.Host{func(int) string { return stalled.URL + "/browser.zip" }}

	ctx, cancel := context.WithTimeout(t.Context(), resolutionDeadline)
	defer cancel()
	started := time.Now()
	_, err := resolveManagedBrowser(ctx, managed)
	elapsed := time.Since(started)

	select {
	case <-requested:
	default:
		t.Fatal("the stalled download host was never asked, so the deadline was not exercised")
	}
	if err == nil || !strings.Contains(err.Error(), browserSelectionHint) {
		t.Fatalf(
			"stalled download: error = %v, want a bounded failure naming %q",
			err,
			browserSelectionHint,
		)
	}
	if limit := resolutionDeadline + browserExitTimeout; elapsed > limit {
		t.Fatalf("stalled download returned after %s, want at most %s", elapsed, limit)
	}
}
