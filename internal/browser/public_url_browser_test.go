//go:build browser

package browser_test

import "testing"

// TestReleaseConsumerBlocksUndeclaredReads preserves the fixed public source policy without presentation sessions.
func TestReleaseConsumerBlocksUndeclaredReads(t *testing.T) {
	view := releaseBrowserPage(t, releaseFixture, "application/json; charset=utf-8", 200)
	if got := view.frame.MustEval(`async()=>{try{await fetch('/healthz',{credentials:'omit'});return 'allowed'}catch{return 'blocked'}}`).
		Str(); got != "blocked" {
		t.Fatal("independent consumer read an undeclared endpoint")
	}
	if view.requests.Load() != 1 || view.credentials.Load() {
		t.Fatal("source policy changed the bounded credential-free read")
	}
}
