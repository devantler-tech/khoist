#!/usr/bin/env bash
# Verify installed runtime absence and the delivered rejection of new legacy UI.
: "${repo_root:?run through tests/source/run.sh}"
: "${test_dir:?integration scratch directory is required}"

kube get deployment dpc -o json | jq -e 'all(.spec.template.spec.containers[]; all((.env // [])[]; (.name != "UI_CONTRACT_ENABLED" and .name != "UI_APPEARANCE_ENABLED" and .name != "UI_HOST_ORIGINS")))' >/dev/null
probe --url http://dpc/assets/ui-contract.js --want-status 404
probe --url http://dpc/api/v1/ui-config --want-status 404
probe --url http://dpc/api/v1/workspace-config --contains '"discoveryEnabled":false,"lineageEnabled":false'
echo 'PASS: installed controller exposes no presentation runtime or operator grants'

yq -o=json '.' "$repo_root/docs/examples/http-source-product.yaml" |
	jq '.metadata.name="ui-admission-fixture" | .spec.ui={url:"https://product.example/ui",title:"Synthetic interface",
		contract:{apiVersion:"data-product-ui/v2",hostOrigins:["https://catalog.example"],capabilities:["status","resize","appearance"]}}' \
		>"$test_dir/ui-admission.json"
# Only a real Invalid admission response with the requested cause counts as rejection.
reject_new_ui() {
	if kube apply --dry-run=server -f "$1" >"$test_dir/ui-admission.log" 2>&1; then
		echo 'admission unexpectedly accepted a new UI declaration' >&2
		return 1
	fi
	case "$(cat "$test_dir/ui-admission.log")" in
	*'is invalid:'* | *'(Invalid)'*) ;;
	*)
		cat "$test_dir/ui-admission.log" >&2
		return 1
		;;
	esac
	grep -F "$2" "$test_dir/ui-admission.log" >/dev/null
}
reject_new_ui "$test_dir/ui-admission.json" 'API-only products do not support spec.ui; remove it'
echo 'PASS: installed admission rejects a new bounded v2 appearance declaration'
jq '.spec.ui.contract.apiVersion="data-product-ui/v1"' "$test_dir/ui-admission.json" >"$test_dir/ui-invalid.json"
reject_new_ui "$test_dir/ui-invalid.json" 'Appearance requires data-product-ui/v2'
echo 'PASS: installed admission retains the v1 appearance negative control'
jq '.spec.ui.contract.capabilities=["status","resize"]' "$test_dir/ui-invalid.json" >"$test_dir/ui-v1.json"
reject_new_ui "$test_dir/ui-v1.json" 'API-only products do not support spec.ui; remove it'
jq 'del(.spec.ui)' "$test_dir/ui-v1.json" >"$test_dir/ui-free.json"
kube apply --dry-run=server -f "$test_dir/ui-free.json" >/dev/null
echo 'PASS: installed admission rejects new v1 UI and accepts the same UI-free declaration'
