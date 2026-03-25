# key-keeper

`key-keeper` is a small Go auth gateway for proxy `forward_auth` flows. Current use is protecting OTLP HTTP and proxied gRPC ingress before requests reach an OpenTelemetry backend. Direction stays generic and config-driven for similar proxy-auth cases.

## Configuration

Set `KEY_KEEPER_CONFIG` to the YAML config path. Config values can interpolate environment variables with `${ENV_VAR}` placeholders. Every `rules[].validators[].value` must be base64 input; `key-keeper` resolves `${ENV_VAR}` first, then base64-decodes the final value during config load.

```yaml
server:
  listen_addr: ":8080"
  access_log:
    format: json

rules:
  - name: otel-grpc-traces
    host:
      type: exact
      value: ingest-grpc.example.com
    paths:
      - type: prefix
        value: /opentelemetry.proto.collector.trace.v1.TraceService/
    validators:
      - type: header_exact
        header: x-api-key
        # actual value from OTEL_INGEST_API_KEY_BASE64: change-me
        value: ${OTEL_INGEST_API_KEY_BASE64}
```

## Running locally

```bash
# actual value: change-me
export OTEL_INGEST_API_KEY_BASE64=Y2hhbmdlLW1l
go test ./...
go run ./cmd/key-keeper
```

## Container image

Published image: `ghcr.io/flexdinesh/key-keeper`

```bash
# actual value: change-me
docker pull ghcr.io/flexdinesh/key-keeper:latest
docker run --rm \
  -p 8181:8181 \
  -e OTEL_INGEST_API_KEY_BASE64=Y2hhbmdlLW1l \
  ghcr.io/flexdinesh/key-keeper:latest
```

The image already includes the default config at `/app/config/config.yaml`. For custom config, mount your file to that path or set `KEY_KEEPER_CONFIG` to another mounted path.

## Compose example

Use [`examples/compose.yml`](./examples/compose.yml) with [`examples/Caddyfile`](./examples/Caddyfile) to run the published GHCR image in front of OTLP HTTP and gRPC ingress. Replace `otel-backend` with your actual collector or backend service name.

The repo-root [`compose.dev.yml`](./compose.dev.yml) stays for the local repo-driven deploy flow.
