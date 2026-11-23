# repohealth

A command-line tool that reads a repository's description from `repohealth.yml`.
Everything specific to a repository lives in that file.

## Usage

```sh
go run github.com/mocoarow/repohealth/cmd/repohealth@<version> validate -config repohealth.yml
go run github.com/mocoarow/repohealth/cmd/repohealth@<version> snapshot -config repohealth.yml
go run github.com/mocoarow/repohealth/cmd/repohealth@<version> report -config repohealth.yml -out report
```

| Command | Description |
| --- | --- |
| `validate` | Load and validate the configuration file |
| `snapshot` | Measure the HEAD commit and append the result to `.repohealth/` |
| `report` | Write CSV files from the saved history to the directory given by `-out` |

`snapshot` writes `.repohealth/history.jsonl` (one line per snapshot) and `.repohealth/latest.json` next to the configuration file. Commit `.repohealth/` so the history is shared; changes under it do not make the worktree dirty.

`snapshot` requires:

- A merge-commit workflow. Repositories that bring changes in by squash or rebase get 0 cross-area changes.
- A clean worktree: no uncommitted changes to tracked files.
- A full clone, not a shallow one.
- Biome on `PATH` when the repository has TypeScript.

## Configuration

```yaml
version: 1
areas:
  - name: auth
    paths: [svc-auth]
  - name: question
    paths: [svc-question]
  - name: web
    paths: [svc-web]
groups:
  - name: backend
    areas: [auth, question]
exclude:
  - "**/*_mock.go"
stablePackages:
  - "*/domain/**"
coverage:
  - file: coverage.lcov
  - file: svc-web/coverage/lcov.info
    root: svc-web
```

- `version` must be `1`.
- `areas` names must be unique. Paths are slash-separated directories relative to the directory holding the configuration file; they must exist there, must not leave it or go through a symbolic link, and must not contain one another.
- Area and group names match `^[a-z0-9][a-z0-9_-]*$`.
- `groups` are optional. Each names two or more distinct areas defined in `areas`, and group names must be unique.
- `exclude` holds doublestar glob patterns relative to the same directory. A pattern must be a clean slash-separated path, such as `gen/**` rather than `./gen/**` or `gen/`.
- `stablePackages` are optional patterns of the same form over Go package directories.
- `coverage` lists optional lcov reports. `root` is the directory their source paths are relative to, the repository root when left out. Both are clean slash-separated paths like the `exclude` patterns.
