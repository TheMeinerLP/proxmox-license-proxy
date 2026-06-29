# Contributing

Thanks for your interest! This is a small Go binary; the bar to contribute is low.

## Development

Requires Go 1.26+.

```sh
go test -race ./...   # run the tests
golangci-lint run     # same config as CI (.golangci.yml)
go build ./...         # compile everything
```

CI (GitHub Actions) is the source of truth: every push and PR runs the tests
(`-race`), `golangci-lint` and a build. Keep all three green.

## Conventions

- **Code & comments in English.**
- **[Conventional Commits](https://www.conventionalcommits.org)** are required
  (`feat:`, `fix:`, `docs:`, `refactor:`, `perf:`, `test:`, `build:`, `ci:`,
  `chore:`, `revert:`). PRs are **squash-merged** and the **PR title** becomes the
  commit - a CI check lints it. Release Please derives the next version and the
  changelog from these (`feat` -> minor, `fix` -> patch; `!`/`BREAKING CHANGE` -> major).
- Add a test for new behaviour where it is reasonable (table-driven tests fit
  most of this codebase).

## Architecture (short)

```
subscription   domain core (Proxmox protocol, key/status types) - no internal deps
registry       JSON file persistence (flock + atomic writes)
app            business logic shared by HTTP + CLI (verify decision, license defaults)
transport/httpapi   the HTTP server (verify.php emulation, REST API, health probes)
cli            cobra commands (server + client side)
config         wire Config -> validated domain Settings
certs / hosts / client   client-side helpers (trust store, /etc/hosts, self-install)
```

Keep it simple - this deliberately avoids hexagonal/DDD ceremony.

## Releasing

Releases are automated - **do not push tags by hand**:

1. Merge Conventional-Commit PRs into `master`.
2. Release Please opens/updates a release PR (`chore: release X.Y.Z`) with the
   bumped version and `CHANGELOG.md`.
3. Merging that PR creates the tag + GitHub Release, then the workflow runs
   GoReleaser (binaries, checksums, `.deb`/`.rpm`/`.apk`) and pushes a multi-arch
   image to `ghcr.io`.

### One-time maintainer setup

- **Settings -> Actions -> General -> Workflow permissions:** enable *"Allow GitHub
  Actions to create and approve pull requests"*.
- **Settings -> General -> Pull Requests:** enable *Allow squash merging* and set
  the squash commit message to *"Pull request title"*.
- *(Optional)* add a `RELEASE_PLEASE_TOKEN` secret (PAT or GitHub App token) so CI
  also runs on the release PR; without it the default `GITHUB_TOKEN` is used.
- The packaging assets live in [`packaging/`](packaging/) (systemd unit, default
  config, maintainer scripts); the build is configured in `.goreleaser.yaml`.
