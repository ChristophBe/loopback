# Architecture of loopback

This document describes how the components in [mvp.md](mvp.md) fit together. Product goals are in [goal.md](goal.md) and the look of the clients is in [brand/](brand/README.md).

## Principle: interface-first, generate from specs

Every interface between components is defined as a spec first. Code is generated from the spec and is never written by hand where a generator can do it. Generated code lives in `gen/` directories, is never edited, and CI fails if `make gen` produces a diff.

| Spec (source of truth) | Generates |
|---|---|
| `api/openapi.yaml` (OpenAPI 3.1) | Go server interface and types (`oapi-codegen`, strict Echo server), TypeScript client and types for the web app (`openapi-typescript`, `openapi-fetch`), Kotlin client and models for the plugin (`openapi-generator`), request validation, API reference, mock server for client development (Prism) |
| `api/schemas/lesson.schema.json` (JSON Schema for lesson, quiz, flashcard and review front matter) | Go structs for the importer, TypeScript and Kotlin types, content lint in CI over every lesson `content/**/*.md` and every quiz, flashcard and review `content/**/*.json` |
| `api/schemas/check-result.schema.json` (normalized test result) | Kotlin `CheckResult` model, Go type for the completion report, shared fixtures for the runner tests |
| `backend/ent/schema/*.go` (ent schemas) | typed Go ORM client, queries and predicates (`ent generate`), versioned SQL migrations (Atlas) |
| `brand/tokens.json` | CSS variables for the web app, Kotlin theme constants for the plugin |
| `api/gamification.yaml` (XP per lesson type, level curve, daily goal) | Go rules config and docs. Clients read the values from `GET /config` and keep no copy |

Workflow: edit the spec, run `make gen`, implement only the handler and business logic, then run the contract tests (generated from the OpenAPI spec, for example with schemathesis) against the backend in CI.

## Overview

```text
 content/ (Markdown + YAML, git)
        │  cmd/import (validate against lesson.schema.json)
        ▼
 ┌──────────────┐   REST /api/v1 (OpenAPI)   ┌─────────────────────┐
 │  PostgreSQL  │◄───── Go backend (Echo) ◄──┤ JetBrains plugin    │
 │  (ent)       │                            │ (Kotlin)            │
 └──────────────┘                      ▲     │ runs go/cargo test  │
                                       │     │ locally             │
                                       │     └─────────────────────┘
                                       └──── React + Vite SPA (mobile web)
```

The backend is the only component with state. The plugin and the web app are clients of the same REST API, so they show the same progress, XP and streak.

## Repository layout

```text
api/            openapi.yaml, schemas/, gamification.yaml
backend/        Go module
  cmd/api       HTTP server
  cmd/import    content importer
  internal/     auth, content, progress, gamification, http
  ent/schema    ent entities
  migrations/   Atlas migrations
web/            React + Vite SPA
plugin/         JetBrains plugin (Kotlin, Gradle IntelliJ Platform plugin)
content/        lessons per <lang>/<project>/NN-slug.md, quizzes, flashcards and reviews as <lang>/<project>/NN-slug.json, template-<lang>/
brand/          tokens and rules (exists)
deploy/         compose.yaml, Dockerfiles, .env.example
Makefile        gen, check-gen, test, up, down, reset
```

Generated output goes to `backend/gen`, `web/src/gen` and `plugin/src/gen`.

## Backend

A modular monolith in Go. Echo is the router, with middleware for auth, request validation against the OpenAPI spec, request IDs and request logging. ent is the ORM on PostgreSQL. Logging uses logrus with the JSON formatter in production.

| Module | Responsibility |
|---|---|
| auth | OAuth sign-in (GitHub, Google) and email magic link. Short-lived access JWT plus refresh token. The plugin uses the device-code flow: it shows a code, the learner approves it in the browser. |
| content | The importer reads the Markdown files (format: [docs/content-format.md](docs/content-format.md)), validates the YAML front matter (id, language, project, order, owned files, XP, minutes, type `lesson`, `quiz`, `flashcard` or `review`), and upserts them into Postgres, versioned by the content git SHA. The API for content is read-only. |
| progress | Lesson completions and quiz or review attempts per learner. |
| gamification | An append-only `xp_events` ledger. Level, streak and daily goal are derived from it, with the streak computed in the learner's time zone. One engine serves both clients, so their state matches. |

### Auth tokens

