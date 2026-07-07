# AGENTS.md

Guidance for AI agents working in this repository. Gidoco is a single-binary Go
webhook receiver that pulls a Git repo and runs `docker compose up` on the
subdirectories that changed. See `README.md` for user-facing config and usage.

## Layout

- Flat `package main` at the repo root — there is no `cmd/` or `internal/`.
  All `.go` files belong to `package main` and live at the top level; do not
  introduce subpackages for new features.
- `main.go` is the entrypoint (gin HTTP server + orchestration). Feature
  modules: `git.go`, `docker.go`, `encryption.go`, `config.go`, `logger.go`.
  Tests sit alongside as `*_test.go`; `testdata/` holds SOPS fixtures.

## Commands

Task runner is `just` (see `justfile`), not make:

- `just build` — release binary to `build/gidoco`
  (`CGO_ENABLED=0 go build -trimpath -ldflags '-w -s'`). Do not use plain
  `go build` for release artifacts.
- `just build-debug` — debug build with `-gcflags="all=-N -l"`.
- `just test` / `just test-cover` — `go test -v ./...` (coverage →
  `build/coverage.html`).
- `just lint` — `go vet ./...`. `just tidy` — `go mod tidy`.
- Single test: `go test -v -run TestName .` (one root package).

There is no CI (only `.github/dependabot.yml`); nothing runs tests on push.

## Tests

- Unit tests need **no Docker daemon**. Git operations use in-memory storage
  (`go-git/v6/storage/memory`) and filesystems use `memfs` (`go-billy/v6`).
  Do not add Docker-dependent tests for git/docker/encryption logic.
- `encryption_test.go` sets `SOPS_AGE_KEY` itself and reads fixtures from
  `testdata/` — no external setup or env configuration required.
- Shared helpers live in `test_helper_test.go`
  (`touchFile`, `writeFile`, `mkdirAll`, `readFile`, `parentDir`).

## Code style

`.agent/rules/code-style-guide.md` is the source of truth. Key rules:

- Use `slog` with its `*Context` methods (`DebugContext`, `InfoContext`, …).
  The logger is propagated via context (`logger.go`: `FromContext` /
  `NewContext`) — derive scoped loggers with `slog.With(...)` + `NewContext`.
- Use `go-git` and `viper`; do not shell out to `git` or hand-roll config.
- Do **not** add section banner comments.

## Architecture notes

- A "compose project" is any top-level subdirectory of `repoRoot` containing a
  file matching `^(docker-)?compose(\.enc)?\.ya?ml$` (`docker.go`). Root-level
  compose files are not projects.
- `diffUpdate` collapses changed paths to the **top-level directory only**;
  root-level file changes are ignored and never trigger a deploy.
- Repo-level config: a `.gidoco.yml` placed in the managed repo root can set
  `includedProjects` and/or `excludedProjects` (mutually exclusive) to filter
  which subdirectories are deployed. Not documented in `README.md`.
- `AlwaysUp` (env `ALWAYS_UP`, undocumented in README) makes startup deploy
  all projects instead of a pull + changed-only update. Webhook path always
  uses changed-only `runUpdate` regardless of this flag.
- SOPS decryption targets filenames matching `\benc\.\w+$`
  (e.g. `secrets.enc.yml` → `secrets.yml`). Runs on fresh clone (whole tree)
  and on changed files per-project when `encryptionEnabled` (default `true`).

## Config & runtime

- `config.yml` and `.env` are **gitignored** — local-only, do not commit.
  Schema is in `README.md`; `viper.AutomaticEnv()` maps env vars
  (`REPO_ROOT`, `PORT`, …) to the same struct keys.
- `repoRoot` must be an absolute path (enforced in `checkConfig`).
- Runtime needs the Docker socket (`compose.yml` mounts
  `/var/run/docker.sock`) and the repo path must be identical on host and
  container (`/root/repo:/root/repo`).
- `DRY_RUN=true` runs compose in dry-run mode — safe for local testing.
