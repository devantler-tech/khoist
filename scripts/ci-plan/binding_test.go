package main

import (
	"os"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestWorkflowKeepsTrustedCommitBindings(t *testing.T) {
	data, err := os.ReadFile("../../.github/workflows/ci.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateTrustedBindings(data); err != nil {
		t.Fatal(err)
	}
	base := "${{ github.event.pull_request.base.sha || github.event.merge_group.base_sha || github.sha }}"
	for _, tc := range []struct{ name, before, after string }{
		{"candidate classifier", "working-directory: ci-trusted", "working-directory: ci-candidate"},
		{"head checkout", "ref: " + base, "ref: ${{ github.event.pull_request.head.sha }}"},
		{"wrong tested identity", "TESTED_SHA: ${{ github.sha }}", "TESTED_SHA: ${{ github.event.pull_request.head.sha }}"},
		{"wrong base identity", "BASE_SHA: " + base, "BASE_SHA: ${{ github.sha }}"},
		{"candidate ref", "path: ci-candidate", "ref: refs/heads/main\n          path: ci-candidate"},
		{"candidate path", "path: ci-candidate", "path: ci-unchecked"},
		{"persisted credentials", "persist-credentials: false", "persist-credentials: true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mutated := strings.ReplaceAll(string(data), tc.before, tc.after)
			if mutated == string(data) {
				t.Fatal("trust-boundary mutation did not change the workflow")
			}
			if err := validateTrustedBindings([]byte(mutated)); err == nil {
				t.Fatal("accepted redirected CI identity or execution")
			}
		})
	}
}

type bindingStep struct {
	ID               string
	Uses             string
	With             map[string]any
	Env              map[string]string
	WorkingDirectory string `yaml:"working-directory"`
}

func validateTrustedBindings(data []byte) error {
	var workflow struct {
		Jobs map[string]struct{ Steps []bindingStep }
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return err
	}
	base := "${{ github.event.pull_request.base.sha || github.event.merge_group.base_sha || github.sha }}"
	for _, job := range []string{"plan", "required-checks"} {
		trusted := 0
		for _, step := range workflow.Jobs[job].Steps {
			if step.With["path"] == "ci-trusted" {
				trusted++
				if !strings.HasPrefix(step.Uses, "actions/checkout@") || step.With["ref"] != base ||
					step.With["persist-credentials"] != false {
					return errPlan("checker and classifier must use the exact trusted event base")
				}
			}
		}
		if trusted != 1 {
			return errPlan("missing or duplicate trusted checkout")
		}
	}
	candidate, selector := 0, 0
	for _, step := range workflow.Jobs["plan"].Steps {
		if step.With["path"] == "ci-candidate" {
			candidate++
			if !strings.HasPrefix(step.Uses, "actions/checkout@") ||
				(step.With["ref"] != nil && step.With["ref"] != "${{ github.sha }}") ||
				step.With["fetch-depth"] != 0 || step.With["persist-credentials"] != false {
				return errPlan("candidate must be the complete exact tested tree")
			}
		}
		if step.ID == "select" {
			selector++
			if step.WorkingDirectory != "ci-trusted" || step.Env["BASE_SHA"] != base ||
				step.Env["TESTED_SHA"] != "${{ github.sha }}" ||
				step.Env["EVENT_NAME"] != "${{ github.event_name }}" {
				return errPlan("classifier execution and event identities must remain trusted")
			}
		}
	}
	if candidate != 1 || selector != 1 {
		return errPlan("missing or duplicate tested tree or selector")
	}
	return nil
}
