# Released v8.0.0 schema

`khoist-v8.0.0-crd.raw` is the unmodified CRD extracted from the signed chart
`ghcr.io/devantler-tech/charts/khoist@sha256:d9eb5e1c3f16113d5e513110683c5158ca3aa03afd24d643e2ff59dd67160d61`.
Its SHA256 is `3ca99ab0f379c7ebef049a3e26c811b64efebead88b0f17ca10c79f96e9e4da0`,
also enforced before the owned API server starts.

Cosign verified issuer `https://token.actions.githubusercontent.com`, repository
`devantler-tech/khoist`, ref `refs/tags/v8.0.0`, source
`f8025c710b76d3809870176cf1fcd0c75ff8ff32`, exact chart digest, and signer
`https://github.com/devantler-tech/khoist/.github/workflows/publish-chart.yaml@refs/tags/v8.0.0`.
Chart name, version and appVersion were `khoist`, `8.0.0`, `8.0.0`. The extracted
bytes also match that release's canonical generated CRD. Do not regenerate this
released fixture from the candidate API types.

The raw capture is intentionally not a YAML formatting target: its exact released
bytes are part of this test's provenance. The test checks its checksum, decodes
the YAML and installs it on a real owned API server. Current generated CRD copies
retain their normal generation, validation and identity checks.
