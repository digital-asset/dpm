# rst-to-mdx

A DPM component that converts reStructuredText documents into
Mintlify-compatible MDX files. Built for the migration from
`docs-website/docs/replicated/` (RST, Sphinx) to `docs-main/` (MDX,
Mintlify), but the converter runs against any RST file on disk.

## Quick start

```bash
cd examples/tools/rst-to-mdx
make build

# Convert one file
./rst-to-mdx path/to/input.rst path/to/output.mdx

# Or invoke through dpm — the local daml.yaml registers the component
# automatically when dpm runs from this directory.
dpm rst-to-mdx path/to/input.rst path/to/output.mdx

# Convert an entire RST tree to a mirror MDX tree in one command
./rst-to-mdx --batch \
  --input-dir <docs-website>/docs/replicated \
  --output-dir <output-mdx-tree> \
  --target-root <docs-main> \
  --copy-images

# Audit which RST files don't have a corresponding page in docs.json
./rst-to-mdx --audit-coverage \
  --docs-root <docs-website> \
  --target-root <docs-main>
```

## Input flexibility

The converter has no hard dependency on `docs-website/`. Any RST file
works as input, regardless of where it lives on disk. The only feature
that depends on `docs-website/` is **cross-reference resolution**:

| Situation | Cross-reference (`:ref:`, `:doc:`, etc.) behavior |
|---|---|
| Input lives somewhere under `docs-website/` | Auto-detected; the label index is built once and refs resolve to `/<path>#<anchor>` URLs (Mintlify serves `docs-main/` as site root). |
| Input lives elsewhere, `--docs-root <path>` passed | Same as above using the explicit root. |
| Input lives elsewhere, no `--docs-root` | Refs become `[label](#TODO-resolve-…)` markers a manual review can resolve later. |

Every other transform — headings, inline formatting and roles, links
(named, anonymous, `:doc:`, `:download:`, autolinks like `<https://…>`),
code blocks, admonitions, `.. todo::` / `.. wip::` notes, `.. toggle::`
collapsibles → `<Accordion>`, images and figures, lists, tables
(list-table, csv-table, grid), Sphinx tabs, `.. youtube::` and
`.. raw:: html` video embeds, comments, frontmatter, and provenance
markers — runs identically regardless of input location.

## Heading mapping

Underline characters map to a fixed level (Canton's published CSS uses
the same rule):

| Underline  | Level |
|---|---|
| `#`        | H1 |
| `*` / `=`  | H2 |
| `-`        | H3 |
| `~`        | H4 |
| `^`        | H5 |
| `"`        | H6 |

Overlined+underlined headings render one level shallower than the same
character used as underline-only, capped at H1. So `### Title ###` and
`############` both produce H1; `*** Title ***` is H1; `--- Title ---`
is H2.

## CLI flags

```
rst-to-mdx <input.rst> [output.mdx] [flags]
rst-to-mdx --batch --input-dir <dir> --output-dir <dir> [flags]

  --title string          override auto-detected page title (default: first heading)
  --description string    set frontmatter description
  --source-label string   provenance source label (auto from path)
  --docs-root string      root of an RST docs tree for cross-ref resolution
                          (auto-detects `docs-website/` if input lives in one)
  --target-root string    target docs-main/ root for image copy and path
                          derivation. Default "./docs-main" resolves
                          relative to cwd, so invoke from a docs repo root
                          or pass an explicit path.
  --copy-images           copy referenced images into target-root/images/docs_website/
  --strict                fail on unresolved :ref: or missing literalinclude
  --dry-run               print what would be written without touching disk
  -v, --verbose           show detailed conversion progress
      --version           print version and exit
```

### Common invocations

```bash
# Single file with image asset copy
./rst-to-mdx in.rst out.mdx --copy-images --target-root /tmp --verbose

# Override the auto-detected title
./rst-to-mdx in.rst out.mdx --title "Setting Up the Sandbox"

# Bail loudly on missing refs/files
./rst-to-mdx in.rst out.mdx --strict
```

