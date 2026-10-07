# e01s02 — Container and CI parity

**type:** feat · **risk:** P1 · **context:** infra · **bcps:** 3 · **scope:** S01

## 1. Business narrative

As the developer, I run the same app in a container that I will later ship, and every push runs the same gate as my laptop. Problems found in CI are problems I can reproduce locally with one command.

## 2. Actors

- **Developer** — runs `docker compose up --build`.
- **GitHub Actions** — runs CI on each push and pull request.

## 3. Preconditions

- e01s01 is done: `make preflight` is green.
- Docker 29.8.2 and Compose v5.5.1 work in WSL (`specs/state.yaml`).

## 4. Trigger

The developer runs `docker compose up --build`, or pushes a commit.

## 5. Main flow

1. `docker build` compiles a static binary (`CGO_ENABLED=0`) in `golang:1.26` and copies it into `gcr.io/distroless/static-debian13:nonroot`.
2. `docker compose up --build` starts the app on http://localhost:8080 with `DB_PATH=/data/upareal.db` on a named volume.
3. The compose healthcheck runs `/upareal healthcheck`, which calls `GET http://127.0.0.1:8080/healthz` and exits 0 or 1.
4. On push or PR, CI checks out the code, sets up Go from `go.mod`, installs golangci-lint v2.14.0, runs `make preflight` and runs `docker build`.

## 6. Alternative flows

- **6a. Healthcheck fails** — compose marks the container `unhealthy`. `docker compose ps` shows it.
- **6b. Preflight fails in CI** — the workflow fails and the image is not built.
- **6c. Volume not writable** — the app fails at startup. The image creates `/data` owned by uid 65532 so a new named volume inherits that owner.

## 7. Business rules

- The container runs as non-root (uid 65532).
- The container image holds no secrets. Config comes only from the environment.

## 8. Data

Named volume `upareal-data` mounted at `/data`. It holds the SQLite file and its WAL files.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `upareal healthcheck` | Exits 0 when `/healthz` returns 200 within 3 s, else exits 1 |
| `compose.yaml` | Service `app`, port `8080:8080`, volume `upareal-data:/data`, env `DB_PATH` |
| `.github/workflows/ci.yml` | Triggers on `push` and `pull_request`; jobs run Preflight and `docker build` |

## 10. Non-functional

- The final image has no shell and no package manager.
- The image tag in CI is not pushed anywhere. Publishing is e10.

## 11. Security & privacy

- Non-root user. Distroless base. No secrets in the image or in `compose.yaml`.
- CI uses no repository secrets in this story.
- Actions are pinned to major versions: `actions/checkout@v7`, `actions/setup-go@v7`, `golangci/golangci-lint-action@v9`.

## 12. Dependencies (slopcheck)

| Dependency | Version | Tag | Evidence |
|------------|---------|-----|----------|
| `golang` image | `1.26` (1.26.8 at plan time) | [OK] | Docker Hub official image |
| `gcr.io/distroless/static-debian13` | `nonroot` | [OK] | Tag exists (gcr.io tags list) |
| `actions/checkout` | v7 (v7.0.1) | [OK] | GitHub latest release |
| `actions/setup-go` | v7 (v7.0.0) | [OK] | GitHub latest release |
| `golangci/golangci-lint-action` | v9 (v9.3.0), lint v2.14.0 | [OK] | GitHub latest release |
| `github.com/rhysd/actionlint` | v1.7.12 (verify only, via `go run`) | [OK] | proxy.golang.org `@latest` |

## 13. Requirements

#### ADDED: R1 — Container parity
`docker compose up --build` serves the same app as `make run`.

#### ADDED: R2 — Container health
The container reports healthy only when `/healthz` returns 200.

#### ADDED: R3 — CI parity
Every push and PR runs `make preflight` and `docker build`.

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `upareal healthcheck` subcommand | Distroless has no shell or curl, so the binary must check itself. |

No other new units.

## 15. Implementation steps

1. Add `healthcheck` handling in `cmd/upareal` (when `os.Args[1] == "healthcheck"`). → verify: `go test -race ./cmd/upareal/... -run TestHealthcheckCommand`
2. Add a multi-stage `Dockerfile` (`golang:1.26` builder with `CGO_ENABLED=0`, `/data` owned by 65532, distroless `static-debian13:nonroot`) and a `.dockerignore`. → verify: `docker build -t upareal:dev .`
3. Add `compose.yaml` with the `app` service, port 8080, the `upareal-data` volume, `DB_PATH` and the healthcheck. → verify: `docker compose up -d --build --wait && curl -fsS http://localhost:8080/healthz; rc=$?; docker compose down; exit $rc`
4. Add `.github/workflows/ci.yml`: checkout, setup-go (`go-version-file: go.mod`), golangci-lint v2.14.0, `make preflight`, `docker build`. → verify: `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 .github/workflows/ci.yml`

## 16. Risks

- **The CI Go patch version differs from local** (`setup-go` reads `go 1.26.5` from `go.mod`). Detect: the CI log shows the Go version.
- **`--wait` hangs if the healthcheck never passes.** Mitigation: compose sets `start_period` and `retries`, so `--wait` fails instead of hanging.
- **CI can only be proven after a push.** The local check (actionlint) proves the syntax. The first green run on GitHub is part of the verification script.

## 17. Acceptance criteria

```gherkin
Feature: Container and CI parity

  Scenario: Container serves the app
    Given the image is built
    When the developer runs docker compose up --build
    Then http://localhost:8080/healthz returns 200
    And docker compose ps shows the app as healthy

  Scenario: Data survives a restart
    Given the app container has created /data/upareal.db
    When the container is recreated
    Then /data/upareal.db still exists on the upareal-data volume

  Scenario: CI runs the local gate
    Given a commit is pushed
    When the CI workflow runs
    Then it runs make preflight and docker build
    And it fails if either fails
```

## 18. Out of scope

- Pushing images to a registry, Caddy, Litestream, SSM (e10).
- Dependabot and Renovate.
- Branch protection rules on GitHub.

## 19. Verification Script (Step-by-Step)

1. Run `docker compose up -d --build --wait`. It returns once the app is healthy.
2. Run `docker compose ps`. The `app` service shows `healthy`.
3. Open http://localhost:8080. The pt-BR home page appears.
4. Run `docker compose exec app /upareal healthcheck; echo $?`. Expect `0`.
5. Run `docker compose down && docker compose up -d --wait`, then `docker compose exec app ls /data`. `upareal.db` is still there.
6. Run `docker compose down`.
7. Push the branch. In GitHub → Actions, the `ci` workflow ends green.

## 20. Traceability

- Scope: S01 (same checks locally and in CI).
- Decisions: `specs/planning-context.yaml` → `operations` (Docker-first, local compose without Caddy/Litestream).
- Code and tests carry `// story: e01s02`. Dockerfile, compose and workflow carry `# story: e01s02`.
