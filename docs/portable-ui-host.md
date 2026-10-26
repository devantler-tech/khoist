# Deploy the independent publisher-review host

The signed application image includes `/publisher-review`, an independent offline report host.
Pin the verified release digest and run with the image’s nonroot user, a read-only filesystem and
restricted network access. It shares its transport and health implementation with `/ui-kit`.

Existing `/ui-kit` deployments retain only the publisher-review route. The product-session assets
and runtime are absent. `PUBLISHER_PREFLIGHT_ENABLED=true` enables the page and inert metadata
libraries; legacy presentation and discovery settings have no effect. The root redirects to
`/publisher-review`. See [publisher review](publisher-preflight.md) for the workflow.

## Choose the transport

TLS is the default. Mount a certificate and private key readable by the workload's
nonroot user, then run:

```text
/publisher-review --listen-address 0.0.0.0:8443 \
  --tls-cert /tls/cert.pem --tls-key /tls/key.pem
```

The default listener is `127.0.0.1:8443` when `--listen-address` is omitted. A listener
must name an IP address or `localhost` and a port from 1 to 65535. Use bracketed IPv6
addresses when needed. Missing TLS material fails startup.

When the deployment's trusted Gateway already terminates HTTPS, explicitly choose
an internal HTTP backend:

```text
/publisher-review --http-behind-gateway --listen-address 0.0.0.0:8080
```

This mode requires an explicit listener and rejects either TLS option. The Gateway
must expose a verified HTTPS origin to browsers. Restrict access to the HTTP backend
with the deployment's Service, Gateway and network policy; the command does not
authenticate peers or establish a Gateway trust relationship. Keep the backend
private. Platform owns these resources and their rollout.

## Enable publisher review and observe health

Set `PUBLISHER_PREFLIGHT_ENABLED=true` explicitly. Missing or false disables report pages; malformed
values fail before listening. Both executables deny frames and network connections in their browser
policy and expose no product-session routes.

`GET /healthz` always returns `200` with the three-byte body `ok` followed by a newline,
including when the UI is disabled. `HEAD` returns the same headers without a body.
The endpoint rejects query parameters, request bodies and other methods. It reports
only process availability, supplies no configuration or product readiness, and makes
no external requests. Use it for workload liveness and readiness probes.

The listener limits headers to 16 KiB, header reads to five seconds, complete request
reads to ten seconds, writes to fifteen seconds and idle connections to sixty seconds.

## Verify the packaged host

Run the container smoke against a built or verified immutable release image:

```bash
bash tests/source/ui-kit.sh ghcr.io/devantler-tech/khoist@sha256:<verified-digest>
bash tests/source/publisher-review.sh ghcr.io/devantler-tech/khoist@sha256:<verified-digest>
```

The smoke exercises TLS with a trusted disposable certificate and the internal HTTP
option, publisher activation and withdrawal, retired-route absence under legacy settings,
health and nonroot execution with a read-only filesystem.
It publishes only loopback ports
and deletes its containers and disposable certificate afterward. The required source
integration job runs it against the built application image. Release publication
runs it against the signed image digest after verifying the publisher, tag and source
revision.

The dedicated publisher smoke also checks that unrelated presentation settings
cannot enable legacy routes, that malformed publisher opt-in fails before serving,
and that SIGTERM exits successfully in both transport modes. The shared host allows
ten seconds for active responses to drain before closing remaining connections.

Packaging checks establish release behavior. Platform adoption and deployment-owned cleanup
remain tracked in [platform#4890](https://github.com/devantler-tech/platform/issues/4890).
