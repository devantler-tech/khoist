package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddedADRPlan(t *testing.T) {
	for _, tc := range []struct {
		name, path, mode, want string
	}{
		{"new prose ADR", "docs/adr/0002-new-decision.md", "add", "adr"},
		{"multiple new ADRs", "docs/adr/0002-new-decision.md", "multiple", "adr"},
		{"modified ADR", "docs/adr/0001-existing.md", "modify", "full"},
		{"deleted ADR", "docs/adr/0001-existing.md", "delete", "full"},
		{"renamed ADR", "docs/adr/0002-renamed.md", "rename", "full"},
		{"mixed browser edit", "docs/adr/0002-new-decision.md", "mixed", "full"},
		{"executable ADR", "docs/adr/0002-new-decision.md", "executable", "full"},
		{"symlink ADR", "docs/adr/0002-new-decision.md", "symlink", "full"},
		{"oversized ADR", "docs/adr/0002-new-decision.md", "oversized", "full"},
		{"binary ADR", "docs/adr/0002-new-decision.md", "binary", "full"},
		{"invalid UTF-8 ADR", "docs/adr/0002-new-decision.md", "invalid-utf8", "full"},
		{"missing ADR object", "docs/adr/0002-new-decision.md", "missing", "full"},
		{"nested path", "docs/adr/0002-nested/new.md", "add", "full"},
		{"uppercase title", "docs/adr/0002-New.md", "add", "full"},
		{"missing number", "docs/adr/new.md", "add", "full"},
		{"long number", "docs/adr/00002-new.md", "add", "full"},
		{"empty title segment", "docs/adr/0002-new--decision.md", "add", "full"},
		{"non-Markdown", "docs/adr/0002-new.go", "add", "full"},
		{"runtime example", "docs/examples/0002-new-decision.md", "add", "full"},
		{"executable documentation", "docs/postgresql-age-image.md", "add", "full"},
		{"website", "website/src/content/docs/new.md", "add", "full"},
		{"workflow", ".github/workflows/new.yaml", "add", "full"},
		{"classifier", "scripts/ci-plan/new.go", "add", "full"},
		{"dependency", "go.sum", "add", "full"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, base := adrFixture(t)
			paths := changeADRFixture(t, repo, tc.path, tc.mode)
			gitTest(t, repo, append([]string{"add", "--"}, paths...)...)
			if tc.mode == "executable" {
				gitTest(t, repo, "executable-adr")
			}
			gitTest(t, repo, "commit", "-qm", "candidate")
			head := gitTest(t, repo, "rev-parse", "HEAD")
			if tc.mode == "missing" {
				removeFixtureBlob(t, repo)
			}
			got, err := classify(repo, base, head)
			if got != tc.want || (got == "adr" && err != nil) {
				t.Fatalf("profile=%s err=%v; want %s", got, err, tc.want)
			}
		})
	}
}

func adrFixture(t *testing.T) (string, string) {
	t.Helper()
	repo, _ := fixture(t)
	path := "docs/adr/0001-existing.md"
	write(t, repo, path, "# Existing decision\n")
	gitTest(t, repo, "add", "--", path)
	gitTest(t, repo, "commit", "-qm", "existing ADR")
	return repo, gitTest(t, repo, "rev-parse", "HEAD")
}

func changeADRFixture(t *testing.T, repo, path, mode string) []string {
	t.Helper()
	p := filepath.Join(repo, path)
	existing := "docs/adr/0001-existing.md"
	paths := []string{path}
	write(t, repo, path, "# New decision\n\nA bounded prose decision.\n")
	switch mode {
	case "delete":
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
	case "rename":
		if err := os.Rename(filepath.Join(repo, existing), p); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, existing)
	case "mixed":
		write(t, repo, browserFiles[0], "//go:build browser\npackage browser_test\n// edit\n")
		paths = append(paths, browserFiles[0])
	case "multiple":
		second := "docs/adr/0003-second-decision.md"
		write(t, repo, second, "# Another decision\n")
		paths = append(paths, second)
	case "symlink":
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("../../go.mod", p); err != nil {
			t.Fatal(err)
		}
	case "oversized":
		write(t, repo, path, strings.Repeat("a", 1024*1024+1))
	case "binary":
		write(t, repo, path, "# Decision\n\x00not prose")
	case "invalid-utf8":
		write(t, repo, path, "# Decision\n\xff")
	}
	return paths
}

func removeFixtureBlob(t *testing.T, repo string) {
	t.Helper()
	blob := gitTest(t, repo, "adr-blob")
	if len(blob) != 40 || strings.Trim(blob, "0123456789abcdef") != "" {
		t.Fatalf("unexpected fixture blob identity %q", blob)
	}
	if err := os.Remove(filepath.Join(repo, ".git", "objects", blob[:2], blob[2:])); err != nil {
		t.Fatal(err)
	}
}

func TestADRPlanRequiresExactAvailableCommits(t *testing.T) {
	repo, base := adrFixture(t)
	write(t, repo, "docs/adr/0002-new-decision.md", "# New decision\n")
	gitTest(t, repo, "add", "--", "docs/adr/0002-new-decision.md")
	gitTest(t, repo, "commit", "-qm", "candidate")
	head := gitTest(t, repo, "rev-parse", "HEAD")
	for _, commits := range [][2]string{
		{"HEAD", head},
		{base, "HEAD"},
		{"", head},
		{base, ""},
		{strings.Repeat("0", 40), head},
		{base, strings.Repeat("0", 40)},
		{base, base},
	} {
		got, err := classify(repo, commits[0], commits[1])
		if got != "full" {
			t.Fatalf("commits=%v profile=%s err=%v", commits, got, err)
		}
	}
}

func TestADRPlanIgnoresWorktreeAndReplacementRefs(t *testing.T) {
	repo, base := adrFixture(t)
	path := "docs/adr/0002-new-decision.md"
	write(t, repo, path, "# New decision\n")
	gitTest(t, repo, "add", "--", path)
	gitTest(t, repo, "commit", "-qm", "tested tree")
	head := gitTest(t, repo, "rev-parse", "HEAD")
	write(t, repo, "unrelated.go", "package unrelated\n")
	gitTest(t, repo, "add", "--", "unrelated.go")
	gitTest(t, repo, "commit", "-qm", "replacement tree")
	replacement := gitTest(t, repo, "rev-parse", "HEAD")
	gitTest(t, repo, "replace", head, replacement)
	write(t, repo, path, "\x00uncommitted bytes\n")
	got, err := classify(repo, base, head)
	if got != "adr" || err != nil {
		t.Fatalf("immutable tested tree redirected: profile=%s err=%v", got, err)
	}
}
