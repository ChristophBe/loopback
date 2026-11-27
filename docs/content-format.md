# Content format

Lessons, quizzes, flashcards and reviews are plain files in git under `content/`. The importer reads them and the API serves them read-only (see [architecture.md](../architecture.md)). The JSON Schemas in [`api/schemas/`](../api/schemas) are the source of truth. This page explains them for authors.

Run `make lint-content` before you push. It validates every file under `content/`, the check-result fixtures and every example on this page. A test fails when an example drifts from its schema.

## Layout

| Type | File | Schema |
| --- | --- | --- |
| `lesson` | `content/<lang>/<project>/NN-slug.md` | [`lesson.schema.json`](../api/schemas/lesson.schema.json) |
| `quiz` | `content/<lang>/<project>/NN-slug.json` | [`quiz.schema.json`](../api/schemas/quiz.schema.json) |
| `flashcard` | `content/<lang>/<project>/NN-slug.json` | [`flashcard.schema.json`](../api/schemas/flashcard.schema.json) |
| `review` | `content/<lang>/<project>/NN-slug.json` | [`review.schema.json`](../api/schemas/review.schema.json) |

A lesson is Markdown with YAML front matter. The other types are JSON, and their `type` field selects the schema. No type accepts fields that are not listed here.

## Shared fields

| Field | Type | Meaning |
| --- | --- | --- |
| `id` | string | Unique, kebab-case: lowercase letters and digits separated by single `-`. |
| `type` | string | `lesson`, `quiz`, `flashcard` or `review`. |
| `language` | string | `go` or `rust`. |
| `project` | string | Kebab-case project name, for example `redis-server`. |
| `title` | string | Non-empty display title. |

## Lesson

Front matter fields, besides the shared ones. All of them are required unless marked optional.

| Field | Type | Meaning |
| --- | --- | --- |
| `order` | integer, 1 or more | Position of the lesson within its project. |
| `minutes` | integer, 1 to 15 | Estimated time. A lesson that takes longer must be split. |
| `xp` | integer, 1 or more | XP awarded on completion. |
| `ownedFiles` | list of strings | Paths in the learner's project that the lesson edits and the plugin may overwrite. No duplicates. |
| `hints` | list of strings, optional | Revealed one at a time, from nudge to near-solution. |
| `failureExplanations` | map, optional | Maps a test name to the explanation shown when that test fails. |

The body must contain the level-2 headings `## Objective` and `## Instructions`, in this order. Headings inside fenced code blocks do not count. Other sections may follow.

<!-- example: lesson -->
````markdown
---
id: go-redis-01-parse-simple-string
type: lesson
language: go
project: redis-server
order: 1
title: Parse a simple string
minutes: 5
xp: 50
ownedFiles:
  - resp/parse.go
  - resp/parse_test.go
hints:
  - A simple string starts with `+` and ends with `\r\n`.
  - Strip the prefix and the trailing `\r\n`, then return what is left.
failureExplanations:
  TestParseSimpleStringMissingPrefix: The input has no `+` prefix, so Parse must return an error instead of text.
---

## Objective

Parse a RESP simple string.

## Instructions

Return the text of a RESP simple string such as `+OK\r\n`.

Return an error when the input has no `+` prefix.
````

## Quiz

Besides the shared fields, `questions` is required: a non-empty list. Each question has these fields, all required.

| Field | Type | Meaning |
| --- | --- | --- |
| `prompt` | string | The question. |
| `choices` | list of strings | At least 2, no duplicates. |
| `answer` | integer, 0 or more | Zero-based index into `choices`. |
| `explanation` | string | Shown after the learner answers. |

<!-- example: quiz -->
```json
{
  "id": "go-slices-quiz",
  "type": "quiz",
  "language": "go",
  "project": "loopback",
  "title": "Slices",
  "questions": [
    {
      "prompt": "What does len(nil) return for a nil slice?",
      "choices": ["0", "-1", "It panics"],
      "answer": 0,
      "explanation": "A nil slice has length 0."
    }
  ]
}
```

## Flashcard

Besides the shared fields, `cards` is required: a non-empty list. Each card has two required string fields.

| Field | Meaning |
| --- | --- |
| `front` | The prompt side. |
| `back` | The answer side. |

<!-- example: flashcard -->
```json
{
  "id": "rust-ownership-cards",
  "type": "flashcard",
  "language": "rust",
  "project": "loopback",
  "title": "Ownership",
  "cards": [
    { "front": "What happens to a value when its owner goes out of scope?", "back": "It is dropped." }
  ]
}
```

## Review

A review shows flawed code and asks the learner to flag the bad lines. Besides the shared fields, `code` and `flaggedLines` are required.

| Field | Type | Meaning |
| --- | --- | --- |
| `code.language` | string | `go` or `rust`. |
| `code.source` | string | The code to review. Use `\n` for line breaks. |
| `flaggedLines` | non-empty list | The lines the learner should flag. |
| `flaggedLines[].line` | integer, 1 or more | One-based line number in `code.source`. |
| `flaggedLines[].note` | string | What is wrong and what to do. |

<!-- example: review -->
```json
{
  "id": "go-ignored-error-review",
  "type": "review",
  "language": "go",
  "project": "loopback",
  "title": "Ignored error",
  "code": {
    "language": "go",
    "source": "func read(path string) []byte {\n\tdata, _ := os.ReadFile(path)\n\treturn data\n}\n"
  },
  "flaggedLines": [
    { "line": 2, "note": "The error is discarded. Return it and let the caller handle it." }
  ]
}
```

## Keeping the examples valid

Each example is preceded by a `<!-- example: <type> -->` comment. `make lint-content` extracts these blocks and validates them with the same code that checks `content/`. It fails when a type has no example or more than one.
