//go:build browser

package browser_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"sync"

	"github.com/go-rod/rod/lib/launcher"
)

// headlessBlankPage is what a usable headless browser prints for about:blank with --dump-dom.
var headlessBlankPage = []byte(`<html><head></head><body></body></html>`)

// managedBrowser caches the Rod-managed browser for the whole test binary, so it is
// downloaded and validated once rather than before every launch.
var managedBrowser struct {
	once sync.Once
	bin  string
	err  error
}

// browserBinary returns the executable a launch should start. A binary selected with
// Rod's -rod=bin=<path> test flag is used as given. Otherwise the Rod-managed revision is
// resolved once, within ctx, so a missing, stalled or unusable binary fails every
// launch promptly with the same diagnostic instead of reaching the package timeout.
func browserBinary(ctx context.Context, configured string) (string, error) {
	if configured != "" {
		return configured, nil
	}
	managedBrowser.once.Do(func() {
		managedBrowser.bin, managedBrowser.err = resolveManagedBrowser(ctx, launcher.NewBrowser())
	})
	return managedBrowser.bin, managedBrowser.err
}

// resolveManagedBrowser downloads the managed revision when it is absent and validates
// it, with both steps bounded by ctx. Rod's own resolution validates without a deadline.
// A cached binary that fails validation is reported, with how to replace it, rather than deleted.
func resolveManagedBrowser(ctx context.Context, managed *launcher.Browser) (string, error) {
	managed.Context = ctx
	bin := managed.BinPath()
	_, err := os.Stat(bin)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err := managed.Download(); err != nil {
			return "", fmt.Errorf(
				"download the Rod-managed browser: %w; %s",
				err,
				browserSelectionHint,
			)
		}
	case err != nil:
		return "", fmt.Errorf(
			"inspect the Rod-managed browser %s: %w; %s",
			bin,
			err,
			browserSelectionHint,
		)
	}
	if err := validateBrowser(ctx, bin); err != nil {
		return "", fmt.Errorf(
			"validate the Rod-managed browser %s: %w; remove %s to download it again, or %s",
			bin,
			err,
			managed.Dir(),
			browserSelectionHint,
		)
	}
	return bin, nil
}

const browserSelectionHint = "to use an installed browser, append -args -rod=bin=<path> to go test"

// validateBrowser renders about:blank headlessly with the same arguments Rod uses. The
// browser runs in its own process group, which is killed when ctx ends, and its output
// pipes are released after browserExitTimeout so a helper process cannot hold the call.
func validateBrowser(ctx context.Context, bin string) error {
	cmd := exec.CommandContext(ctx, bin, "--headless", "--no-sandbox", "--use-mock-keychain",
		"--disable-dev-shm-usage", "--disable-gpu", "--dump-dom", "about:blank")
	ownProcessGroup(cmd)
	cmd.WaitDelay = browserExitTimeout
	output, err := cmd.CombinedOutput()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("browser did not answer before the launch deadline: %w", ctxErr)
	}
	if err != nil {
		return fmt.Errorf("run browser: %w\n%s", err, output)
	}
	if !bytes.Contains(output, headlessBlankPage) {
		return errors.New("browser did not render a headless blank page")
	}
	return nil
}
