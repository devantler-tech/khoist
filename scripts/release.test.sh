#!/bin/sh
set -eu

repo_root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
workflow="$repo_root/.github/workflows/cd.yaml"
release_workflow="$repo_root/.github/workflows/release.yaml"
deployment="$repo_root/deploy/deployment.yaml"
deploy_kustomization="$repo_root/deploy/kustomization.yaml"
deploy_namespace="$repo_root/deploy/namespace.yaml"
deploy_rbac="$repo_root/deploy/rbac.yaml"
deploy_network_policy="$repo_root/deploy/network-policy.yaml"
generated_rbac="$repo_root/config/rbac/role.yaml"
generated_crd="$repo_root/config/crd/bases/data.devantler.tech_dataproducts.yaml"
deploy_crd="$repo_root/deploy/data.devantler.tech_dataproducts.yaml"
chart_crd="$repo_root/charts/data-product-controller/crds/data.devantler.tech_dataproducts.yaml"

fail() {
	printf '%s\n' "release contract test failed: $1" >&2
	exit 1
}

command -v yq >/dev/null 2>&1 || fail 'yq is required to validate release manifests'

yq -e '.jobs.release.with.disable-issue-side-effects | ((tag == "!!bool") and (. == true))' "$release_workflow" >/dev/null ||
	fail 'release must disable success/failure hooks that write to issues and pull requests'
yq -e '.jobs.release.permissions | (((.issues // "none") == "none") and ((."pull-requests" // "none") == "none"))' "$release_workflow" >/dev/null ||
	fail 'release must not grant issue or pull-request access after disabling their hooks'
yq -e '.jobs.release.permissions | ((.contents == "write") and (."id-token" == "write"))' "$release_workflow" >/dev/null ||
	fail 'release must retain contents and identity permissions for publication'

# Releases are signed as this repository's own cd.yaml on the release tag
# (devantler-tech/monorepo ADR 0017): the publish action must run directly in
# this job, never inside a reusable workflow whose identity the signature would name.
yq -e '.jobs.publish | has("uses") | not' "$workflow" >/dev/null ||
	fail 'CD must not sign through a reusable publisher workflow'
yq -e '[.jobs.publish.steps[] | select(has("uses") and (.uses | test("^devantler-tech/[.]github/actions/publish-app@")))] | length == 1' "$workflow" >/dev/null ||
	fail 'CD must publish with exactly one publish-app action step'
yq -e '.jobs.publish.steps[] | select(has("uses") and (.uses | test("^devantler-tech/[.]github/actions/publish-app@"))) | .uses | test("^devantler-tech/[.]github/actions/publish-app@[0-9a-f]{40}$")' "$workflow" >/dev/null ||
	fail 'CD must pin the publish-app action to a full commit SHA'
yq -e '.jobs.publish.steps[] | select(has("uses") and (.uses | test("^devantler-tech/[.]github/actions/publish-app@"))) | .with | ((.["app-name"] == "data-product-controller") and ((.mode // "deployment") == "deployment") and (has("oci-name") | not))' "$workflow" >/dev/null ||
	fail 'CD must build the controller image and pin it into the data-product-controller container'
yq -e '.jobs.publish.steps[0].uses | test("^actions/checkout@[0-9a-f]{40}$")' "$workflow" >/dev/null ||
	fail 'CD must check out the release source before publishing'
# shellcheck disable=SC2016 # ${{ github.sha }} is the literal workflow expression, not a shell expansion.
yq -e '.jobs.publish.steps[0].with | ((.["persist-credentials"] | ((tag == "!!bool") and (. == false))) and (.ref == "${{ github.sha }}"))' "$workflow" >/dev/null ||
	fail 'CD must check out the tagged commit without a persisted credential'
yq -e '.jobs.publish.permissions | ((length == 3) and (.contents == "read") and (.packages == "write") and (."id-token" == "write"))' "$workflow" >/dev/null ||
	fail 'CD publication must hold only checkout, package and signing permissions'
yq -e '.jobs.publish.concurrency["cancel-in-progress"] | ((tag == "!!bool") and (. == false))' "$workflow" >/dev/null ||
	fail 'CD publications must be serialized without cancellation'
yq -e '.jobs.publish.concurrency.queue == "max"' "$workflow" >/dev/null ||
	fail 'CD must retain pending release publications instead of replacing them'
# shellcheck disable=SC2016 # Literal GitHub expression: the old and new publisher share this lock.
yq -e '.jobs.publish.concurrency.group == "publish-verified-${{ github.repository }}"' "$workflow" >/dev/null ||
	fail 'CD must share the previous publisher lock to protect mutable aliases during the transition'

