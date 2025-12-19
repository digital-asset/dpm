Copyright (c) 2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
SPDX-License-Identifier: Apache-2.0

# Contributing to the dpm repository

This page gives a high-level overview of how to contribute to the development of `dpm`.

## Development documentation

Developers working on the internals of `dpm`, or are working on developing components can run `make run-internal-docs` to view more documentation. Note that this documentation is intended for contributors to `dpm`, and is _not_ part of the public API of `dpm`.

## Issues

Features and bugs are tracked in GitHub issues in this project.

## Testing

All new features should include tests as part of the development process. To run tests, run `go test -v ./...`.

