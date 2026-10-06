# Release of Dpm

Unreleased Changes since last tagged release (1.0.22)

## Additions

- **Git dependencies.** You can depend on a pre-built `.dar` stored in a Git repository, as well as on one published to an OCI registry. Declare it in `dependencies` or `data-dependencies` in `daml.yaml`, or add it with `dpm add dar <git-uri> --dependencies` (or `--data-dependencies`).

  A file in a repository is written as `git:<host>/<owner>/<repo>#<ref>?path=<file>.dar`. `dpm install package` and `dpm update` fetch it over HTTPS and, when the ref is a branch or tag, rewrite that line to the commit it pointed at. Later installs stay on that exact file. `dpm resolve` returns the cached local path.

  Repeated repositories can be shortened with an `artifact-locations` alias. Pinning expands the alias to the full `git:` line.

  ```yaml
  artifact-locations:
    "@example-repo":
      url: "git:github.com/org/repo"

  dependencies:
    - git:github.com/org/repo#main?path=packages/foo.dar
    - "@example-repo#main?path=packages/bar.dar"

  data-dependencies:
    - git:github.com/org/repo?release=v1.0.0&asset=foo.dar
  ```

  GitHub releases use `?release=<tag>`. Leave off `asset` to take every `.dar` in the release, or name one with `&asset=<file>.dar`. Release assets are GitHub-only. A file inside the repository works on any HTTPS Git host. Pasted GitHub or GitLab file links are accepted and rewritten into the form above.

  `dpm update --check` confirms the project is ready to build: repository files are pinned to a commit and cached, and release assets are downloaded. It does not change the project.

## Removal / Deprecated
- None
