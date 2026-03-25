---
title: Base64 validator secrets
description: Store validator secrets as base64 input, including env-interpolated values
date: 2026-03-25
slug: base64-validator-secrets
status: implemented
tags:
  - config
  - secrets
  - docs
related_paths:
  - internal/config/config.go
  - internal/config/config_test.go
  - docs/configuration.md
  - README.md
---

## Why

Plaintext validator secrets in config and env were too direct. Base64 keeps operator input consistent across YAML literals and `${ENV_VAR}` placeholders, while leaving runtime auth behavior unchanged.

## What

`rules[].validators[].value` is now base64 input everywhere.

`${ENV_VAR}` placeholders still interpolate in YAML config.

After interpolation, `key-keeper` base64-decodes validator values during config load.

Accepted input is standard base64 with or without padding.

Startup fails for missing env vars, invalid base64, or decoded empty validator values.

## How

Decode in config load, before final validation, so auth evaluation still compares raw request header values against decoded secrets.

Keep the existing `value` field instead of adding a new config shape.

Document the contract in README, config docs, sample config, compose examples, and tests.

Add plaintext comments above encoded sample values in docs, examples, and tests to keep encoded fixtures readable.

## Tradeoffs

This is a breaking config semantic change for literal plaintext validator values; they must now be converted to base64.

Keeping the same field avoids schema churn, but makes docs and error messages more important.

## Gotchas

Interpolation happens before base64 decode.

Docs/examples use `OTEL_INGEST_API_KEY_BASE64` to make the expected env content explicit.
