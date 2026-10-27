#!/usr/bin/env bash
# Sourced only inside the disposable source acceptance harness.
: "${repo_root:?run through tests/source/run.sh}" "${test_dir:?run through tests/source/run.sh}"

# ui_admission_schema applies a verified chart's CRD without replacing its identity or stored version.
ui_admission_schema() {
	local chart=$1 rejecting=$2 uid=$3
	helm show crds "$chart" >"$test_dir/ui-admission-schema.yaml"
	publication_kube apply -f "$test_dir/ui-admission-schema.yaml" >/dev/null
	publication_kube wait crd/dataproducts.data.devantler.tech --for=condition=Established --timeout=60s >/dev/null
	publication_kube get crd/dataproducts.data.devantler.tech -o json | jq -e \
		--arg uid "$uid" --argjson rejecting "$rejecting" '
    .metadata.uid == $uid and .status.storedVersions == ["v1alpha1"] and
    (.spec.versions | length == 1) and (.spec.versions[0] |
      .name == "v1alpha1" and .served == true and .storage == true and
      (.schema.openAPIV3Schema.properties.spec.properties.ui as $ui |
        $ui.type == "object" and
        (any($ui["x-kubernetes-validations"][]?;
          .rule == "false" and .message == "API-only products do not support spec.ui; remove it")) == $rejecting))' >/dev/null
}

# ui_admission_product_snapshot preserves status content while real rollout transition clocks may advance.
ui_admission_product_snapshot() {
	local phase=$1
	publication_kube get dataproduct existing-export -o json | jq -Se '
    . as $product |
    select((.metadata.uid | type == "string" and length > 0) and .metadata.deletionTimestamp == null and
      .spec.ui.url == "https://data.example.com/legacy" and
      .spec.ui.title == "Legacy admission fixture" and
      .status.observedGeneration == .metadata.generation and
      any(.status.conditions[]; .type == "Ready" and .status == "True") and
      any(.status.conditions[]; .type == "ConnectorReady" and .status == "True") and
      all(.status.conditions[]; .observedGeneration == $product.metadata.generation and
        (.lastTransitionTime | fromdateiso8601 | type == "number"))) |
    {uid:.metadata.uid,spec:.spec,status:.status}' >"$test_dir/ui-admission-$phase-raw-product.json"
	# Keep the complete observation. Only the comparison permits transition clocks
	# to advance when actual workload rollouts temporarily change availability.
	jq -S '.status.conditions |= (sort_by(.type) | map(del(.lastTransitionTime)))' \
		"$test_dir/ui-admission-$phase-raw-product.json" >"$test_dir/ui-admission-$phase-product.json"
	if [[ $phase == after-crd-apply ]]; then
		cmp "$test_dir/ui-admission-baseline-raw-product.json" "$test_dir/ui-admission-$phase-raw-product.json"
	elif [[ $phase != baseline ]]; then
		cmp "$test_dir/ui-admission-baseline-product.json" "$test_dir/ui-admission-$phase-product.json"
		jq -e --slurpfile before "$test_dir/ui-admission-baseline-raw-product.json" '
      . as $after | all($before[0].status.conditions[]; . as $old |
        any($after.status.conditions[]; .type == $old.type and
          (.lastTransitionTime | fromdateiso8601) >= ($old.lastTransitionTime | fromdateiso8601)))' \
			"$test_dir/ui-admission-$phase-raw-product.json" >/dev/null
	fi
}

