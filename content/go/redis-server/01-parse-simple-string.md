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
---

Return the text of a RESP simple string such as `+OK\r\n`.

Return an error when the input has no `+` prefix.
