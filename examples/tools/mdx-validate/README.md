# mdx-validate

Validates Mintlify MDX documentation files. The dpm component complement
to `rst-to-mdx`: where the converter emits MDX, `mdx-validate` checks the
MDX in tree before it ships.

- **Frontmatter — `title:` required.** Errors on MDX files that have no
  YAML frontmatter, that have frontmatter without a `title:` field, or
  whose `title:` value is empty.
- **Images** - Checks that image references have a matching image.
- **Snippets are skipped.** Files under any `snippets/` directory are
  excluded (Mintlify reusable snippets don't have frontmatter by design).
- **`--staged` mode.** Validates only `.mdx` files in git's staged
  index — suitable for a pre-commit hook.

Deferred:
- Internal link / anchor checking via `mintlify broken-links` wrapper
- Heading hierarchy warnings
- External link `--check-external` HTTP probes
- Pre-commit/lefthook config snippets

## Usage

`<path>` is a single `.mdx` file or a directory to validate — point it at
your docs tree (for example a `docs-main/` checkout).

```sh
# Validate a specific file or directory
dpm mdx-validate <path>

# With no argument, validates ./docs-main relative to the current directory
# (the default — assumes you run it from a docs repo root)
dpm mdx-validate

# Pre-commit mode: only files in the staged git index
dpm mdx-validate --staged

# Make warnings blocking
dpm mdx-validate --strict <path>
```

## Build

This component lives under `examples/tools/mdx-validate/` in the dpm repo.
It is a standalone Go module targeting **Go 1.21** and the standard library
only.

Run make commands from that directory:

```sh
make build      # builds ./mdx-validate
make test       # runs unit + runner tests
make smoke      # builds, then runs smoke-test.sh
make release    # cross-compile to dist/<os>-<arch>/ for publishing
make clean
```

`make release` produces one directory per platform under `dist/`, each
containing the binary, a `component.yaml`, and the `LICENSE`. Windows gets
`component.windows.yaml` (renamed to `component.yaml`) because the binary
there is `mdx-validate.exe` — `dpm publish` validates that the manifest's
`path:` resolves to a real file on every platform.

## Publishing & installing as an OCI component

dpm components are distributed as OCI artifacts (Open Container Initiative —
the same artifact format used by container registries), one manifest per
`<os>/<arch>`. The commands below require **dpm 3.5.1+** (`dpm publish` and
`dpm tags` do not exist in 3.4.x).

### 1. Publish to an OCI registry

```sh
make release   # cross-compile into dist/<os>-<arch>/

dpm publish component oci://<registry>/mdx-validate:<version> \
  -p darwin/arm64=dist/darwin-arm64 \
  -p darwin/amd64=dist/darwin-amd64 \
  -p linux/arm64=dist/linux-arm64 \
  -p linux/amd64=dist/linux-amd64 \
  -p windows/amd64=dist/windows-amd64
```

- The docs tooling registry is `europe-docker.pkg.dev/da-images/public`.
- `--dry-run` validates every per-platform manifest and the required
  `LICENSE` **without pushing** — run this first.
- Auth defaults to Docker's `~/.docker/config.json`; override with
  `--auth <file>`. `--extra-tags`/`-t` adds tags beyond the semver;
  `--include-git-info`/`-g` stamps git provenance annotations.
- Promotion from the `*-unstable` registry to the public one is a gated
  step owned by the dpm/release team (`dpm repo promote-components …`), not
  part of normal component development.

### 2. Declare it in a project

Add the component to the project's `daml.yaml` under `components:`. An entry
is one of three forms:

```yaml
components:
  # published component pulled from the configured registry
  - mdx-validate:0.1.0
  # …or a full OCI reference
  - oci://europe-docker.pkg.dev/da-images/public/mdx-validate:0.1.0
  # …or a local checkout, for development
  - name: mdx-validate
    path: ./tools/mdx-validate
```

> The older `override-components:` key still works but is **deprecated** in
> dpm 3.5.1 — prefer `components:`.

### 3. Use it (and confirm it installed)

```sh
dpm --help                       # mdx-validate appears under Dpm-SDK Commands
dpm mdx-validate --version       # confirms the resolved binary runs
dpm mdx-validate <path>          # real run
```

dpm pulls and caches the resolved platform under
`~/.dpm/cache/components/mdx-validate/<version>/`. To run a published
component once without declaring it in a project:

```sh
dpm component run mdx-validate <version> [args]
dpm tags oci://<registry>/mdx-validate     # list published versions
```

### 4. Remove it from a project

Delete the component's entry from `daml.yaml` `components:`; it stops
appearing in `dpm`. To also drop the cached download:

```sh
rm -rf ~/.dpm/cache/components/mdx-validate
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0    | Clean run (no errors; warnings allowed unless `--strict`) |
| 1    | Blocking findings reported |
| 2    | Usage error or I/O failure |

## Adding a validator

1. Implement `validate.Validator` in `internal/validate/<name>.go`.
2. Register the new validator in `validate.DefaultValidators()`.
3. Add unit tests for the validator itself and (if path discovery
   changes) update `runner_test.go`.
4. Run `make smoke` and validate against a real docs tree (e.g. a
   `docs-main/` checkout) before landing — the false-positive guard is part
   of the contract.
