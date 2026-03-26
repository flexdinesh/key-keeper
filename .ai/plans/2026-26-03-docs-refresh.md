# Docs refresh for key-keeper

## Brief summary

Make `README.md` the crisp landing doc. Trim `docs/configuration.md` into a short reference. Keep both centered on the narrow contract: lightweight API key auth gateway, best beside a reverse proxy like Caddy, immutable YAML config loaded into memory.

## Key implementation changes

### README.md

- Replace intro with 2 lines on why: super lightweight auth gateway for API key validation, meant to sit alongside a reverse proxy like Caddy.
- Add a "Where it works best" section: config-driven rules and keys in memory, YAML config only, no DB, no dynamic config or key updates at runtime.
- Add a concise behavior note: built for Caddy `forward_auth`, usable with any server, returns `200` on success, `401` when a required header is missing, and `403` when the key or value is wrong.
- Add a brief config section with one accurate YAML example.
- Inline examples directly in the README:
  - Docker Compose example with Caddy reverse proxy
  - Matching `Caddyfile` snippet
  - Direct `docker run` example
- In the Compose example, mount the config file into `/app/config/config.yaml`.
- Make container examples explicitly bring their own mounted config file. Do not imply the bundled sample config is enough for real use.
- Keep links to `examples/with-caddy/compose.yml`, `examples/with-caddy/Caddyfile`, and `examples/with-caddy/config.yaml` as runnable references.

### docs/configuration.md

- Trim the long field-by-field doc into a compact reference.
- Keep only: config load path, restart-to-apply model, env interpolation plus base64 validator value contract, supported matcher and validator types, one example config, auth response behavior.

## Tests or verification

- Verify docs against `config/config.yaml`, `examples/with-caddy/compose.yml`, `examples/with-caddy/Caddyfile`, `examples/with-caddy/config.yaml`, `Dockerfile`, and `cmd/key-keeper/main.go`.
- Verify status code claims against `internal/httpserver/handler_test.go`.
- Verify config contract against `internal/config/config.go`.

## Decisions made by user

- Docs must be straight to the point, concise, and precise.
- Lead with the why.
- Emphasize immutable in-memory YAML config.
- No DB.
- No dynamic runtime updates.
- Caddy `forward_auth` first, but not Caddy-only.
- Update both `README.md` and `docs/configuration.md`.
- Inline example content in `README.md`.

## Tradeoffs and risks discussed

- Inline README examples improve scan speed, but duplicated snippets can drift from `examples/`.
- Shorter config docs reduce operator hand-holding, but better fit the project's narrow scope.

## Remaining open questions

- None.

## Execution guidance

If implementation deviates from this plan, update this file to reflect the latest approved plan and surface the deviation to the user before continuing.