smoke=$(yq '.jobs.smoke.steps[] | select(.run != null) | .run' "$workflow")
# shellcheck disable=SC2016 # $RELEASE_TAG is the literal workflow variable, not a shell expansion.
printf '%s\n' "$smoke" | grep -F -- '--certificate-identity "https://github.com/devantler-tech/khoist/.github/workflows/cd.yaml@refs/tags/$RELEASE_TAG"' >/dev/null ||
	fail 'published-image verification must trust only this repository'"'"'s cd.yaml on the release tag'
if printf '%s\n' "$smoke" | grep -F 'publish-app.yaml' >/dev/null; then
	fail 'published-image verification must not trust the shared publisher'
fi

[ -f "$deployment" ] || fail 'deploy/deployment.yaml is required by publish-app'
cmp "$generated_crd" "$deploy_crd" || fail 'published and generated CRDs differ'
cmp "$generated_crd" "$chart_crd" || fail 'chart and generated CRDs differ'
container_count=$(yq '[.spec.template.spec.containers[] | select(.name == "data-product-controller")] | length' "$deployment")
[ "$container_count" = '1' ] ||
	fail 'deployment must contain exactly one data-product-controller container'
[ "$(yq '.spec.template.spec.containers[] | select(.name == "data-product-controller") | .image' "$deployment")" = 'ghcr.io/devantler-tech/khoist:latest' ] ||
	fail 'publish-app must receive the expected mutable image placeholder'

[ "$(yq '.namespace' "$deploy_kustomization")" = 'data-product-system' ] ||
	fail 'deploy kustomization must target data-product-system'
[ "$(yq '[.resources[] | select(. == "namespace.yaml")] | length' "$deploy_kustomization")" = '1' ] ||
	fail 'deploy kustomization must create its namespace'
[ "$(yq '.kind' "$deploy_namespace")" = 'Namespace' ] ||
	fail 'deploy/namespace.yaml must define a Namespace'
[ "$(yq '.metadata.name' "$deploy_namespace")" = 'data-product-system' ] ||
	fail 'deploy namespace must be data-product-system'
[ "$(yq '.metadata.namespace' "$deployment")" = 'data-product-system' ] ||
	fail 'deployment must declare the target namespace for standalone scanners'
[ "$(yq '.spec.template.spec.securityContext.runAsUser' "$deployment")" = '65532' ] ||
	fail 'deployment must run with the nonroot image UID'
[ "$(yq '.spec.template.spec.securityContext.runAsGroup' "$deployment")" = '65532' ] ||
	fail 'deployment must run with the nonroot image GID'
[ "$(yq '.spec.template.spec.containers[0].resources.limits.cpu' "$deployment")" = '100m' ] ||
	fail 'deployment must set a CPU limit'
[ "$(yq '.kind' "$deploy_network_policy")" = 'NetworkPolicy' ] ||
	fail 'deploy/network-policy.yaml must define a NetworkPolicy'
[ "$(yq '.metadata.namespace' "$deploy_network_policy")" = 'data-product-system' ] ||
	fail 'deploy NetworkPolicy must target data-product-system'
[ "$(yq '[.resources[] | select(. == "network-policy.yaml")] | length' "$deploy_kustomization")" = '1' ] ||
	fail 'deploy kustomization must include the NetworkPolicy'
cluster_lease_rules=$(
	yq ea '[select(.kind == "ClusterRole") | .rules[] | select(.resources[] == "leases")] | length' "$deploy_rbac"
)
[ "$cluster_lease_rules" = '0' ] || fail 'deploy ClusterRole must not grant cross-namespace Lease access'
namespace_lease_rules=$(
	yq ea '[select(.kind == "Role") | .rules[] | select(.resources[] == "leases")] | length' "$deploy_rbac"
)
[ "$namespace_lease_rules" = '1' ] || fail 'deploy Role must grant Lease access in data-product-system'
generated_cluster_lease_rules=$(
	yq ea '[select(.kind == "ClusterRole") | .rules[] | select(.resources[] == "leases")] | length' "$generated_rbac"
)
[ "$generated_cluster_lease_rules" = '0' ] ||
	fail 'controller-gen output must not grant cross-namespace Lease access'
generated_namespace_lease_rules=$(
	yq ea '[select(.kind == "Role" and .metadata.namespace == "data-product-system") | .rules[] | select(.resources[] == "leases")] | length' "$generated_rbac"
)
[ "$generated_namespace_lease_rules" = '1' ] ||
	fail 'controller-gen output must grant Lease access only in data-product-system'

sh "$repo_root/scripts/chart-signing.test.sh"

printf '%s\n' 'release contract tests passed'
