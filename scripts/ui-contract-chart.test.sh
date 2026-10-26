#!/bin/sh
set -eu
repo_root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
chart="$repo_root/charts/data-product-controller"
base=$(helm template controller "$chart" --namespace data-product-system --set route.enabled=true --set route.host=catalog.example)
for contract in false true invalid; do
	for appearance in false true invalid; do
		rendered=$(helm template controller "$chart" --namespace data-product-system --set route.enabled=true --set route.host=catalog.example --set uiContract.enabled="$contract" --set uiAppearance.enabled="$appearance")
		[ "$rendered" = "$base" ] || {
			echo 'Retired settings changed rendered resources' >&2
			exit 1
		}
		printf '%s' "$rendered" | yq ea -o=json -I=0 'select(.kind == "Deployment")' - | jq -s -e 'all(.[]; all(.spec.template.spec.containers[]; all((.env // [])[]; (.name != "UI_CONTRACT_ENABLED" and .name != "UI_APPEARANCE_ENABLED" and .name != "UI_HOST_ORIGINS"))))' >/dev/null
		printf '%s' "$rendered" | yq ea -o=json -I=0 'select(.kind == "DataProduct")' - | jq -s -e 'length > 0 and all(.[]; .spec | has("ui") | not)' >/dev/null
	done
done
yq -o=json '.' "$chart/values.yaml" | jq -e 'has("uiContract") == false and has("uiAppearance") == false' >/dev/null
jq -e '.properties | has("uiContract") == false and has("uiAppearance") == false' "$chart/values.schema.json" >/dev/null
printf '%s\n' 'Retired UI chart settings are absent and cannot activate runtime behavior'
