# Require mounted config for container startup

## Brief summary

Keep `docker build` and GHCR publishing generic, but stop baking `config/config.yaml` into the image so container startup fails unless a real config is mounted at `/app/config/config.yaml` or `KEY_KEEPER_CONFIG` points to another mounted path.

## Key implementation changes

- Remove `COPY config/config.yaml /app/config/config.yaml` from `Dockerfile`.
- Keep `ENV KEY_KEEPER_CONFIG=/app/config/config.yaml` in `Dockerfile`.
- Leave `cmd/key-keeper/main.go` repo-local fallback as-is so source runs still work; enforce missing-config failure only for containers.
- Update `README.md`, `docs/configuration.md`, and container examples to say the image no longer includes a usable config and startup fails without a mount.

## Tests or verification

- Run `go test ./...`.
- Build image with `docker build`.
- Run container without config mount; expect non-zero exit and config load error.
- Run container with mounted config and required env vars; expect server start.

## Decisions made by user

- Chose start-time-only enforcement.
- Keep generic image builds and current GHCR workflow shape.
- Enforce missing-config failure at container start, not at image build.

## Tradeoffs and risks discussed

- Missing config is caught later at `docker run` or deploy start, not at `docker build`.
- Local repo runs can still succeed via `config/config.yaml`.
- Docs must be aligned or users may still expect a bundled config.

## Remaining open questions

- None.

## Execution guidance

If implementation deviates from this plan, update the saved plan file to reflect the latest approved plan and surface the deviation to the user before continuing.
