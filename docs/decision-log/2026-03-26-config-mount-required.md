---
title: Require mounted config for container startup
description: Keep image builds generic and fail container startup without a mounted config file.
date: 2026-03-26
slug: config-mount-required
status: implemented
tags:
  - docker
  - config
  - runtime
related_paths:
  - Dockerfile
  - README.md
  - docs/configuration.md
---

## Why

The image baked in `config/config.yaml` as a default, but that file was not a meaningful real-world default and could mislead users into thinking the image was self-contained.

## What

Keep container image builds generic.

Do not ship a bundled runtime config in the image.

Require operators to provide a config file mount at container start, using `/app/config/config.yaml` by default or another mounted path via `KEY_KEEPER_CONFIG`.

## How

Remove the Dockerfile copy step for `config/config.yaml`.

Keep `KEY_KEEPER_CONFIG=/app/config/config.yaml` as the container default path.

Rely on startup config load failure when the mounted file is missing, and document that contract in container-facing docs.
