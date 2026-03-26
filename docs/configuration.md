# Configuration

`key-keeper` loads one YAML file at startup.

- Set `KEY_KEEPER_CONFIG` to choose the file.
- If unset, the binary uses `config/config.yaml`.
- The container image sets `KEY_KEEPER_CONFIG=/app/config/config.yaml` but does not include a config there.
- Container startup fails until you mount a config file there or point `KEY_KEEPER_CONFIG` at another mounted file.
- Config is immutable at runtime. Change YAML or env vars, then restart.
- Rules and keys stay in memory. No database.

## How rule evaluation works

- Rules are checked top to bottom.
- The first matching rule wins.
- A rule matches by forwarded host and path.
- All validators on the matched rule must pass.
- If no rule matches, the request is denied.

## Example

```yaml
server:
  listen_addr: ":8181"
  shutdown_timeout: 5s
  access_log:
    format: json

rules:
  - name: otel-http-traces
    host:
      type: exact
      value: ingest-http.example.com
    paths:
      - type: prefix
        value: /v1
    validators:
      - type: header_exact
        header: x-api-key
        # actual value from OTEL_INGEST_API_KEY_BASE64: change-me
        value: ${OTEL_INGEST_API_KEY_BASE64}

  - name: otel-grpc-traces
    host:
      type: exact
      value: ingest-grpc.example.com
    paths:
      - type: prefix
        value: /opentelemetry.proto.collector
    validators:
      - type: header_exact
        header: x-api-key
        # actual value from OTEL_INGEST_API_KEY_BASE64: change-me
        value: ${OTEL_INGEST_API_KEY_BASE64}
```

## Supported types

- Host matcher: `exact`, `wildcard_suffix`
- Path matcher: `exact`, `prefix`
- Validator: `header_exact`

## Secrets and env vars

- Config supports `${ENV_VAR}` placeholders.
- `rules[].validators[].value` must be base64 input.
- Expansion happens first. Base64 decode happens next.
- Startup fails if an env var is missing, base64 is invalid, or the decoded value is empty.

## Request metadata used

- Host matching uses `X-Forwarded-Host`.
- Path matching uses `X-Forwarded-Uri`.
- Query strings are ignored for path matching.
- Header names are matched case-insensitively.

## Auth responses

- `200 OK`: matched rule and all validators passed
- `401 Unauthorized`: matched rule but a required header was missing
- `403 Forbidden`: matched rule but a header value was wrong
- `403 Forbidden`: no configured rule matched
