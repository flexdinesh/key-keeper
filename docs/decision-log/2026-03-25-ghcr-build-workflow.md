---
title: Publish container image to GHCR from GitHub Actions
description: Build on PRs, publish from main, document GHCR-based usage.
date: 2026-03-25
slug: ghcr-build-workflow
status: implemented
tags:
  - ci
  - docker
  - ghcr
  - docs
related_paths:
  - .github/workflows/build.yml
  - README.md
  - examples/with-caddy/compose.yml
---

## Why

The repo is moving to a public GitHub workflow and public image distribution path.

Users need a registry image path for normal usage instead of building from repo source.

## What

Add a GitHub Actions workflow that builds the image on pull requests and publishes `ghcr.io/flexdinesh/key-keeper` on pushes to `main`.

Document GHCR pull/run usage and switch the public compose example to consume the published image.

Keep the existing deploy flow separate in this session.

## How

Use Docker Buildx in GitHub Actions.

On pull requests, run build-only validation.

On pushes to `main`, log in to GHCR with `GITHUB_TOKEN` and push `latest` plus `sha-<shortsha>` tags.

Point the example compose stack at the GHCR image and align the example auth port with the mounted config example.

## Tradeoffs

`latest` keeps docs simple but is not immutable.

Keeping deploy unchanged avoids scope creep, but deploy and image publish stay decoupled.

## Assumptions

GHCR package path is `ghcr.io/flexdinesh/key-keeper`.

Package visibility may still need to be set public in GitHub after first publish.
