# e15s02 — English contributor docs in the repository

**type:** docs · **risk:** P3 · **context:** repo · **bcps:** 1 · **scope:** — (new, user request 2026-10-09)

## 1. Business narrative

A developer who wants to help needs to set up the project, understand the modules and their rules, and know the workflow before the first pull request. Today this is spread across README, AGENTS.md, CONVENTIONS.md and specs/. Two short English Markdown files give one entry point. GitHub renders them; nothing runs in the app.

## 2. Actors

- **Contributor**: reads the repository on GitHub or locally.

## 5. Main flow

1. `CONTRIBUTING.md` (root, GitHub shows it on new issues and PRs):
   - setup and prerequisites, `make` commands, environment variables pointer to README;
   - workflow: branch, Conventional Commits with module scope, `make preflight`, never commit to main;
   - where to add things: a new page, a new query, a new migration;
   - the non-negotiables (money in centavos, `user_id` on every query, zero-sum, encryption helper, pt-BR UI / English code).
2. `docs/architecture.md`:
   - modules and their dependencies, module ownership of schemas, no cross-module tables;
   - domain invariants summary with a pointer to `specs/planning-context.yaml`;
   - time, money and encryption handling; request flow through `web/`.
3. README "Contributing" points to `CONTRIBUTING.md`.

## 7. Business rules

- English, per CONVENTIONS.md ("docs MUST be English").
- Link to the source of truth (CONVENTIONS.md, planning-context.yaml) instead of copying long rules.

## 13. Requirements

#### ADDED: R1: Contributor docs that explain setup, workflow and architecture

## 15. Implementation steps

1. Write `CONTRIBUTING.md` and `docs/architecture.md`; link them from README. → verify: `test -s CONTRIBUTING.md && test -s docs/architecture.md && grep -q CONTRIBUTING.md README.md`
2. Full gate. → verify: `make preflight`

## 17. Acceptance criteria

```gherkin
Feature: Contributor docs
  Scenario: New contributor
    Given a developer who opens the repository
    When they follow README → CONTRIBUTING.md
    Then they can set up, run the tests and open a correct first PR
    And docs/architecture.md explains the modules and their rules
```

## 18. Out of scope

- A docs site generator, godoc examples, ADR rewrites.

## 19. Verification Script (Step-by-Step)

1. Open `CONTRIBUTING.md` and `docs/architecture.md` on GitHub or in an editor. Check that they read well and that every link works.
2. Run `make preflight`. It ends green.

## 20. Traceability

- Docs live at `CONTRIBUTING.md` and `docs/architecture.md`.
