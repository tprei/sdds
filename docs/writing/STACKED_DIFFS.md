# Stacked Diffs

Use stacked diffs when one change is too large for a single reviewable PR.

The standard is the GitHub-visible shape of the work, not the local tool used to create it. Humans may use plain Git. Agents should use Graphite CLI when it is available. Both workflows must produce the same branch structure, PR titles, and PR descriptions.

## Required Shape

Each stack slice is a normal branch and a normal GitHub PR.

```txt
main
  -> sdds/notes-domain
      -> sdds/notes-api
          -> sdds/notes-mobile-list
              -> sdds/notes-mobile-api
```

GitHub PR bases must follow the same order:

```txt
PR 1: sdds/notes-domain      -> main
PR 2: sdds/notes-api         -> sdds/notes-domain
PR 3: sdds/notes-mobile-list -> sdds/notes-api
PR 4: sdds/notes-mobile-api  -> sdds/notes-mobile-list
```

Do not require reviewers to open Graphite. Review happens in GitHub.

## PR Titles

Use a position prefix for every stacked PR:

```txt
[1/4] Add note domain and SQLite schema
[2/4] Add note API routes
[3/4] Add mobile note list and search UI
[4/4] Connect mobile notes UI to API
```

## PR Description

Every stacked PR must include a stack section in the PR body:

```md
## Stack

1. #12 [1/4] Add note domain and SQLite schema
2. #13 [2/4] Add note API routes
3. #14 [3/4] Add mobile note list and search UI
4. #15 [4/4] Connect mobile notes UI to API

This PR is: 2 of 4.
Review order: #12 -> #13 -> #14 -> #15.

## Scope

This PR adds the HTTP routes for creating, listing, and searching notes.

## Depends On

- #12 for note domain types and SQLite schema.

## Intentionally Left Out

- Mobile UI.
- API client integration.
```

When PR numbers do not exist yet, use branch names. Update the stack section after PRs are opened.

## Plain Git Workflow

Use this when Graphite is unavailable or the author prefers plain Git:

```bash
git checkout main
git pull
git checkout -b sdds/notes-domain
```

Open the first PR against `main`.

For each next slice, branch from the previous slice:

```bash
git checkout sdds/notes-domain
git checkout -b sdds/notes-api
```

Open the next PR against its parent branch, not against `main`.

## Graphite Workflow

Use Graphite CLI as the preferred helper for agent-driven stacks and for humans who want local stack management:

```bash
gt init
gt create sdds/notes-domain
gt create sdds/notes-api
gt create sdds/notes-mobile-list
gt log --stack
gt submit --cli --edit
```

Graphite helps create, restack, sync, and submit the branches. It does not replace the GitHub PR description. The GitHub-visible stack section is still required.

Use `gt submit --cli --edit` so PR metadata can be written from the terminal instead of the Graphite dashboard.

## Review Rules

- Keep each PR under 1,000 changed lines, excluding generated code.
- Keep each PR focused on one reviewable step.
- Multiple commits per PR are fine while developing.
- Each PR must be understandable in isolation and reviewable in order.
- CI must pass for the PR being merged.
- Merge from the bottom of the stack upward.
- Human review is required for every PR.

## Agent Rules

- If `gt` is available, use Graphite CLI for stack management.
- If `gt` is unavailable, use plain Git and preserve the same GitHub-visible structure.
- Always include the stack order in PR descriptions.
- Do not depend on the Graphite dashboard for reviewer context.
- Do not push to `main`.
- Do not merge PRs unless a human explicitly asks.
