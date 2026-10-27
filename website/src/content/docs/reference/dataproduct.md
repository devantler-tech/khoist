---
title: DataProduct
description: Find the versioned Kubernetes contract and the fields a publisher needs.
---

`apiVersion: data.devantler.tech/v1alpha1` and `kind: DataProduct` identify the resource.
The [delivered CRD](https://github.com/devantler-tech/khoist/blob/main/config/crd/bases/data.devantler.tech_dataproducts.yaml)
is the field and admission reference. Kubernetes can show the installed version:

```bash
kubectl explain dataproduct.spec --api-version=data.devantler.tech/v1alpha1
```

## Publisher fields

| Field                                  | Purpose                                                      |
| -------------------------------------- | ------------------------------------------------------------ |
| `metadata.name`, `metadata.namespace`  | Kubernetes lookup identity and namespace boundary            |
| `spec.id`                              | Public absolute product identity                             |
| `spec.name`, `spec.description`        | Human-readable public description                            |
| `spec.version`                         | Publisher's product-contract version                         |
| `spec.owner`                           | Public ownership metadata                                    |
| `spec.outputs`                         | Named query interfaces, protocol, HTTPS URL and contract URL |
| `spec.inputs`                          | Explicit same-namespace producer/output references           |
| `spec.source` and its engine selection | Optional externally owned source observation                 |

Use the [complete example](/examples/harbour-product.yaml) for a metadata-only API.
Read [composition](/guides/compose-products/) and the linked source-provider contract
before adding optional observations.

## Public status

Conditions distinguish aggregate readiness, source, connector, contract and composition
observations. Inspect the generation and reason before acting.
KHoist does not place records, credential values or database query results in status.

The delivered CRD rejects new, added or changed `spec.ui`. Unchanged legacy
declarations can still be edited in unrelated fields or have UI removed; the field
is not pruned from stored objects. Consumer-owned applications integrate through
the named output APIs. See the [upgrade guide](/guides/upgrade/).
