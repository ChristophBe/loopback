---
name: implement
description: Implement a ready GitHub issue end to end in a git worktree. Analyses the ticket and the related code, checks for needed refactorings, plans with approval, implements, extends the tests, reviews the diff and opens a pull request. Use when the user wants to implement, build or work on an issue by number. Not for epics or issues that do not meet the Definition of Ready.
argument-hint: <issue number>
allowed-tools: Read, Grep, Glob, Edit, Write, Skill, EnterPlanMode, ExitPlanMode, mcp__codegraph__codegraph_explore, Bash(git:*), Bash(codegraph:*), Bash(gh issue view:*), Bash(gh pr create:*), Bash(gh pr view:*), Bash(gh pr checks:*), Bash(make:*), Bash(go:*), Bash(pnpm:*), Bash(./gradlew:*)
---

Implement GitHub issue `$ARGUMENTS` and open a pull request for it.

1. **Load the ticket.** Run `gh issue view <n> --json number,title,body,labels,milestone,state`. Read `docs/definition-of-ready.md`.
   - Stop if the issue is closed, or has the label `epic` (tell the user to split a task out with `/issue`) or `needs-refinement`.
   - Stop if a `Blocked by #n` issue is still open. List the blockers.
   - Stop if a Definition of Ready item is unmet. List the gaps. Never guess requirements.
2. **Create the worktree.** Name the branch `<type>/<n>-<slug>`: type `feat` for `enhancement`, `fix` for `bug`, `chore` for `task`, and the slug is the title in kebab-case without the component prefix. Run `git fetch origin main` and `git worktree add .claude/worktrees/<branch> -b <branch> origin/main`. If the branch already exists, show the state of its worktree and ask whether to reuse it. Do all further work inside the worktree.
3. **Analyse the related code with CodeGraph.** The project is set up for it: `.mcp.json` registers the MCP server, and `.claude/settings.json` allows its tools and adds the prompt hook.
   - Index the worktree: run `codegraph init -y` in it if it has no `.codegraph/` yet, otherwise `codegraph sync`. The index is git-ignored. Pass the worktree path as `projectPath` to every `codegraph_explore` call, so the results show the branch and not `main`.
   - Call `codegraph_explore` first, before Grep or Read, with the files, specs and symbols the ticket names, and once more with the ticket's behavior in plain words. Treat the returned source as already read.
   - Use the blast radius it reports to list the callers and the tests of every symbol you expect to change. A symbol with no tests found is a place that needs new tests.
   - Read the sections of `architecture.md`, `mvp.md` and `brand/README.md` that the ticket links. List the helpers, tests and patterns to reuse with `file:line`.
   - If the `codegraph` command is missing, tell the user to install it (see `README.md`), then fall back to Grep and Read for this run.
4. **Check for refactorings.** Decide whether the current structure makes the change awkward: duplication to extract, a missing seam for tests, an interface that has to change first. Use the CodeGraph callers of the code in question to see what a refactoring touches. If one is needed, plan it as separate behaviour-preserving commits before the feature commits, with the existing tests green after each. Do not refactor unrelated code.
5. **Plan and wait for approval.** Call EnterPlanMode. Map each acceptance criterion to the code change and the test that proves it. List spec changes first (`api/openapi.yaml`, `api/schemas/`, `backend/ent/schema/`, `api/gamification.yaml`), then refactorings, files and risks. Call ExitPlanMode and wait. After a rejection, revise the plan. Never write code before the plan is approved.
6. **Implement.** Change the specs first, run `make gen`, then write the handler and business logic. Never edit generated code (`backend/gen`, `backend/ent`, `web/src/gen`, `plugin/src/gen`). Before editing a symbol, call `codegraph_explore` on it and check its callers. After larger edits run `codegraph sync` in the worktree so later queries see the changes. Match the style of the surrounding code and follow `brand/README.md` for UI. Commit in small logical steps with imperative messages that mention `#<n>`.
7. **Extend the tests.**
   - Write at least one test per acceptance criterion that fails without the change, plus the error paths and edge cases the ticket names.
   - Follow the existing patterns: Go table tests and Echo `httptest` (`backend/internal/http/server_test.go`), ent tests with `TEST_DATABASE_URL` (`backend/ent/user_test.go`), Vitest and Testing Library (`web/src/App.test.tsx`), `kotlin.test` (`plugin/src/test`).
   - Use the "tested via callers" information of CodeGraph to find the existing tests that cover the changed symbols, and extend those files.
   - Measure the coverage of the changed code: `go test -coverprofile` with `go tool cover -func` for Go, `pnpm exec vitest run --coverage` if configured for the web app, the Gradle test report for the plugin. Test every uncovered new line or justify it in the pull request.
   - Run `make lint`, `make test` and `make check-gen`. While iterating, run the component targets (`make lint-go`, `make test-web`, and so on).
8. **Review.** Run `codegraph sync`, then use `codegraph_explore` on every changed symbol to check that all callers still fit the new behavior. Run the `code-review` skill on the branch diff against `main`, then the `simplify` skill. Fix every confirmed finding in follow-up commits and run the tests again.
9. **Open the pull request.** Run `git push -u origin <branch>` and `gh pr create --base main`.
   - Title: `<component>: <what>`, taken from the issue.
   - Body: `Closes #<n>`, a short summary, the acceptance criteria as a checklist with the test that proves each, the refactorings made, the review findings and how each was handled, how to verify, and which checks were run and which were not.
   - Print the pull request URL and the result of `gh pr checks`.
   - Leave the worktree in place and tell the user to remove it with `git worktree remove .claude/worktrees/<branch>` after the merge.

Rules: stop and ask when the ticket is ambiguous or the approved plan no longer fits. Never push to `main`, never force-push and never skip hooks. Report exactly which checks ran and which failed, and never describe a check as green that did not run.