Access tokens are HS256 JWTs (`sub` is the user id, issuer `loopback`) signed with `JWT_SECRET`. The API refuses to start when the secret is unset or shorter than 32 bytes. `JWT_ACCESS_TTL` sets the access-token lifetime (default `15m`). A refresh token is 32 random bytes; only its SHA-256 is stored in `Session.token_hash`, and `REFRESH_TOKEN_TTL` (default `720h`) sets its lifetime. `POST /auth/refresh` revokes the presented session and creates a new one, so each refresh token works once and a revoked or expired token is rejected with `invalid_grant`. The auth middleware reads the security requirements from the embedded OpenAPI spec: every operation without `security: []` needs a valid `Authorization: Bearer` access token, otherwise the API answers `401` with the `Error` schema. Handlers read the user with `auth.UserID(ctx)`.

Device-code sign-in: `POST /auth/device` returns a device code (secret, polled with `POST /auth/token`), a user code (`XXXX-XXXX`) and the verification URI (`DEVICE_VERIFICATION_URI`); both codes live 10 minutes. The learner enters the user code in the web app, which calls `POST /auth/device/approve` with the learner's access token. Until then `POST /auth/token` answers `400 authorization_pending`, afterwards it returns a token pair once (the session is named `plugin`), and after expiry `400 expired_token`. The grants are held in memory (`auth.MemoryDeviceStore`, behind `auth.DeviceStore`), so they are lost on a restart and not shared between API instances.

### Data model

ent entities with edges (for example `User` has many `Completion` and `XpEvent`):

`User`, `Identity` (provider account), `Session` (refresh token), `Course` (language and project), `Lesson`, `Exercise` (quiz, flashcard, review), `Completion`, `Attempt`, `XpEvent`, `StreakDay`, `ContentVersion`.

### REST API (`/api/v1`)

| Area | Endpoints |
|---|---|
| auth | `POST /auth/magic-link`, `GET /auth/oauth/{provider}`, `POST /auth/device`, `POST /auth/device/approve`, `POST /auth/token`, `POST /auth/refresh` |
| content | `GET /courses`, `GET /courses/{id}/lessons`, `GET /lessons/{id}` (instructions, starter files, tests, hints, owned files), `GET /exercises?type=`, `GET /exercises/{id}` |
| progress | `GET /me`, `GET /me/progress`, `POST /lessons/{id}/completions` (the plugin reports the check result and test summary), `POST /exercises/{id}/attempts` |
| gamification | `GET /me/stats` (XP, level, XP to next level, streak, daily goal), `GET /me/xp-events`, `GET /config` |
| ops | `GET /healthz` |

Rules the server enforces:

