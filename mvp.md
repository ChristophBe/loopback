# MVP of the Code Trainer

The MVP tests the core idea from [goal.md](goal.md): a daily 5-minute lesson in which the learner makes a small coding change, gets an automated check and reads explanatory feedback. It should show whether this loop is engaging and whether people learn from it.

## Scope

**In scope**

- **Languages:** Rust and Go.
- **Project:** the Redis-style server, as a vertical slice of about 10 lessons per language.
- **IDE:** a JetBrains plugin that works in GoLand, RustRover and IntelliJ IDEA.
- **Lessons with automated checks:** each lesson is one small coding change, verified by tests, with feedback that explains what went wrong.
- **Bad-code review exercises:** at least 2 per language.
- **Progress:** XP, levels, a daily streak and a daily goal.
- **Accounts:** one account whose progress is shared between the plugin and the web app.
- **Mobile web app:** quizzes and flashcards, bad-code review exercises and a progress view, in the phone browser.

**Out of scope (after the MVP)**

- VS Code plugin.
- Native mobile apps.
- Leaderboards and other social features.
- Story or game world.
- Badges beyond a minimal set.
- Projects other than the Redis-style server (HTTP server, shell, memory allocator, parser, job queue, reverse proxy).
- Beginner course.

## Components

| Component | Responsibility |
|---|---|
| JetBrains plugin | Lesson panel, creating the learner project from a template repo, applying each lesson's starter code and tests, running checks, showing feedback, showing XP and streak |
| Check runner | Runs `go test` or `cargo test` locally with the learner's own toolchain and turns the results into explanatory feedback |
| Content format | Language-neutral lesson definition: instructions, starter code, tests, hints, explanations, review exercises. Lessons are Markdown files with YAML metadata in the repo, reviewed through pull requests. Each lesson declares the files it owns |
| Lesson content | About 10 Redis-style server lessons per language, plus the bad-code review exercises and quizzes |
| Backend | Written in Go. Accounts, progress, XP and streak storage, and an API for the plugin and the web app |
| Gamification engine | XP, levels, streaks and the daily goal. Runs in the backend so both clients show the same state |
| Mobile web app | Login, quizzes and flashcards, bad-code review exercises, progress view |

How the components fit together, the API and the spec-driven code generation are described in [architecture.md](architecture.md).

## Design

The product is branded **loopback**. The look is minimalist, terminal-first and technical, and it should blend in with modern IDEs instead of looking like a game. The rules and tokens are in [brand/](brand/README.md).

- **Shared by both clients:** the JetBrains plugin and the mobile web app use the same tokens (`brand/tokens.json`), so they look the same.
- **Themes:** dark and light. The plugin follows the IDE theme and the web app follows the system setting.
- **Type:** JetBrains Mono for all text.
- **Color:** IDE-grey neutrals and one muted steel-blue accent. Check results use a glyph and a word as well as color: `✓ pass`, `✗ fail`, `! hint`.
- **Components needed for the MVP:** button, prompt input, lesson card, check result, XP bar and code block with a flagged line for bad-code reviews.
- **Gamification tone:** XP, level and streak are shown as plain numbers and a thin bar. There are no mascots, no confetti and no emoji.

## Core flow

1. The learner signs in and picks Rust or Go.
2. The plugin opens the day's lesson. On the first lesson it copies the template repo locally. For later lessons it applies the lesson's starter code and tests to that project, overwriting only the files the lesson owns. If the toolchain is missing, it shows install instructions.
3. The learner makes the small coding change.
4. The learner runs the check, which runs locally, and reads the feedback.
5. The learner earns XP and keeps the streak.
6. Later, on the phone, the learner does a quiz or a bad-code review that uses the same account and progress.

## Build order

1. Content format.
2. Check runner with one lesson.
3. Plugin skeleton.
4. Backend and accounts.
5. XP and streak.
6. Mobile web app.
7. The rest of the lesson content.

## Success criteria

- A learner can finish the whole slice in each language.
- Daily lesson completion is measurable.
- Learners come back on consecutive days.
- Learners report that the feedback is clear and the lessons are fun.

## Decisions

- **Backend:** Go with the Echo router, ent as ORM on PostgreSQL, and logrus for logging.
- **Specs first:** the API, content schemas and database schema are specs, and code is generated from them (see [architecture.md](architecture.md)).
- **Accounts:** OAuth providers and email magic link. The plugin signs in through the device-code flow.
- **Web app:** React and Vite single-page app.
- **Content delivery:** the backend imports the content repo and serves lessons through the REST API to the plugin and the web app.
- **Project setup:** the plugin copies a template repo locally, then each lesson adds its starter code and tests to it.
- **Checks:** run locally on the learner's machine with their installed Go or Rust toolchain. This matches the goal of learning the real tools and needs no sandbox. Results are not verified server-side, so XP can be gamed. That is acceptable for the MVP.
- **Authoring:** lessons are files in the repo, Markdown with YAML metadata, reviewed through pull requests.
- **Earlier edits:** each lesson declares the files it owns. Applying a lesson overwrites only those files, so the learner's other code stays untouched. Lessons must be written to work with the learner's own code outside their owned files.
- **Missing toolchain:** the plugin checks for the Go or Rust toolchain when the project is set up. If it is missing, the plugin shows install instructions and a re-check button. It does not install anything automatically.
- **Navigation:** the path is fixed. Learners can redo finished lessons but cannot skip ahead.
