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
		{"wrong tested identity", "TESTED_SHA: ${{ github.sha }}", "TESTED_SHA: ${{ github.event.pull_request.head.sha }}"},
		{"wrong base identity", "BASE_SHA: " + base, "BASE_SHA: ${{ github.sha }}"},
		{"candidate ref", "path: ci-candidate", "ref: refs/heads/main\n          path: ci-candidate"},
		{"candidate path", "path: ci-candidate", "path: ci-unchecked"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if strings.Count(string(data), tc.before) != 1 {
				t.Fatal("plan mutation must identify exactly one declaration")
			}
			mutated := strings.Replace(string(data), tc.before, tc.after, 1)
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
	Run              string
}

// Pin the complete approved execution body, including argument order and the
// conservative fallback/output behavior. A command hidden in a comment or an
// additional output override must not satisfy this trust-boundary contract.
const trustedSelectorRun = `set -euo pipefail
profile=full
if [[ "$EVENT_NAME" == pull_request && -f scripts/ci-plan/main.go ]]; then
  if candidate=$(go run scripts/ci-plan/main.go classify "$GITHUB_WORKSPACE/ci-candidate" "$BASE_SHA" "$TESTED_SHA"); then
    case "$candidate" in
      browser|adr|full) profile="$candidate" ;;
      *) echo '::warning::Classifier output was unknown; retaining full acceptance.' ;;
    esac
  else
    echo '::warning::Classifier failed; retaining full acceptance.'
  fi
fi
printf 'profile=%s\n' "$profile" >> "$GITHUB_OUTPUT"
printf 'CI profile: %s; base=%s; tested=%s\n' "$profile" "$BASE_SHA" "$TESTED_SHA"
`

// Each checkout mutation leaves the other job's trusted binding intact, so an
// early rejection in plan cannot hide a missing required-checks validation.
func TestTrustedCheckoutBindingsAreIndependent(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../.github/workflows/ci.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateTrustedBindings(data); err != nil {
		t.Fatal(err)
	}
	for _, job := range []string{"plan", "required-checks"} {
		unchanged := mutateBindingJob(
			t,
			data,
			job,
			func(steps []bindingStep) []bindingStep { return steps },
		)
		if err := validateTrustedBindings(unchanged); err != nil {
			t.Fatalf("unmodified %s binding must survive fixture round-trip: %v", job, err)
		}
		for _, mutation := range []string{"head ref", "persisted credentials", "missing", "duplicate"} {
			t.Run(job+"/"+mutation, func(t *testing.T) {
				t.Parallel()
				mutated := mutateBindingJob(t, data, job, func(steps []bindingStep) []bindingStep {
					for index, step := range steps {
						if step.With["path"] != "ci-trusted" {
							continue
						}
						switch mutation {
						case "head ref":
							steps[index].With["ref"] = "${{ github.event.pull_request.head.sha }}"
						case "persisted credentials":
							steps[index].With["persist-credentials"] = true
						case "missing":
							return append(steps[:index:index], steps[index+1:]...)
						case "duplicate":
							return append(steps, step)
						}
						return steps
					}
					t.Fatal("missing original trusted checkout")
					return nil
				})
				if err := validateTrustedBindings(mutated); err == nil {
					t.Fatal(
						"accepted an independently redirected, missing or duplicate trusted checkout",
					)
				}
			})
		}
	}
}

// Checking only Env would accept all of these changed executable bodies.
func TestSelectorScriptBindingRejectsRedirectedExecution(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../.github/workflows/ci.yaml")
	if err != nil {
		t.Fatal(err)
	}
	unchanged := mutateBindingJob(
		t,
		data,
		"plan",
		func(steps []bindingStep) []bindingStep { return steps },
	)
	if err := validateTrustedBindings(unchanged); err != nil {
		t.Fatalf("unmodified selector must survive fixture round-trip: %v", err)
	}
	for _, tc := range []struct{ name, before, after string }{
		{"reversed commits", `classify "$GITHUB_WORKSPACE/ci-candidate" "$BASE_SHA" "$TESTED_SHA"`, `classify "$GITHUB_WORKSPACE/ci-candidate" "$TESTED_SHA" "$BASE_SHA"`},
		{"other tested checkout", `"$GITHUB_WORKSPACE/ci-candidate"`, `"$GITHUB_WORKSPACE/ci-unchecked"`},
		{"candidate classifier", "go run scripts/ci-plan/main.go", `go run "$GITHUB_WORKSPACE/ci-candidate/scripts/ci-plan/main.go"`},
		{"empty script", "", ""},
		{"comment-only invocation", "", "# go run scripts/ci-plan/main.go classify \"$GITHUB_WORKSPACE/ci-candidate\" \"$BASE_SHA\" \"$TESTED_SHA\"\nprofile=adr\n"},
		{"overridden output", "", "\nprintf 'profile=adr\\n' >> \"$GITHUB_OUTPUT\"\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mutated := mutateBindingJob(t, data, "plan", func(steps []bindingStep) []bindingStep {
				for index, step := range steps {
					if step.ID != "select" {
						continue
					}
					switch tc.name {
					case "empty script", "comment-only invocation":
						steps[index].Run = tc.after
					case "overridden output":
						steps[index].Run += tc.after
					default:
						if strings.Count(step.Run, tc.before) != 1 {
							t.Fatal(
								"selector mutation must identify exactly one executable argument",
							)
						}
						steps[index].Run = strings.Replace(step.Run, tc.before, tc.after, 1)
					}
					return steps
				}
				t.Fatal("missing original selector")
				return nil
			})
			if err := validateTrustedBindings(mutated); err == nil {
				t.Fatal("accepted a redirected, inert or overridden selector script")
			}
		})
	}
}

// mutateBindingJob round-trips only fields examined by this binding validator.
// Each mutation gets fresh maps from the actual workflow and can change only one job.
func mutateBindingJob(
	t *testing.T,
	data []byte,
	name string,
	mutate func([]bindingStep) []bindingStep,
) []byte {
	t.Helper()
	var workflow struct {
		Jobs map[string]struct{ Steps []bindingStep }
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	job, ok := workflow.Jobs[name]
	if !ok {
		t.Fatal("missing binding job")
	}
	job.Steps = mutate(job.Steps)
	workflow.Jobs[name] = job
	mutated, err := yaml.Marshal(workflow)
	if err != nil {
		t.Fatal(err)
	}
	return mutated
}

// validateTrustedBindings requires independent exact-base checkouts for the
// classifier and reducer, plus the complete approved selector execution body.
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
			if step.Run != trustedSelectorRun || step.WorkingDirectory != "ci-trusted" ||
				step.Env["BASE_SHA"] != base ||
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
