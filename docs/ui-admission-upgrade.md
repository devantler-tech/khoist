# Stage 1: stop new UI declarations

This major release rejects new, added or changed `spec.ui` declarations. Remove
the field from publisher input. An unchanged legacy declaration remains stored,
survives unrelated edits and can be removed. Once removed, it cannot be added
again. API-only products use named outputs; the registry does not execute UI code.

The field stays typed in `data.devantler.tech/v1alpha1`, the only served and storage
version. This release performs no object conversion or storage migration and does
not remove the optional UI property from historical descriptor/report formats.

## Upgrade

1. Verify the selected release's chart and image signatures and pin their immutable
   digests using [released artifact acceptance](release-acceptance.md). Retain the
   previously verified chart, image and Helm revision for rollback.
2. Remove `spec.ui` from newly published declarations. The offline publisher check
   rejects it under both profiles without changing either report format. A complete
   inventory of existing declarations is required before the later field removal,
   rather than as a prerequisite for this ratcheting stage.
3. **Explicitly apply the selected release's CRD before upgrading workloads.** Helm
   does not upgrade CRDs from `crds/`. After verifying and downloading the chart to
   `release-chart.tgz`, use the target cluster's normal reviewed deployment process:

   ```sh
   helm show crds ./release-chart.tgz > release-crds.yaml
   kubectl apply -f release-crds.yaml
   kubectl wait crd/dataproducts.data.devantler.tech --for=condition=Established --timeout=60s
   ```

4. Read the live CRD back. Its `v1alpha1` schema must retain the typed `spec.ui`
   property with `x-kubernetes-validations` rule `false` and message
   `API-only products do not support spec.ui; remove it`. `status.storedVersions`
   must remain `[v1alpha1]`. A successful Helm upgrade alone does not prove this.
5. Upgrade the workloads using that same verified chart and image digest. Confirm
   current product readiness and the independently authorised query paths. Record
   deployment and inventory evidence in
   [platform#4890](https://github.com/devantler-tech/platform/issues/4890).

## Recover

Roll workloads back to the previously verified release's stored Helm revision.
Helm rollback leaves the newer CRD installed: unchanged legacy objects remain
readable and removable, while new UI writes remain rejected.

To restore UI writes as well, explicitly apply the previous verified release's
CRD with the same `helm show crds` and `kubectl apply` steps, then read its full
schema back. Preserve the existing CRD identity and stored `v1alpha1` version;
do not delete/recreate the CRD or its products. Returning to this release requires
explicitly applying this release's CRD again.

The admission and disposable Helm lifecycle tests verify this boundary. They do
not establish production adoption. Field removal remains a separate major release
under [ADR 0024](adr/0024-retire-legacy-ui-declaration-within-v1alpha1.md), gated on
complete zero-UI inventories and consumer adoption.
