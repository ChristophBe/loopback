# Definition of Ready

An issue is ready when someone can pick it up and finish it without asking the author. The `/issue` and `/bug` skills and the issue forms in `.github/ISSUE_TEMPLATE/` check these rules. The section headings below are the headings used in the issue body.

## Labels

- **Type:** `enhancement` (feature), `task`, `bug`, `question`. Content issues use `content` as their component and `enhancement` as type.
- **Component:** `backend`, `web`, `plugin`, `content`, `api-spec`, `infra`. An issue has at least one.
- **State:** `needs-refinement` marks an issue that does not meet this definition yet. It is not ready to be picked up.

Create the labels once with:

```sh
gh label create task --color 5319e7 --description "Technical work without user-visible change"
gh label create backend --color 1d76db --description "Go API, ent schema, importer"
gh label create web --color 1d76db --description "React web app"
gh label create plugin --color 1d76db --description "JetBrains plugin"
gh label create content --color 1d76db --description "Lessons, quizzes, reviews"
gh label create api-spec --color 1d76db --description "OpenAPI, JSON schemas, gamification rules"
gh label create infra --color 1d76db --description "Docker Compose, CI, tooling"
gh label create needs-refinement --color fbca04 --description "Does not meet the Definition of Ready"
```

## All issues

- **Title:** imperative, specific, under 72 characters, starts with the component: `api: reject completion for a locked lesson`.
- **Context:** why this matters, with a link to the section of [goal.md](../goal.md), [mvp.md](../mvp.md) or [architecture.md](../architecture.md) it comes from.
- **Labels:** at least one component label and one type label.
- **Size:** one pull request, at most about two days of work. Larger work is split into issues, and a parent issue lists them as a task list.
- **Dependencies:** `Blocked by #n` for each blocker, or `none`.
- **No open questions:** a question that blocks the work is its own `question` issue and is linked as a blocker.

## Feature and task (`/issue`)

Headings: `Context`, `Acceptance criteria`, `Out of scope`, `Interface impact`, `Test approach`, `Dependencies`.

- **Acceptance criteria:** a checklist of observable, testable outcomes, at least one. Each item can be answered with yes or no.
- **Out of scope:** what is not part of this issue, or `none`.
- **Interface impact:** the specs that change (`api/openapi.yaml`, `api/schemas/`, `backend/ent/schema/`, `api/gamification.yaml`), or `none`. Specs change first and generated code is never edited by hand (see [architecture.md](../architecture.md#principle-interface-first-generate-from-specs)).
- **Test approach:** which tests prove the criteria: unit, contract, runner fixture or manual steps.
- **UI work** names the [brand components](../brand/README.md#components) it uses and states that it was checked in dark and light theme and at phone width.
- **Local run:** if the change touches the stack, the criteria include that it works with `make up`.

## Content (`/issue`)

Headings: `Context`, `Learning objective`, `Lesson details`, `Acceptance criteria`, `Dependencies`.

- **Learning objective:** the one thing the learner can do afterwards.
- **Lesson details:** language, project, lesson number, owned files, XP and minutes (about 5), starter code, tests, hints and the explanation for each likely failure.
- **Acceptance criteria:** include that the content lint passes and that the lesson works with the reference solution of the lessons before it.

## Bug (`/bug`)

Headings: `Environment`, `Steps to reproduce`, `Expected behavior`, `Actual behavior`, `Evidence`, `Frequency and impact`, `Related issues`, `Suspected cause`.

- **Environment:** component, version or commit, OS, and the IDE and version or browser. Add the Go or Rust version when a check is involved.
- **Steps to reproduce:** numbered, minimal, starting from a clean state such as `make reset`.
- **Expected behavior** and **Actual behavior:** one statement each.
- **Evidence:** log lines, request ID, screenshot or failing test output.
- **Frequency and impact:** always or sometimes, and what it blocks (a lesson, correct XP, nothing but looks).
- **Related issues:** the search for duplicates was done. Link related issues or write `none found`.
- **Suspected cause:** optional, and only marked as a suspicion.