- `POST /lessons/{id}/completions` takes an idempotency key.
- XP is awarded once per lesson. Redoing a lesson gives none.
- The path is fixed: lesson N cannot be completed before lesson N-1.
- Check results are reported by the client and are not verified. This is the accepted trust model for the MVP (see [mvp.md](mvp.md#decisions)).

## Web app

React and Vite single-page app, mobile first, following the system theme. It uses the generated TypeScript client with TanStack Query, and the `lb-` components from [brand/](brand/README.md).

| Route | Content |
|---|---|
| `/login` | OAuth buttons and magic-link prompt |
| `/` | Dashboard: courses, today's lesson card, XpBar |
| `/course/:id` | The fixed lesson path with progress |
| `/practice` | Quizzes and flashcards |
| `/review/:id` | Bad-code review exercise |
| `/profile` | Level, XP, streak, daily goal |
| `/device` | Approves the plugin's device code after sign-in |

Lessons are solved only in the IDE. The web app shows the lesson card with the instruction to open it in the plugin.

## JetBrains plugin

Kotlin, built with the Gradle IntelliJ Platform plugin, targeting GoLand, RustRover and IntelliJ IDEA.

- **Lesson tool window:** renders the lesson Markdown, hints and feedback, and shows XP and streak.
- **Project bootstrap:** copies `template-<lang>` locally, then applies each lesson's starter code and tests, overwriting only the files the lesson owns.
- **Check runner:** an interface with `GoTestRunner` (`go test -json`) and `CargoTestRunner` (`cargo test` with libtest JSON, `cargo-nextest` as fallback). Both produce the normalized `CheckResult` from `check-result.schema.json`.
- **Feedback mapper:** turns a `CheckResult` into the CheckResult view (expected and got, plus the hint mapped in the lesson YAML).
- **Reporting:** posts the completion to the API. Reports queue offline and are sent when the connection returns.
- **Toolchain detection:** shows install instructions and a re-check button when Go or Rust is missing.
- **Auth:** device-code flow, tokens in the IDE `PasswordSafe`.
- **Theme:** follows the IDE theme, tokens from `brand/tokens.json`.

## Running locally

`make up` starts the whole backend side with one Docker Compose file (`deploy/compose.yaml`). Nothing outside Docker is needed except the plugin's own toolchain, and no external accounts are needed.

| Service | Image / build | Port | Purpose |
|---|---|---|---|
| `db` | `postgres:16` with a named volume | 5432 | Data. A healthcheck (`pg_isready`) gates the services below. |
| `migrate` | `backend` image, one-shot | none | Applies the Atlas migrations, then exits. |
| `import` | `backend` image, one-shot | none | Runs `cmd/import` on `./content`, which is mounted read-only. It validates every file, upserts them idempotently in one transaction (unchanged rows are not written) and records the git SHA of the content as a `ContentVersion`: `git rev-parse HEAD`, or the `CONTENT_SHA` environment variable when git is unavailable (`make import` sets it). Re-run with `make import` after editing a lesson. |
| `api` | `backend` image, `air` live reload in the dev profile | 8080 | The REST API. Starts after `migrate` and `import` have completed successfully. |
| `web` | `node` image running the Vite dev server | 5173 | The SPA with hot reload. Source mounted from `./web`. `/api` is proxied to `api:8080`, so the browser makes no cross-origin calls. |
| `mailpit` | `axllent/mailpit` | 8025 (UI), 1025 (SMTP) | Catches magic-link emails. Open the UI to click the link. |

Design rules that keep this working:

- **One configuration source:** `deploy/.env.example` is copied to `.env` by `make up` on first run. The backend reads only environment variables. The same variables work in Docker Compose and in production.
- **Dev sign-in:** with `AUTH_DEV_LOGIN=true` the API adds `POST /auth/dev-login` (no body, no secret; signs in the fixed user `dev@loopback.local`) and the login page shows a `dev login` button. The flag defaults to off, the route is not in `openapi.yaml` and is registered only when the flag is set. GitHub and Google OAuth need real client credentials, so they are optional locally and the buttons are hidden when the credentials are empty. Magic link through Mailpit and dev login work offline.
- **Plugin against local stack:** the plugin runs on the host, not in a container, because it is launched by the IDE (`./gradlew runIde`). Its API base URL is a setting that defaults to `http://localhost:8080/api/v1` in the sandbox IDE. The device-code approval page is served by the web app at `http://localhost:5173/device`. The check runner uses the Go or Rust toolchain installed on the host, as in production.
- **Web app URLs:** the SPA reads the API base path `/api/v1` relative to its own origin, so the same build runs behind the Vite proxy locally and behind a reverse proxy in production.
- **Production images:** the `backend` Dockerfile is multi-stage and produces a static binary. The `web` Dockerfile builds the SPA and serves it with a static server. The Compose file uses the dev stages by default and `docker compose --profile prod` builds the production stages, which tests that the images start.
- **Client-only work:** `docker compose --profile mock up web mock` runs Prism from `openapi.yaml` instead of the backend, so the web app can be built against a spec before the backend exists.
- **Reset:** `make reset` removes the database volume and re-runs migrations and import.
- **Code generation:** `make gen` runs in a `tools` container image that pins `oapi-codegen`, `ent`, `atlas`, `openapi-generator` and `openapi-typescript`, so every developer and CI produce identical output.

## Cross-cutting

- **Versioning:** the API is `v1`. Content is versioned by git SHA. The plugin sends its version, and the backend can reject versions below a minimum.
- **Configuration:** environment variables, no secrets in the repo. Local defaults live in `deploy/.env.example`.
- **Local development:** see [Running locally](#running-locally).
- **CI:** `make check-gen`, Go tests, contract tests, web tests and build, plugin build, content lint.
- **Observability:** logrus structured logs, a request ID on every log line, `/healthz`.
- **Analytics:** events for the success criteria in [mvp.md](mvp.md): daily lesson completion and consecutive-day return.

## Build order

This refines the build order in [mvp.md](mvp.md#build-order). Each step starts with its spec.

1. **Specs and content format:** `lesson.schema.json`, `check-result.schema.json`, one sample lesson, content lint.
2. **Check runner with one lesson:** runner and fixtures, driven by the `CheckResult` schema.
3. **OpenAPI spec and backend skeleton:** generated Echo server, ent schemas, importer, auth, and the Docker Compose stack (`db`, `migrate`, `import`, `api`, `mailpit`) so every later step runs locally.
4. **Plugin skeleton:** generated Kotlin client, lesson window, bootstrap, runner, reporting.
5. **Gamification:** `gamification.yaml`, XP ledger, streak, `/me/stats`.
6. **Web app:** generated client, dashboard, practice, review.
7. **Lesson content.**

## Risks

- **XP can be gamed** because results are client-reported. Accepted for the MVP. Server-side verification is the later fix.
- **Owned-files drift:** a lesson written against files the learner changed may break. Content lint and a CI job that replays the whole path against a reference solution catch this.
- **Test output differs across toolchain versions:** `go test -json` and libtest JSON vary. Pin the supported versions and keep recorded outputs as runner fixtures.
