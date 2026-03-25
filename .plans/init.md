# Config-Driven Auth Gateway For Caddy `forward_auth`

## Summary
Build a small Go HTTP auth gateway that Caddy calls via `forward_auth` before proxying traffic upstream. Initial use is protecting OTel ingress, but the design should stay generic for similar header-auth proxy cases. The service will not expose a native gRPC API in v1; instead, it will authorize both HTTP and proxied gRPC traffic through a normal HTTP auth endpoint using Caddy-forwarded request metadata.

The service will load an immutable YAML config at startup, resolve `${ENV_VAR}` placeholders for secret values, match requests by forwarded host plus path, and require exact header matches. If no rule matches, deny by default. On success return `200`. On failure return `401` for missing required auth headers and `403` for present-but-wrong values.

## Key Changes
### Service behavior
- Expose a single HTTP auth endpoint, e.g. `GET/POST /auth`, intended only for Caddy `forward_auth`.
- Read request identity from Caddy headers:
  - `X-Forwarded-Host` for the original host
  - `X-Forwarded-Uri` for the original path/query
  - request headers for validator checks such as `x-api-key`
- Parse the forwarded URI and evaluate rules against:
  - host matcher: exact host or wildcard suffix like `*.customer.example.com`
  - path matcher: exact or prefix
- Evaluate rules in deterministic order as declared in config; first matching rule wins.
- For the selected rule, require all configured header validators to match exactly.
- Return:
  - `200 OK` when the rule matches and all validators pass
  - `401 Unauthorized` when the rule matches but one or more required headers are missing
  - `403 Forbidden` when required headers are present but any value is wrong
  - `403 Forbidden` when no rule matches, since policy is deny-by-default
- Keep success responses empty/minimal and do not inject extra identity headers in v1.

### Config and interfaces
Define a YAML config shaped roughly like:

```yaml
server:
  listen_addr: ":8080"

rules:
  - name: otel-grpc-ingest
    host:
      type: exact # or wildcard_suffix
      value: ingest-grpc.example.com
    path:
      type: prefix # or exact
      value: /opentelemetry.proto.collector.trace.v1.TraceService/
    validators:
      - type: header_exact
        header: x-api-key
        value: ${OTEL_INGEST_API_KEY}
```

Public/config contract decisions:
- Config file is YAML only.
- Secret values may be embedded literally or referenced with `${ENV_VAR}` placeholders; placeholders are resolved once at startup.
- Header names should be matched case-insensitively using canonical HTTP handling.
- Path matching should ignore query string for rule selection unless there is a concrete need to include it; use the parsed URI path only.
- Wildcard host support is suffix-based only, not full glob/regex.
- No runtime config reload in v1; changes apply on container restart.

### Implementation structure
- Go module with a small internal split:
  - config loading and env interpolation
  - matcher evaluation
  - validator execution
  - HTTP handler / response mapping
- Prefer a straightforward net/http server.
- Add structured logging for:
  - startup config summary without secret values
  - match result and deny reason
  - malformed forwarded metadata
- Keep config in memory as an immutable validated snapshot built at startup.
- Fail fast on startup for invalid config, unresolved required env placeholders, duplicate/ambiguous rule definitions if validation deems them unsafe.

### Container and deployment example
- Multi-stage Dockerfile:
  - builder stage compiles a static Go binary
  - runtime stage uses a minimal base image
- Example `docker-compose.yml` should include:
  - the auth service
  - a Caddy service configured with `forward_auth`
- Example Caddy config should:
  - terminate TLS for `ingest-grpc.example.com`
  - call the auth service before proxying
  - forward the original request metadata needed by the auth service
  - proxy authorized traffic to the OTel backend
- Include sample env wiring for `OTEL_INGEST_API_KEY` and mounted YAML config.

## Test Plan
- Unit tests for config parsing:
  - valid YAML with literal values
  - `${ENV_VAR}` interpolation
  - missing env var failure
  - invalid matcher types or empty validator values
- Unit tests for matching:
  - exact host match
  - wildcard suffix host match
  - exact path match
  - prefix path match
  - first-match-wins ordering
  - no-match deny
- Handler tests:
  - missing `X-Forwarded-Host` or `X-Forwarded-Uri`
  - matching rule with missing auth header returns `401`
  - matching rule with wrong header value returns `403`
  - matching rule with correct headers returns `200`
  - header name case-insensitivity
  - query string does not affect path matching
- Compose-level smoke scenario:
  - request to Caddy without `x-api-key` is denied
  - request with wrong key is denied
  - request with correct key is forwarded to backend
  - gRPC/OTLP path example is matched through forwarded URI

## Assumptions
- Caddy will call this service using `forward_auth`, and the auth service should trust `X-Forwarded-Host` and `X-Forwarded-Uri` as the canonical route identity.
- v1 only needs HTTP auth evaluation; native gRPC server support is out of scope.
- Rule matching uses forwarded host and URI path, not a custom “original URL” header.
- Unmatched requests should be denied rather than passed through.
- Dynamic config updates are intentionally out of scope for v1; “immutable config” means restart-to-apply.
