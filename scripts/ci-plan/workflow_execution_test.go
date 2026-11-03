package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestRequiredWorkflowExecutesTrustedChecker(t *testing.T) {
	run := requiredWorkflowRun(t)
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	write(t, dir, "ci-trusted/scripts/ci-plan/main.go", string(source))
	for _, tc := range []struct{ name, job, result, profile string }{
		{"valid ADR", "", "", "adr"},
		{"selected skipped", "core", "skipped", "adr"},
		{"selected cancelled", "browser", "cancelled", "adr"},
		{"failed classifier", "plan", "failure", "adr"},
		{"unselected ran", "source-integration", "success", "adr"},
		{"unknown profile", "", "", "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			needs := validNeeds("adr")
			if tc.job != "" {
				job := needs[tc.job]
				job.Result = tc.result
				needs[tc.job] = job
			}
			data, err := json.Marshal(needs)
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(t.Context(), "bash")
			cmd.Dir = dir
			cmd.Stdin = strings.NewReader(run)
			cmd.Env = append(os.Environ(), "GOENV=off", "GOWORK=off", "GOFLAGS=",
				"PROFILE="+tc.profile, "NEEDS_JSON="+string(data))
			output, err := cmd.CombinedOutput()
			if (err == nil) != (tc.name == "valid ADR") {
				t.Fatalf("required workflow result=%v output=%s", err, output)
			}
		})
	}
}

func requiredWorkflowRun(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../.github/workflows/ci.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct{ Steps []struct{ Run string } }
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	for _, step := range workflow.Jobs["required-checks"].Steps {
		if strings.Contains(step.Run, "ci-trusted/scripts/ci-plan/main.go check") {
			return step.Run
		}
	}
	t.Fatal("missing actual trusted result-checker step")
	return ""
}
