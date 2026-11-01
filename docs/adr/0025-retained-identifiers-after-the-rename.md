# 0025: Keep the identifiers the rename would break

Date: 2026-11-01

Status: Accepted

## Context

The repository, Go module, documentation, controller image and chart package now use the KHoist
name. Releases from v6.0.2 onwards publish `ghcr.io/devantler-tech/khoist` and
`ghcr.io/devantler-tech/charts/khoist`. Some identifiers still use the earlier
`data-product-controller` name.

Left unexplained, each one looks like an oversight, and a later cleanup could rename it. Several
would then break an installed cluster, a signature check or an upgrade test, so
[#399](https://github.com/devantler-tech/khoist/issues/399) asks for the deliberate ones to be
written down.

## Decision

Keep these identifiers under their current names. Each is part of a contract with something outside
this repository, or with a release that has already shipped.

| Identifier                                                                                  | Where it lives                                  | Why it stays                                                                                                                                                                                                                                                   |
|---------------------------------------------------------------------------------------------|-------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `DataProduct` kind and the `data.devantler.tech` API group                                  | CRD, stored objects, annotations                | Renaming them is a new API. It would orphan every stored product.                                                                                                                                                                                              |
| Leader election ID `data-product-controller.data.devantler.tech`                            | `main.go`                                       | Two controller versions must compete for one lease during a rolling update. A new ID would let both lead at once.                                                                                                                                              |
| Label `app.kubernetes.io/name: data-product-controller`                                     | Chart and `deploy/` selectors                   | A Deployment's selector is immutable, so changing it fails every in-place upgrade.                                                                                                                                                                             |
| Deployment and container name `data-product-controller`                                     | `deploy/deployment.yaml`                        | The publisher's `app-name` input finds the container by this name when it pins the released image. A new Deployment name would also leave the old one running.                                                                                                 |
| Namespace `data-product-system` in the signed manifests                                     | `deploy/`, README commands                      | A workload cannot move namespace in place, so a new name would mean reinstalling. Platform's own `data-product-controller` namespace and release name are Platform's to change, under [platform#4895](https://github.com/devantler-tech/platform/issues/4895). |
| `ghcr.io/devantler-tech/data-product-controller-postgresql-age` and its image source label  | `cd.yaml`, the AGE Dockerfile, `cnpg_hybrid.go` | The hybrid provider's capability check accepts one exact verified image reference. A new repository name needs a new release of that image and a reviewed change to the check, together.                                                                       |
| Chart directory `charts/data-product-controller` and its template helpers                   | Repository paths                                | They do not change what the chart installs, and the chart's published name is already `khoist`. Many scripts and docs use the path, so moving it is churn with no outside benefit.                                                                             |
| Catalogue `url` and `documentationUrl` values                                               | Chart product templates                         | The upgrade test requires a published product specification to stay byte-identical across upgrade and rollback. GitHub redirects the old links.                                                                                                                |
| `data-product-controller` release profile, old chart and image names, old signer identities | Release verification scripts, tests, docs       | Releases up to v6.0.1 were signed under these names. Their signatures cannot change, so verifying them needs the old identities for as long as those releases are supported.                                                                                   |
| Accepted `devantler-tech/data-product-controller` caller repository                         | `cd.yaml`, AGE verification docs                | The same reason, for artifacts already released from the old repository name.                                                                                                                                                                                  |

Any other occurrence of the old name is not protected by this record. Treat it as ordinary cleanup.

## Consequences

- A rename of a listed identifier is a deliberate migration, with its own issue and upgrade and
  rollback evidence. It is not a text replacement.
- Readers see both names in the repository, and this record explains why.
- Removing the old release profile or signer identities also ends verification of those releases.
  Decide that only when they are no longer supported.
- Adopting the renamed artifacts in Platform stays with
  [platform#4895](https://github.com/devantler-tech/platform/issues/4895). This record changes
  nothing that is deployed.
