# Historical v8 publisher reports

The three `khoist-v8-*-preflight-v2.json` files are actual JSON output from the
v8.0.0 `product-check` executable, built from its release source
`f8025c710b76d3809870176cf1fcd0c75ff8ff32`. The UI protocol fixtures contain one
`with-ui` declaration with protocol v1 or v2 and a 200-character `ø` title. The
composition fixture contains `a-consumer` depending on `z-producer`, with UI v2
on the consumer. All URLs, identities and source values are synthetic public data.

They were captured with `PUBLISHER_PREFLIGHT_ENABLED=true product-check --file
INPUT --format json --report-version v2`. These saved reports exercise retained
historical readers; do not regenerate them with the current create schema, which
correctly rejects new UI. Browser URL/contract mutations occur only in these
historical report objects and still pass the real report reader's validation.

SHA256:

- protocol v1: `199935d42cb49cf257a7a3bc82ff7f4d17c48a04e9209f7a9a35f83811132f89`
- protocol v2: `fee9034668b3a35faea914363a8719b30e6e08e6eb9253438be49aa54dc151bd`
- legacy composition: `fff6413b56991bfea203e4ae0dedfb05439be582b0b9019a83016eb2df6021b2`
