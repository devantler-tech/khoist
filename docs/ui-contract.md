# Legacy UI metadata

KHoist accepts bounded legacy `v1alpha1` UI declarations and descriptor fields as public metadata.
They grant no data access and activate no product code. The registry and publisher-review application
never mount product frames, import publisher JavaScript or exchange presentation messages.
Consumer applications own their interfaces and use named API outputs and standard contracts.

The product-session runtime, sample `/ui` route, `/ui-contract-config` routes, kit assets and
`ui-contract` / `ui-appearance` activation gates are absent. Legacy chart values and process settings
cannot restore them. The independent KSail release browser remains available without a presentation
session. See [API-only integration](adr/0023-api-only-product-integration.md) and
[the API consumer guide](api-consumer.md).

## Accepted compatibility metadata

A legacy `spec.ui` or descriptor `ui` object may contain a public HTTPS URL, a title and a closed
`contract` object. Versioned schema retirement is separate from runtime removal; do not reject or
rewrite stored Kubernetes declarations solely to upgrade this runtime.

- URLs follow the literal public HTTPS profile, with no credentials or fragments.
- Titles are nonblank and contain at most 200 UTF-16 units; the standalone UI metadata limit is 16 KiB.
- Contract versions are `data-product-ui/v1` and `data-product-ui/v2`.
- Host origins are exact canonical HTTPS origins, with at most 16 distinct entries.
- Capability names are bounded and unique. V1 accepts `status` and `resize`; v2 also accepts
  `appearance`. An appearance declaration under v1 remains invalid.
- Unknown fields, malformed strings, contradictory identity/readiness observations, duplicate named
  collections and lossy numeric tokens remain rejected by the canonical metadata validators.

These fields are inert descriptions. A valid declaration does not establish a session, a current
publisher approval, a fresh readiness observation or a reachable endpoint. The registry’s legacy
presentation configuration booleans remain false.

## Inspect without execution

The independent [publisher-review host](publisher-preflight.md) serves `metadata.js` and
`descriptor.js` for bounded offline inspection. It has no product execution library and its policy
denies frames and connections. The registry loads the same inert metadata libraries.

New preflight reports do not require retired presentation gates. Existing public report identifiers
remain readable as historical report data; they do not become active feature flags. Saved reports
still pass closed shape, bound, provenance and declaration checks before display or export.

## Adopt and recover

Use a reviewed signed release and the normal GitOps deployment process. Existing `/ui-kit`
deployments retain only the publisher route while consumers move to the dedicated
`/publisher-review` executable. Its independent `publisher-preflight` gate remains default-off.

Remove unused legacy chart/process settings from deployment-owned configuration. Runtime rollback
uses an earlier verified immutable release; schema and stored-declaration migration have separate
acceptance criteria under [#392](https://github.com/devantler-tech/khoist/issues/392) and
[platform adoption](https://github.com/devantler-tech/platform/issues/4890). Source tests and
packaged rollback proof do not establish production adoption.
