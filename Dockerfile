FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/key-keeper ./cmd/key-keeper

FROM alpine:3.21

RUN adduser -D -H -u 10001 appuser

WORKDIR /app

COPY --from=builder /out/key-keeper /app/key-keeper
COPY config/config.yaml /app/config/config.yaml

USER appuser

EXPOSE 8080

ENV KEY_KEEPER_CONFIG=/app/config/config.yaml

ENTRYPOINT ["/app/key-keeper"]
