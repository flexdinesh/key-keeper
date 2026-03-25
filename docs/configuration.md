# Configuration

`key-keeper` is a small config-driven auth gateway for proxy `forward_auth` flows. Current use is OpenTelemetry ingress, but the config model stays generic: match forwarded host/path, then validate request headers.

`key-keeper` loads a single YAML file at startup. The path is read from `KEY_KEEPER_CONFIG`. If that env var is not set, the server uses `config/config.yaml`.

Config is immutable in the current implementation. To apply changes, update the YAML or referenced env vars and restart the service. Config values can interpolate environment variables with `${ENV_VAR}` placeholders. Validator `value` fields are base64 input; `key-keeper` resolves `${ENV_VAR}` first, then base64-decodes the final value during config load.

## Container image

The published image `ghcr.io/flexdinesh/key-keeper` includes a default config at `/app/config/config.yaml` and sets `KEY_KEEPER_CONFIG` to that path.

Use the bundled config:

```bash
docker run --rm \
  -p 8181:8181 \
  -e OTEL_INGEST_API_KEY_BASE64=Y2hhbmdlLW1l \
  ghcr.io/flexdinesh/key-keeper:latest
```

Override the config by mounting your file onto the default path:

```bash
docker run --rm \
  -p 8181:8181 \
  -e OTEL_INGEST_API_KEY_BASE64=Y2hhbmdlLW1l \
  -v "$(pwd)/config.yaml:/app/config/config.yaml:ro" \
  ghcr.io/flexdinesh/key-keeper:latest
```

## Top-level shape

```yaml
server:
  listen_addr: ":8080"
  shutdown_timeout: 5s
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

## Server section

### `server.listen_addr`

- Type: string
- Default: `:8080`
- Purpose: TCP address the HTTP auth service listens on

Examples:

```yaml
server:
  listen_addr: ":8080"
```

```yaml
server:
  listen_addr: "0.0.0.0:9000"
```

### `server.shutdown_timeout`

- Type: Go duration string
- Default: `5s`
- Purpose: graceful shutdown timeout after `SIGTERM` or `SIGINT`

Examples:

```yaml
server:
  shutdown_timeout: 5s
```

```yaml
server:
  shutdown_timeout: 30s
```

### `server.access_log.format`

- Type: string
- Default: `json`
- Supported values:
  - `json`
  - `logfmt`
- Purpose: selects the access log output format for auth requests

`json` writes structured key/value access logs using the JSON logger.

```yaml
server:
  access_log:
    format: json
```

`logfmt` writes a single logfmt-style line in the log message field.

```yaml
server:
  access_log:
    format: logfmt
```

## Rules section

`rules` is an ordered list. Rules are evaluated from top to bottom, and the first matching rule wins.

Each rule requires:

- `name`
- `host`
- `paths`
- at least one `validators` entry

If no rule matches, the request is denied.

### `rules[].name`

- Type: string
- Required: yes
- Purpose: log-friendly rule identifier

Example:

```yaml
rules:
  - name: otel-http-traces
```

## Host matcher

### `rules[].host.type`

- Type: string
- Required: yes
- Supported values:
  - `exact`
  - `wildcard_suffix`

`exact` matches a single host.

```yaml
host:
  type: exact
  value: ingest-grpc.example.com
```

`wildcard_suffix` matches any host that ends with the configured suffix. The value must start with `*.`.

```yaml
host:
  type: wildcard_suffix
  value: "*.customer.example.com"
```

Notes:

- Host matching uses `X-Forwarded-Host`.
- Matching is case-insensitive.
- An optional port on the incoming host, such as `:443`, is ignored.
- A trailing `.` on the incoming host is ignored.

### `rules[].host.value`

- Type: string
- Required: yes
- Purpose: host value used by the selected host matcher type

Examples:

```yaml
host:
  type: exact
  value: ingest-http.example.com
```

```yaml
host:
  type: wildcard_suffix
  value: "*.customer.example.com"
```

## Path matchers

`paths` is a list. A rule matches when any one of its configured path matchers matches the incoming request path.

### `rules[].paths[].type`

- Type: string
- Required: yes
- Supported values:
  - `exact`
  - `prefix`

`exact` matches a single path.

```yaml
paths:
  - type: exact
    value: /v1/traces