## Directory layout

```
examples/tools/rst-to-mdx/
├── component.yaml             # Unix DPM component manifest
├── component.windows.yaml     # Windows manifest (.exe path)
├── daml.yaml                  # local components: registration for `dpm`
├── .gitignore                 # ignores the compiled binary and dist/
├── LICENSE                    # required at every published artifact root
├── cmd/rst-to-mdx/main.go     # Cobra CLI entrypoint
├── internal/
│   ├── convert/               # transform pipeline (one file per transform)
│   ├── include/               # .. include:: + .. literalinclude:: resolver
│   ├── labelindex/            # corpus walker that builds label → heading map
│   ├── navindex/              # docs.json walker so cross-refs land on real pages
│   └── pathmap/               # RST source path → MDX target path rules
├── smoke-test.sh              # end-to-end smoke against a real RST file
├── go.mod
├── Makefile
└── README.md
```

## Local DPM registration

Running `dpm --help` from inside `examples/tools/rst-to-mdx/` picks up the
local `daml.yaml` and surfaces `rst-to-mdx` under "Dpm-SDK Commands"
without any install step. Useful for iterative development.

## Publishing & installing as an OCI component

dpm components are distributed as OCI artifacts (Open Container Initiative —
the same artifact format container registries use), one manifest per
`<os>/<arch>`. The commands below require **dpm 3.5.1+** (`dpm publish` and
`dpm tags` do not exist in 3.4.x).

### 1. Publish to an OCI registry

```sh
make release VERSION=<version>   # cross-compile + stamp the binary

dpm publish component oci://<registry>/rst-to-mdx:<version> \
  -p darwin/arm64=dist/darwin-arm64 \
  -p darwin/amd64=dist/darwin-amd64 \
  -p linux/arm64=dist/linux-arm64 \
  -p linux/amd64=dist/linux-amd64 \
  -p windows/amd64=dist/windows-amd64
```

`make release` writes one `dist/<os>-<arch>/` directory per platform, each
containing the platform binary, a matching `component.yaml` (Windows gets
`component.windows.yaml`), and a `LICENSE` — all required by the publish
validator.

- The docs tooling registry is `europe-docker.pkg.dev/da-images/public`.
- `--dry-run` validates every per-platform manifest and the required
  `LICENSE` **without pushing** — run this first.
- Auth defaults to Docker's `~/.docker/config.json`; override with
  `--auth <file>`. `--extra-tags`/`-t` adds tags beyond the semver;
  `--include-git-info`/`-g` stamps git provenance annotations.
- Promotion from the `*-unstable` registry to the public one is a gated
  step owned by the dpm/release team (`dpm repo promote-components …`).

### 2. Declare it in a project

Add the component to the project's `daml.yaml` under `components:`. An entry
is one of three forms:

```yaml
components:
  # published component pulled from the configured registry
  - rst-to-mdx:0.1.0
  # …or a full OCI reference
  - oci://europe-docker.pkg.dev/da-images/public/rst-to-mdx:0.1.0
  # …or a local checkout, for development
  - name: rst-to-mdx
    path: ./tools/rst-to-mdx
```

> The older `override-components:` key still works but is **deprecated** in
> dpm 3.5.1 — prefer `components:`.

### 3. Use it (and confirm it installed)

```sh
dpm --help                    # rst-to-mdx appears under Dpm-SDK Commands
dpm rst-to-mdx --version      # confirms the resolved binary runs
dpm rst-to-mdx in.rst out.mdx # real run
```

dpm pulls and caches the resolved platform under
`~/.dpm/cache/components/rst-to-mdx/<version>/`. To run a published
component once without declaring it in a project:

```sh
dpm component run rst-to-mdx <version> [args]
dpm tags oci://<registry>/rst-to-mdx     # list published versions
```

### 4. Remove it from a project

Delete the component's entry from `daml.yaml` `components:`; it stops
appearing in `dpm`. To also drop the cached download:

```sh
rm -rf ~/.dpm/cache/components/rst-to-mdx
```
