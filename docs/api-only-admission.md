# API-only admission checkpoint

The `admission`-tagged acceptance starts with the exact CRD extracted from the
signed v8.0.0 chart on an owned, disposable Kubernetes API server, then applies
the delivered schema. The delivered CRD retains the typed `spec.ui` property and
its existing validation, with a rejecting CEL rule at that field. Unchanged legacy UI
metadata can ratchet through unrelated edits; new or changed UI is rejected with
`API-only products do not support spec.ui; remove it`. The sole served/storage
version remains `v1alpha1`; feature defaults and stored objects are unchanged.

Required CI exercises checksum-pinned Kubernetes **1.35.0** and **1.37.0** envtest
assets, covering the chart's minimum version and the current upper admission
checkpoint. The provider lifecycle profiles use Kubernetes **1.35.8**. The
checkpoint verifies actual persisted objects after create, replacement, JSON
and merge patch, server-side apply, metadata and status updates. Rejected
requests must have the expected validation cause and preserve stored state;
timeouts, transport failures and apply conflicts do not count as rejection.

Removal checks preserve product UIDs, non-UI specifications, labels, annotations
and status. Apply cases inspect actual field ownership: sole-owner omission
removes UI; a non-owner's omission retains it; shared ownership retains it until
the final owner omits the field. Reintroduction is then rejected. Explicit null
is checked through a fresh GET and must be absent. A whole-specification-rule
negative control demonstrates why that broader placement blocks unrelated
edits. Restoration of the original server schema must preserve the CRD identity
and accept legacy creation and UI edits again.

The same tagged invocation checks the existing three-product publication example
through both offline API-only report formats. Normal chart and browser checks
retain the historical compatibility and independent publisher-review contracts.
Offline create validation does not predict update ratcheting or grant data access.

For a local run, download the official controller-tools envtest archive for the
chosen version and host architecture, verify its published checksum, and extract
it. Point the test at its absolute `controller-tools/envtest` directory:

```sh
ADMISSION_TEST_ASSETS=/absolute/path/controller-tools/envtest \
  ADMISSION_TEST_VERSION=v1.37.0 \
  go test -tags=admission -count=1 -timeout=600s -v ./tests/admission
```

Supported checkpoint versions are `v1.35.0` and `v1.37.0`. Missing assets or a
different running version fail the invocation. The fixture explicitly disables
existing-cluster mode, overrides ambient binary selection, and uses only the
configuration returned by its own API server. It never loads a user's kubeconfig.
Startup retries and shutdown use explicit process timeouts. The operation
deadline reserves time before the Go test alarm for schema recovery and shutdown;
normal test failures restore the owned original schema and stop the server.
Envtest also performs untimed setup, and its child processes have their own
process groups. Hosted-runner disposal is the backstop for a test alarm, an
unresponsive bootstrap binary or uncatchable process termination; the outer
command timeout alone does not prove child-process cleanup.

The required source lifecycle separately verifies the signed v8.0.0 image and
chart, installs that release, applies the candidate CRD explicitly, upgrades the
workloads, performs a real Helm rollback and restores the candidate. A stored
legacy UI declaration retains its UID, full specification and status; current
readiness and source queries must work at every phase. Helm rollback leaves the
rejecting rule installed. The older v4/v5 metadata rollback tests run under the
signed v8 schema because they intentionally reintroduce UI; they restore the
delivered schema afterwards and do not stand in for Stage 1 admission proof.

The CRD-only step preserves the complete stored status exactly. During real
workload rollouts, condition transition timestamps may advance as availability
changes; every other status value remains equal, timestamps stay valid and do
not move backwards, and readiness must reflect the current generation. Complete
status observations are retained alongside the comparison.

This is admission-migration evidence, not production activation or independent adoption.
[ADR 0024](adr/0024-retire-legacy-ui-declaration-within-v1alpha1.md) records how
this rule is the delivered first stage of the field's removal. Field removal and
signed Platform rollout remain in
[#392](https://github.com/devantler-tech/khoist/issues/392) and
[platform#4890](https://github.com/devantler-tech/platform/issues/4890).