```

`prefix` matches any request path that starts with the configured value.

```yaml
paths:
  - type: prefix
    value: /opentelemetry.proto.collector.trace.v1.TraceService/
```

Notes:

- Path matching uses `X-Forwarded-Uri`.
- Only the URI path is matched. Query strings are ignored.
- Path values must start with `/`.

### `rules[].paths[].value`

- Type: string
- Required: yes
- Purpose: path value used by the selected path matcher type

Examples:

```yaml
paths:
  - type: exact
    value: /v1/metrics
```

```yaml
paths:
  - type: prefix
    value: /v1/traces
```

## Validators

`validators` is a list of checks for the selected rule. All validators must pass.

### `rules[].validators[].type`

- Type: string
- Required: yes
- Supported values:
  - `header_exact`

This checks that a request header exists and exactly matches the decoded configured value.

```yaml
validators:
  - type: header_exact
    header: x-api-key
    # actual value from OTEL_INGEST_API_KEY_BASE64: change-me
    value: ${OTEL_INGEST_API_KEY_BASE64}
```

### `rules[].validators[].header`

- Type: string
- Required: yes
- Purpose: request header name to validate

Examples:

```yaml
header: x-api-key
```

```yaml
header: authorization
```

Notes:

- Header lookup is case-insensitive.
- Missing required headers produce `401 Unauthorized`.

### `rules[].validators[].value`

- Type: string
- Required: yes
- Purpose: base64-encoded exact expected header value

Examples:

Literal base64 value:

```yaml
# actual value: hardcoded-secret
value: aGFyZGNvZGVkLXNlY3JldA==
```

Environment placeholder:

```yaml
# actual value from OTEL_INGEST_API_KEY_BASE64: change-me
value: ${OTEL_INGEST_API_KEY_BASE64}
```

Notes:

- Present but wrong values produce `403 Forbidden`.
- Invalid base64 values fail during config load.
- Values that decode to empty fail during config load.

## Environment variable placeholders

Config values can include placeholders in `${ENV_VAR_NAME}` format. `key-keeper` resolves them before YAML parsing completes.

Example:

```yaml
validators:
  - type: header_exact
    header: x-api-key
    # actual value from OTEL_INGEST_API_KEY_BASE64: change-me
    value: ${OTEL_INGEST_API_KEY_BASE64}
