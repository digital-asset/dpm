# Release of Dpm

Unreleased Changes since last tagged release (1.0.22)

## Additions

- **Git dependencies.** You can depend on a pre-built `.dar` stored in a Git repository, as well as on one published to an OCI registry. Declare it in `dependencies` or `data-dependencies` in `daml.yaml`, or add it with `dpm add dar <git-uri> --dependencies` (or `--data-dependencies`).

  A file in a repository is written as `git:<host>/<owner>/<repo>#<ref>?path=<file>.dar`, or as a structured mapping with `url`, `ref`, and `path` (or `release` and `asset`). The `git:` one-liner works on any SDK that includes this `dpm`. The structured mapping works only on SDK 3.6 and later, not on earlier SDKs: `damlc` before 3.6 rejects map entries in `dependencies` and `data-dependencies`. `dpm` reports that when it launches an SDK command such as `dpm build`, using the SDK selected for the command: `DPM_SDK_VERSION` if set, otherwise the package `sdk-version`, otherwise `multi-package.yaml`. `dpm install package` and `dpm update` still fetch the file over HTTPS and, when the ref is a branch or tag, rewrite that ref to the commit it pointed at. A mapping stays a mapping, and a one-liner stays a one-liner. Later installs stay on that exact file. `dpm resolve` returns the cached local path.

  Repeated repositories can be shortened with an `artifact-locations` alias. Pinning expands the alias to the full `git:` line.

  ```yaml
  sdk-version: 3.6.0

  artifact-locations:
    "@example-repo":
      url: "git:github.com/org/repo"

  dependencies:
    - git:github.com/org/repo#main?path=packages/foo.dar
    - "@example-repo#main?path=packages/bar.dar"
    - git:
        url: github.com/org/repo
        ref: main
        path: packages/baz.dar
      main-package-id: <package-id>

  data-dependencies:
    - git:github.com/org/repo?release=v1.0.0&asset=foo.dar
    - git:
        url: github.com/org/repo
        release: v1.0.0
        asset: bar.dar
  ```

  GitHub releases use `?release=<tag>`. Leave off `asset` to take every `.dar` in the release, or name one with `&asset=<file>.dar`. Release assets are GitHub-only. A file inside the repository works on any HTTPS Git host. Pasted GitHub or GitLab file links are accepted and rewritten into the form above.

  `dpm update --check` confirms the project is ready to build: repository files are pinned to a commit and cached, and release assets are downloaded. It does not change the project.

## Removal / Deprecated

- None
