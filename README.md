# loopback

- [goal.md](goal.md), [mvp.md](mvp.md), [architecture.md](architecture.md)
- Issues follow the [Definition of Ready](docs/definition-of-ready.md). In Claude Code use `/issue` for features, tasks and content, and `/bug` for defects.
- Content authors: see the [content format](docs/content-format.md).

## Development

Prerequisites: Go, Node 24 with pnpm, JDK 17, Docker, `golangci-lint` and GNU Make. Run `make help` for all targets.

- `make gen` generates code from the specs. Never edit generated files (`backend/gen`, `backend/ent`, `web/src/gen`, `plugin/src/gen`).
- `make lint` and `make test` run every linter and test. Each component also has its own target, for example `make lint-go` or `make test-web`.
- `make up` starts the local stack: web at <http://localhost:5173>, API at <http://localhost:8080/api/v1>, Mailpit at <http://localhost:8025>.
- Set `TEST_DATABASE_URL` to a Postgres URL to run the database tests in `make test-go`.
- `/implement <issue number>` implements a ready issue in a worktree, reviews it and opens a pull request.
