---
name: bug
description: Create a GitHub bug report that meets the project's Definition of Ready. Use when the user reports something that is broken or behaves wrongly. Use /issue for features, tasks and content.
argument-hint: <short description of the defect>
allowed-tools: Read, Grep, Write, Bash(gh issue list:*), Bash(gh issue create:*), Bash(gh issue view:*), Bash(gh label list:*), Bash(git rev-parse:*), Bash(uname:*), Bash(go version:*), Bash(cargo --version:*)
---

Create a GitHub bug report for the defect in `$ARGUMENTS` that passes the Definition of Ready.

1. Read `docs/definition-of-ready.md`, the "Bug" and "All issues" parts.
2. If the request is a feature or task, tell the user to use `/issue` and stop.
3. Search for duplicates: `gh issue list --state all --limit 10 --search "<keywords>"`. Show matches and ask whether any is the same bug. If so, offer to add the new facts to it instead of creating an issue.
4. Collect what you can without asking: the commit (`git rev-parse --short HEAD`), the OS (`uname -sr`), and the Go or Rust version when a check is involved. Read the code or logs the user points to and name the exact file, function or test.
5. Ask the user, in one question round, only for what is missing: steps to reproduce from a clean state, expected and actual behavior, evidence, frequency and impact, and the IDE or browser version.
6. Check the draft against the Definition of Ready, item by item. Expected and actual behavior are one statement each. Steps are numbered and minimal. Put a guess about the cause only under `Suspected cause`.
7. If an item is still unmet, say which one. Do not create the issue unless the user insists, and then add the `needs-refinement` label.
8. Show the full draft: title (`<component>: <what is wrong>`), labels (`bug` plus a component label) and body. Write the body to a file in the scratchpad directory. After the user confirms, run `gh issue create --title "<title>" --label "<labels>" --body-file <file>` and print the URL.

Use the section headings from the Definition of Ready, exactly, so the issue matches the bug form. Write in short literal sentences with no filler. Never create an issue or a label without confirmation. If a label is missing, tell the user to run the label commands in the Definition of Ready.
