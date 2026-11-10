# 0027: Keep full acceptance for changes with uncertain impact

Date: 2026-11-10

Status: Accepted

## Context

New decision records describe choices that may precede any implementation.
Running every Kubernetes and database scenario on these pull requests takes
runner capacity from changes that can affect those systems. A repository-wide
Markdown shortcut would be unsafe because some documentation is an input to
acceptance tests.

## Decision

Use the narrow coverage profile only when the reviewed base classifier can
prove that the exact pull request tree adds only new prose decision records.
Keep the normal documentation, security, browser, controller and release
contract checks. The [CI coverage guide](../continuous-integration.md) lists
the retained checks and the cases that require full coverage.

The base commit supplies both the classifier and the required result checker.
The candidate tree is evidence for classification. Keep full coverage for
changes to the workflow or classifier, for manual and merge-group runs, and
whenever the classifier cannot establish the narrower case.

## Consequences

- A new decision can receive feedback without competing for the cluster and
  database acceptance jobs.
- An edit to an existing decision still receives full coverage. Expanding the
  eligible files requires a separate review of their runtime readers and
  a native test showing that required checks still fail on incomplete results.
- Judge the improvement using a hosted run that records the selected profile,
  its actual job results and the time until the required check finishes.
