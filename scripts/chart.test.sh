#!/bin/sh
set -eu

# Chart rendering behavior lives in helm-unittest suites under
# charts/data-product-controller/tests/. This script runs them, then the checks
# helm-unittest cannot express: the Kubernetes version gate, file-level CRD and
# documentation checks, packaging, and the script-level publication and
# render-equality tests.

repo_root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
chart="$repo_root/charts/data-product-controller"
generated_crd="$repo_root/config/crd/bases/data.devantler.tech_dataproducts.yaml"
chart_crd="$chart/crds/data.devantler.tech_dataproducts.yaml"

fail() {
	printf '%s\n' "chart test failed: $1" >&2
	exit 1
}

command -v yq >/dev/null 2>&1 || fail 'yq is required to validate rendered RBAC'
helm unittest --help >/dev/null 2>&1 || fail 'the helm-unittest plugin is required (see AGENTS.md)'

helm unittest "$chart"

# helm-unittest ignores Chart.yaml kubeVersion, so exercise Helm's own gate,
# including the current Platform version.
for supported_version in 1.35.0 1.36.4 1.37.0; do
	helm template data-product-controller "$chart" --kube-version "$supported_version" >/dev/null ||
		fail "Kubernetes $supported_version must be accepted"
done
for unsupported_version in 1.34.0 1.34.12; do
	if unsupported_render=$(helm template data-product-controller "$chart" --kube-version "$unsupported_version" 2>&1); then
		fail "Kubernetes $unsupported_version must be rejected"
	fi
	printf '%s' "$unsupported_render" | grep -F 'requires kubeVersion' >/dev/null ||
		fail "Kubernetes $unsupported_version must fail the version gate"
done

cmp "$generated_crd" "$chart_crd" || fail "generated CRD copies differ"

# helm-unittest selects documents per template, so a duplicate workload or binding
# emitted by another template would still pass the suites. These whole-render
# totals keep the chart-wide counts.
# render_count <yq expression> <helm template args...>
render_count() {
	expression=$1
	shift
	helm template data-product-controller "$chart" "$@" | yq ea "$expression" -
}
workloads='[select(.kind == "Deployment") | .spec.template.spec.containers[]] | length'
optional='--set image.digest=sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa --set httpSource.enabled=true --set httpSource.secretName=existing-export --set httpSource.sourceCIDR=192.0.2.10/32 --set httpSource.consumerPodLabels.app=trusted-consumer'
probe='--set contractProbe.enabled=true --set contractProbe.url=https://contracts.example.com/schema --set contractProbe.targetCIDR=192.0.2.1/32 --set contractProbe.monitorPodLabels.app=monitor'
[ "$(render_count "$workloads" --namespace data-product-system)" = 2 ] || fail 'default render must have exactly 2 workload containers'
# shellcheck disable=SC2086 # The option strings are deliberately split into arguments.
[ "$(render_count "$workloads" --namespace data-product-system $optional)" = 3 ] || fail 'HTTP source render must have exactly 3 workload containers'
# shellcheck disable=SC2086
[ "$(render_count "$workloads" --namespace data-product-system $optional $probe)" = 4 ] || fail 'optional render must have exactly 4 workload containers'
for namespace in data-product-system alternate-products; do
	for kind in Role RoleBinding; do
		[ "$(render_count "[select(.kind == \"$kind\")] | length" --namespace "$namespace")" = 1 ] ||
			fail "the chart must render exactly one $kind ($namespace)"
	done
done

# The unit-test suites must never ship in the published chart.
package_dir=$(mktemp -d)
trap 'rm -rf "$package_dir"' EXIT HUP INT TERM
helm package "$chart" --destination "$package_dir" >/dev/null
if tar -tzf "$package_dir"/*.tgz | grep -E '^[^/]+/(tests/|\.helmignore$)' >/dev/null; then
	fail 'packaged chart must exclude development-only files'
fi

# The documented connector observer grant stays minimal and product-scoped.
rbac="$repo_root/docs/examples/connector-observer-rbac.yaml"
grant=$(yq ea 'select(.kind == "Role") | .rules' "$rbac" -o=json -I=0)
[ "$grant" = '[{"apiGroups":["apps"],"resources":["deployments"],"resourceNames":["export-http-source"],"verbs":["get"]}]' ] || fail 'example must grant only a named Deployment get'
role_namespace=$(yq ea 'select(.kind == "Role") | .metadata.namespace' "$rbac")
binding_namespace=$(yq ea 'select(.kind == "RoleBinding") | .metadata.namespace' "$rbac")
product_namespace=$(yq '.metadata.namespace' "$repo_root/docs/examples/http-source-product.yaml")
[ "$role_namespace" = "$product_namespace" ] && [ "$binding_namespace" = "$product_namespace" ] || fail 'observer access must be scoped to the product namespace'

sh "$repo_root/scripts/api-publication-chart.test.sh"
bash "$repo_root/scripts/publication-products.test.sh"
sh "$repo_root/scripts/ui-contract-chart.test.sh"
sh "$repo_root/scripts/ui-host-origins-chart.test.sh"

printf '%s\n' 'chart behavior tests passed'
