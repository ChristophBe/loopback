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