```

With:

```bash
# actual value: change-me
export OTEL_INGEST_API_KEY_BASE64=Y2hhbmdlLW1l
```

Behavior:

- If the env var exists, the placeholder is replaced with its value.
- Validator values are then base64-decoded during config load.
- If the env var is missing, startup fails.
- If a validator value is not valid standard base64, startup fails.
- Unpadded standard base64 is accepted.
- If a validator value decodes to empty, startup fails.
- Placeholders currently support env var names matching `[A-Z0-9_]+`.

## Request matching behavior

The auth endpoint expects Caddy `forward_auth` requests and reads:

- `X-Forwarded-Host`
- `X-Forwarded-Uri`

The selected rule is determined by:

1. host matcher
2. any configured path matcher
3. first matching rule in declaration order

After a rule matches, all validators on that rule must pass.

## Response behavior

Current status mapping:

- `200 OK`: matched rule and all validators passed
- `401 Unauthorized`: matched rule but a required header was missing
- `403 Forbidden`: matched rule but a header value was wrong
- `403 Forbidden`: no configured rule matched the request
- `400 Bad Request`: `X-Forwarded-Host` or `X-Forwarded-Uri` missing or invalid

## Access logs

One access log record is written for every request handled by the service, including `/auth` and `/healthz`.

The access log includes:

- request method
- `X-Forwarded-Host`
- `X-Forwarded-Uri`
- handler path
- response status
- auth decision
- matched rule, when present
- deny reason, when present
- validator header, when present
- validator value marker: `[REDACTED]`, `[EMPTY]`, or `[MISSING]`
- duration in milliseconds
- remote address
- first `X-Forwarded-For` value, when present
- user agent
- content length
- `X-Forwarded-Proto`

### Example JSON access log

```json
{
  "time": "2026-03-12T10:15:30Z",
  "level": "INFO",
  "msg": "auth access",
  "method": "GET",
  "forwarded_host": "ingest-grpc.example.com",
  "forwarded_uri": "/v1/traces",
  "path": "/auth",
  "status": 200,
  "decision": "allowed",
  "rule": "otel-grpc-traces",
  "reason": "",
  "validator_header": "x-api-key",
  "validator_value": "[REDACTED]",
  "duration_ms": 1
}
```

Example denied JSON access log:

```json
{
  "time": "2026-03-12T10:15:31Z",
  "level": "INFO",
  "msg": "auth access",
  "method": "POST",
  "forwarded_host": "ingest-http.example.com",
  "forwarded_uri": "/v1/logs",
  "path": "/auth",
  "status": 403,
  "decision": "denied_wrong_value",
  "rule": "strict-ingest",
  "reason": "invalid value for header x-api-key",
  "validator_header": "x-api-key",
  "validator_value": "[REDACTED]",
  "duration_ms": 0,
  "remote_addr": "172.19.0.4:48122",
  "forwarded_for": "203.0.113.10",
  "user_agent": "otel-collector/0.102.1",
  "content_length": 0,
  "forwarded_proto": "https"
}
```

### Example logfmt access log

```text
msg="auth access" method=GET forwarded_host=ingest-grpc.example.com forwarded_uri=/v1/traces path=/auth status=403 decision=denied_wrong_value rule=otel-grpc-traces reason="invalid value for header x-api-key" validator_header=x-api-key validator_value=[REDACTED] duration_ms=0 remote_addr=127.0.0.1:12345 forwarded_for=- user_agent="otel client" content_length=0 forwarded_proto=-
```

Example allowed logfmt access log:

```text
msg="auth access" method=POST forwarded_host=ingest-http.example.com forwarded_uri=/v1/traces path=/auth status=200 decision=allowed rule=otel-http-traces reason=- validator_header=x-api-key validator_value=[REDACTED] duration_ms=1 remote_addr=172.19.0.4:48120 forwarded_for=203.0.113.10 user_agent=otel-collector/0.102.1 content_length=0 forwarded_proto=https
```

## Complete examples

### Example: HTTP OTLP endpoint

```yaml
server:
  listen_addr: ":8080"
  shutdown_timeout: 5s

rules:
  - name: otel-http-traces
    host:
      type: exact
      value: ingest-http.example.com
    paths:
      - type: prefix
        value: /v1/traces
    validators:
      - type: header_exact
        header: x-api-key
        # actual value from OTEL_INGEST_API_KEY_BASE64: change-me
        value: ${OTEL_INGEST_API_KEY_BASE64}
```

### Example: gRPC OTLP endpoint

```yaml
server:
  listen_addr: ":8080"

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

### Example: wildcard tenant host

```yaml
rules:
  - name: tenant-metrics
    host:
      type: wildcard_suffix
      value: "*.customer.example.com"
    paths:
      - type: exact
        value: /v1/metrics
    validators:
      - type: header_exact
        header: x-api-key
        # actual value: tenant-secret
        value: dGVuYW50LXNlY3JldA==
```

### Example: multiple required headers

```yaml
rules:
  - name: strict-ingest
    host:
      type: exact
      value: ingest-http.example.com
    paths:
      - type: prefix
        value: /v1/logs
      - type: exact
        value: /v1/metrics
    validators:
      - type: header_exact
        header: x-api-key
        # actual value from OTEL_INGEST_API_KEY_BASE64: change-me
        value: ${OTEL_INGEST_API_KEY_BASE64}
      - type: header_exact
        header: x-tenant-id
        # actual value: tenant-a
        value: dGVuYW50LWE=
```

## Invalid config examples

Unsupported matcher type:

```yaml
host:
  type: regex
  value: ".*"
```

Invalid wildcard host:

```yaml
host:
  type: wildcard_suffix
  value: example.com
```

Invalid path value:

```yaml
paths:
  - type: prefix
    value: v1/traces
```

Missing validators:

```yaml
rules:
  - name: invalid-rule
    host:
      type: exact
      value: ingest-http.example.com
    paths:
      - type: exact
        value: /v1/traces
    validators: []
```
