---
title: Use safe log format for validator values
description: Logs keep header-state signal while never printing sensitive validator values.
date: 2026-03-25
slug: log-format-redaction
status: implemented
tags:
  - logging
  - security
related_paths:
  - internal/httpserver/handler.go
  - internal/auth/evaluator.go
  - docs/configuration.md
---

## Why

Logs need to show whether a validator header was sent, but must not print sensitive values.

## What

Use `validator_value` as a state marker, not raw data.

Markers:

- `[REDACTED]` for present non-empty value
- `[EMPTY]` for present empty value
- `[MISSING]` for absent header

## How

Track header presence separately from header value during auth evaluation.

Emit marker values in auth logs and access logs. Keep `validator_header` unchanged.