# ui_admission_release_lifecycle starts from the latest signed release and never substitutes a reconstructed chart.
ui_admission_release_lifecycle() {
	: "${product_digest:?immutable candidate image required}" "${candidate_chart:?packaged candidate required}"
	local release_dir=$test_dir/ui-admission-release allowance image runtime version revision crd_uid saved_revision=0 previous_revision=0 phase
	allowance=$(publication_budget)
	((allowance <= 180)) || allowance=180
	bash "$repo_root/scripts/verify-release-artifacts.sh" --profile khoist --tag v8.0.0 \
		--source-sha f8025c710b76d3809870176cf1fcd0c75ff8ff32 \
		--image-digest sha256:633fcc5efde4b0023e612e5a2dae6ce3aa3eb8a2800ed5cf152f0efc486e1a43 \
		--chart-digest sha256:d9eb5e1c3f16113d5e513110683c5158ca3aa03afd24d643e2ff59dd67160d61 \
		--publisher-repository devantler-tech/.github --publisher-sha 467f07582436a26b62cf4aa756423512f85ba0c0 \
		--platform linux/amd64 --output-dir "$release_dir" --timeout "$allowance"
	jq -se 'length == 1 and (.[0] | .complete == true and .tag == "v8.0.0" and
    .releaseProfile == "khoist" and .sourceSHA == "f8025c710b76d3809870176cf1fcd0c75ff8ff32" and
    .imageDigest == "sha256:633fcc5efde4b0023e612e5a2dae6ce3aa3eb8a2800ed5cf152f0efc486e1a43" and
    .chartDigest == "sha256:d9eb5e1c3f16113d5e513110683c5158ca3aa03afd24d643e2ff59dd67160d61" and
    .publisherRepository == "devantler-tech/.github" and
    .publisherSHA == "467f07582436a26b62cf4aa756423512f85ba0c0" and .platform == "linux/amd64")' "$release_dir/release.json" >/dev/null
	crd_uid=$(publication_kube get crd/dataproducts.data.devantler.tech -o json | jq -er '.metadata.uid | select(length > 0)')
	# Only the owned fixture temporarily restores the released contract to seed pre-existing UI.
	ui_admission_schema "$release_dir/release-chart.tgz" false "$crd_uid"
	publication_kube patch dataproduct existing-export --type=merge \
		-p '{"spec":{"ui":{"url":"https://data.example.com/legacy","title":"Legacy admission fixture"}}}' >/dev/null

	for phase in baseline candidate rollback restored; do
		if [[ $phase == baseline || $phase == rollback ]]; then
			version=8.0.0
			image=$(jq -er '.imageReference' "$release_dir/release.json")
			runtime=$(jq -er '.runtimeDigest' "$release_dir/release.json")
		else
			version=$(helm show chart "$candidate_chart" | yq -r '.version')
			image="localhost:5055/data-product-controller@$product_digest"
			runtime=$product_digest
		fi
		if [[ $phase == candidate ]]; then
			ui_admission_schema "$candidate_chart" true "$crd_uid"
			ui_admission_product_snapshot after-crd-apply
		fi
		if [[ $phase == rollback ]]; then
			publication_helm rollback dpc "$saved_revision" --wait --timeout 240s >/dev/null
		else
			local chart=$candidate_chart
			[[ $phase != baseline ]] || chart=$release_dir/release-chart.tgz
			jq --arg repository "${image%@*}" --arg digest "${image##*@}" '
        .image.repository=$repository | .image.digest=$digest |
        .httpSource.enabled=true | .connectorReadiness.enabled=true' "$test_dir/values.yaml" >"$test_dir/ui-admission-values.json"
			publication_helm upgrade dpc "$chart" --reset-values --skip-crds --values "$test_dir/ui-admission-values.json" \
				--post-renderer "$repo_root/tests/source/trust-test-ca.sh" --wait --timeout 240s >/dev/null
		fi
		revision=$(helm_installed_revision dpc "$version" "$image" khoist)
		((revision > previous_revision))
		if [[ $phase == baseline ]]; then
			saved_revision=$revision
		else
			# Helm rollback must leave the delivered rejecting CRD installed.
			publication_kube get crd/dataproducts.data.devantler.tech -o json | jq -e --arg uid "$crd_uid" '
        .metadata.uid == $uid and .status.storedVersions == ["v1alpha1"] and
        any(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.ui["x-kubernetes-validations"][]?;
          .rule == "false" and .message == "API-only products do not support spec.ui; remove it")' >/dev/null
		fi
		wait_for "signed Stage 1 $phase preserves current product readiness" 240 readiness True true
		installed_observation "ui-admission-$phase" "$image" "$runtime" KHoist
		installed_query_check
		# Exercise real ratcheting after every candidate/rollback revision without altering the spec.
		publication_kube annotate dataproduct existing-export "example.com/admission-phase=$phase" --overwrite >/dev/null
		ui_admission_product_snapshot "$phase"
		previous_revision=$revision
		echo "PASS: signed v8 Stage 1 $phase revision $revision preserves legacy UI, product identity, specification, status and live readiness"
	done
	publication_kube patch dataproduct existing-export --type=json -p '[{"op":"remove","path":"/spec/ui"}]' >/dev/null
	wait_for 'Stage 1 legacy removal retains current readiness' 240 readiness True true
	installed_query_check
}
