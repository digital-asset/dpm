Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
SPDX-License-Identifier: Apache-2.0

# Dpm

This repository hosts the code for `dpm`, the package manager that serves as the entry point for interacting with components developed by [Digital Asset](https://www.digitalasset.com/).

Documentation for `dpm` is included in the overall docs for the Digital Asset SDK at [docs.canton.network](https://docs.canton.network/appdev/tooling/development-tools-overview#dpm-daml-package-manager). Command usage (`dpm install`, `dpm add dar`, `dpm update`, and so on) is documented there and in the CLI `--help` text.

`dpm` itself is:

1. A simple package manager for DARs, components, and templates
2. A build tool: project configuration (remote DARs/remote components)
3. An extensible launcher for arbitrary subcommands:
    a. Unified CLI interface for local development
    b. Unified CLI interface for remote managment
4. Publisher for components/DARs
5. Supports airgapped installation

## Git-based DAR dependencies

Projects can declare prebuilt DARs hosted in a Git repository under `dependencies` in `daml.yaml`:

```yaml
artifact-locations:
  "@example-repo":
    url: "git:github.com/example-org/example-repo.git"

dependencies:
  - daml-prim
  - daml-stdlib
  - "@example-repo#main?path=packages/foo.dar"
  # Or without an alias:
  # - git:github.com/example-org/example-repo.git#main?path=packages/foo.dar

data-dependencies:
  - "@example-repo#main?path=packages/bar.dar"
```

- **Syntax:** `git:<host>/<repo>#<ref>?path=<repo-relative.dar>`, or the same `#<ref>?path=…` suffix on an `@alias` whose `artifact-locations` URL is a `git:` repo. An artifact location supplies **only** the bare repo URL — the `#<ref>` and `?path=`/`?release=` always live on the dependency line (a location URL that carries a `#ref` or query is rejected).
- **Hosts:** any git host reachable over HTTPS, including GitLab, Bitbucket, and self-hosted servers — for example `git:gitlab.com/example-org/example-repo#main?path=packages/foo.dar`. URLs copied from a host's web UI are also accepted and rewritten to the canonical form, covering both the GitHub `…/blob/<ref>/<file>` and GitLab `…/-/blob/<ref>/<file>` layouts.
- **Pinning:** `dpm install package` / `dpm update` pin branch or tag refs to commit SHAs in `daml.yaml`. The `@alias` is authoring-time shorthand: once pinned, the line is rewritten to the full `git:<host>/<repo>#<sha>?path=…` form.
- **Releases:** release assets are supported in either dependency field via `?release=<tag>&asset=<file>.dar` (on a `git:` line or an `@alias`). Omitting `asset` expands the entry to all `.dar` assets. Unlike the `#<ref>?path=…` form, this is **github.com only**, because it reads the GitHub releases API; on other hosts, depend on a committed `.dar` with `#<ref>?path=…` instead.
- **Fields:** Git DARs work under both `dependencies` and `data-dependencies`; pinning rewrites the field where the entry was declared.
- **Limits:** HTTPS clone URLs only (no SSH), and public repositories only (no credential handling for private clones).

## Contributing

We warmly welcome contributions. See [the contributing guidelines](./CONTRIBUTING.md) for more information.
