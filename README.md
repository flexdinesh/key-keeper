# key-keeper

`key-keeper` is a super lightweight auth gateway for API key validation.
It is built to sit beside a reverse proxy like Caddy and decide auth before traffic reaches your upstream.

## Where it works best

- Config-driven rules and keys loaded once at startup.
- YAML config only. No database.
- Rules and keys stay in memory.
- No dynamic config or key updates during runtime. Restart to apply changes.
- Built for Caddy `forward_auth`, but works with any server that forwards the original host and URI.

## Behavior

- `200` when auth succeeds.
- `401` when a required header is missing.
- `403` when the header value is wrong.
- `403` when no rule matches. Deny by default.

## Configuration

Config values can use `${ENV_VAR}` placeholders. `rules[].validators[].value` must be base64 input. `key-keeper` resolves `${ENV_VAR}` first, then base64-decodes the final value during config load.

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
```

Set `KEY_KEEPER_CONFIG` if you want another path. If unset, the binary default is `config/config.yaml`. The container image sets `KEY_KEEPER_CONFIG=/app/config/config.yaml` but does not include a config there, so mount your config file to `/app/config/config.yaml` or point `KEY_KEEPER_CONFIG` at another mounted path. Container startup fails if no config file is mounted.

For the full config contract, see `docs/configuration.md`.

## Examples

### Docker Compose with Caddy

`docker-compose.yml`

```yaml
services:
  key-keeper:
    image: ghcr.io/flexdinesh/key-keeper:latest
    environment:
      # actual value: change-me
      OTEL_INGEST_API_KEY_BASE64: Y2hhbmdlLW1l
    volumes:
      - ./config.yaml:/app/config/config.yaml:ro
    expose:
      - "8181"

  caddy:
    image: caddy:2.10
    depends_on:
      - key-keeper
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy_data:/data
      - caddy_config:/config

volumes:
  caddy_data:
  caddy_config:
```

`Caddyfile`

```caddy
{
	auto_https disable_redirects
}

ingest-grpc.example.com {
	forward_auth key-keeper:8181 {
		uri /auth
	}

	reverse_proxy otel-backend:4317
}

ingest-http.example.com {
	forward_auth key-keeper:8181 {
		uri /auth
	}

	reverse_proxy otel-backend:4318
}
```

Copy `examples/with-caddy/config.yaml`, change hosts, paths, and keys, then mount it as `./config.yaml`. Replace `otel-backend` with your actual upstream service. Runnable refs: `examples/with-caddy/compose.yml`, `examples/with-caddy/Caddyfile`, `examples/with-caddy/config.yaml`.
