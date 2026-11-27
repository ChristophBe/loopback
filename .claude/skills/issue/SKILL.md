---
name: issue
description: Create a GitHub issue for a feature, task or lesson content that meets the project's Definition of Ready. Use when the user wants to file, write up or refine a non-bug issue. Use /bug for defects.
argument-hint: <short description of the feature, task or content>
allowed-tools: Read, Grep, Write, Bash(gh issue list:*), Bash(gh issue create:*), Bash(gh issue view:*), Bash(gh label list:*)
---

Create a GitHub issue for the request in `$ARGUMENTS` that passes the Definition of Ready.

1. Read `docs/definition-of-ready.md`. Use its type labels, component labels, section headings and rules.
2. Decide the type: feature (`enhancement`), task (`task`) or content (`enhancement` plus `content`). If it is a defect, tell the user to use `/bug` and stop.
3. Search for duplicates: `gh issue list --state all --limit 10 --search "<keywords>"`. Show matches and ask whether any is the same work or a blocker.
4. Ground the issue in the repo. Read the relevant parts of `goal.md`, `mvp.md`, `architecture.md` and `brand/README.md`, and name the real files, endpoints, specs and components. Never invent names.
5. Fill every section for the type from the Definition of Ready. Ask the user, in one question round, only for mandatory facts you cannot derive.
6. Check the draft against the Definition of Ready, item by item. If it is too large for one pull request, propose a split into issues and a parent issue with a task list.
7. If an item is still unmet, say which one. Do not create the issue unless the user insists, and then add the `needs-refinement` label.
8. Show the full draft: title, labels and body. Write the body to a file in the scratchpad directory. After the user confirms, run `gh issue create --title "<title>" --label "<labels>" --body-file <file>` and print the URL. For a split, create the children first and the parent last.

Use the section headings from the Definition of Ready, exactly, so the issue matches the issue forms. Write in short literal sentences with no filler. Never create an issue or a label without confirmation. If a label is missing, tell the user to run the label commands in the Definition of Ready.
