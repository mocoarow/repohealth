# repohealth

A command-line tool that reads a repository's description from `repohealth.yml`.
Everything specific to a repository lives in that file.

## Usage

```sh
go run github.com/mocoarow/repohealth/cmd/repohealth@<version> validate -config repohealth.yml
```

| Command | Description |
| --- | --- |
| `validate` | Load and validate the configuration file |

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
```

- `version` must be `1`.
- `areas` names must be unique. Paths are slash-separated directories relative to the directory holding the configuration file; they must exist there, must not leave it or go through a symbolic link, and must not contain one another.
- `groups` are optional. Each names two or more distinct areas defined in `areas`, and group names must be unique.
- `exclude` holds doublestar glob patterns relative to the same directory. A pattern must be a clean slash-separated path, such as `gen/**` rather than `./gen/**` or `gen/`.
