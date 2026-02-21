---
name: github
description: Interact with GitHub using the `gh` CLI.
metadata: {"openclaw":{"emoji":"🐙","requires":{"bins":["gh"]},"install":[{"id":"brew","kind":"brew","formula":"gh","label":"Install GitHub CLI (brew)"},{"id":"apt","kind":"apt","package":"gh","label":"Install GitHub CLI (apt)"}]}}
---

# GitHub Skill

Use the `gh` CLI to interact with GitHub. Always specify `--repo owner/repo` when not in a git directory.

## Pull Requests

Check CI status on a PR:
```bash
gh pr checks 55 --repo owner/repo
```

List recent workflow runs:
```bash
gh run list --repo owner/repo --limit 10
```

View a run:
```bash
gh run view <run-id> --repo owner/repo
```

## Issues

List issues:
```bash
gh issue list --repo owner/repo
```

Create issue:
```bash
gh issue create --repo owner/repo --title "Bug" --body "Description"
```

## API

The `gh api` command is useful for advanced queries:
```bash
gh api repos/owner/repo/pulls/55 --jq '.title, .state'
```
