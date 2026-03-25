# key-keeper

## Purpose
- Small Go auth gateway for proxy `forward_auth` flows.
- Current use: protect OTLP HTTP and proxied gRPC ingress with header-based auth before traffic reaches the backend.
- Direction: keep it generic for similar proxy-auth cases, but avoid platform creep.

## Defaults
- Config-driven. Immutable at startup.
- Deny by default.
- Prefer simple HTTP auth flow over native gRPC unless forced.
- Keep scope narrow, behavior explicit, ops simple.

## Structure

```text
.
|-- cmd/
|   `-- key-keeper/      # process entrypoint only
|-- internal/
|   |-- auth/            # rule matching + auth decisions
|   |-- config/          # config schema/load/validation/env expansion
|   `-- httpserver/      # HTTP handler, request parsing, response/log mapping
|-- config/              # sample/default runtime config
|-- docs/                # operator and user docs
|-- examples/            # integration examples, eg Caddy + compose
|-- .plans/              # project plans and direction notes
|-- README.md            # quick start + project brief
`-- compose.yml          # local stack example
```

- Keep `cmd/` thin. Wire deps there, not logic.
- Put auth rules and match behavior in `internal/auth`.
- Put config shape, loading, validation, env expansion in `internal/config`.
- Put HTTP-only concerns in `internal/httpserver`.
- Put runnable examples in `examples/`, not `docs/`.
- Update `AGENTS.md`, `README.md`, and `.plans/init.md` together if purpose or direction changes.

## Non-goals
- No runtime config reload unless clearly needed.
- No native gRPC auth server unless HTTP auth flow stops being enough.
- No extra identity/header injection unless required by a real upstream.
