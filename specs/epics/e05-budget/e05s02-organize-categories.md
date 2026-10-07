# e05s02 — Organize groups and categories

**type:** feat · **risk:** P1 · **context:** domain · **bcps:** 5 · **scope:** S06

## 1. Business narrative

The defaults are a starting point. A user adds a "Pets" group with "Ração" and "Veterinário", renames "Lazer" to "Diversão", moves "Streaming" from "Contas e assinaturas" to "Lazer", and puts the groups they care about most at the top.

## 2. Actors

- **User**: on `/categorias`.

## 3. Preconditions

- e05s01 is done.

## 4. Trigger

Any create, rename or reorder action, or moving a category to another group.

## 5. Main flow

1. **Create group:** a name and a kind (Despesas or Receitas). It goes last.
2. **Rename group** (`/categorias/grupos/{id}`).
3. **Move group** ↑ or ↓ among the groups.
4. **Create category** inside a group. It goes last in that group.
5. **Edit category** (`/categorias/{id}`): rename it, or move it to another group **of the same kind**. Moved categories go last in the target group.
6. **Move category** ↑ or ↓ within its group.

## 6. Alternative flows

- **6a. Name empty or over 100 characters:** 400 with "Informe um nome com até 100 caracteres."
- **6b. Moving a category to a group of another kind:** rejected (B2) with "A categoria só pode ir para um grupo do mesmo tipo."
- **6c. Moving past either end:** nothing happens.
- **6d. Another user's group or category ID:** 404 (X2).

## 7. Business rules

- **B1:** there is only one level of groups, and every category belongs to exactly one group.
- **B2:** a category's kind is its group's kind, so moving between kinds is refused.
- Archiving, merging and deleting categories are handled in e08s06, because they depend on the planning envelope.

## 8. Data

No schema change. New queries rename and reposition rows, change a category's group, and find a row's neighbour.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `(*budget.Service).CreateGroup(ctx, userID, kind, name) (Group, error)` | |
| `(*budget.Service).RenameGroup(ctx, userID, id, name) error` | |
| `(*budget.Service).MoveGroup(ctx, userID, id string, up bool) error` | |
| `(*budget.Service).CreateCategory(ctx, userID, groupID, name) (Category, error)` | |
| `(*budget.Service).RenameCategory(ctx, userID, id, name) error` | |
| `(*budget.Service).MoveCategory(ctx, userID, id string, up bool) error` | Moves within its group |
| `(*budget.Service).SetCategoryGroup(ctx, userID, id, groupID) error` | Same kind only |
| Errors | `ErrInvalidName`, `ErrInvalidKind`, `ErrKindMismatch`, `ErrNotFound` |
| `POST /categorias/grupos`, `GET+POST /categorias/grupos/{id}`, `POST /categorias/grupos/{id}/mover?dir=` | |
| `POST /categorias` (with `grupo`), `GET+POST /categorias/{id}`, `POST /categorias/{id}/mover?dir=` | Redirect 303 to `/categorias` |

## 10. Non-functional

Each action is a single transaction.

## 11. Security & privacy

- New names are sealed with the budget key.
- Every query filters by `user_id`, and there's an isolation test.
- Forms use POST and are protected by `CrossOriginProtection`.

## 12. Dependencies (slopcheck)

None new.

## 13. Requirements

#### ADDED: R1: Create and rename groups and categories
#### ADDED: R2: Reorder groups, and categories within a group
#### ADDED: R3: Move a category to another group of the same kind (B2)

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `budget.validName` | One rule for every name: trimmed, 1 to 100 characters. |

## 15. Implementation steps

1. Groups: create, rename and move. → verify: `go test -race ./budget/... -run TestGroups`
2. Categories: create, rename, move, and change group (same kind only). → verify: `go test -race ./budget/... -run 'TestCategoryCRUD|TestReorder'`
3. Web: forms on `/categorias` and the two edit pages. → verify: `go test -race ./web/... -run TestOrganizeCategories`
4. Full gate. → verify: `make preflight`

## 16. Risks

- **The categories page gets crowded.** Mitigation: renaming and changing group happen on edit pages, and the list keeps only ↑/↓ and the add forms.

## 17. Acceptance criteria

```gherkin
Feature: Organize categories

  Scenario: New group with categories
    When the user creates expense group "Pets" with "Ração"
    Then "Pets" is the last expense group with "Ração" inside it

  Scenario: Move category between groups of the same kind
    When the user moves "Streaming" to "Lazer"
    Then "Streaming" is the last category of "Lazer"

  Scenario: Kinds never mix
    When the user moves "Mercado" to "Receitas"
    Then it is rejected
```

## 18. Out of scope

- Archiving, merging and deleting (e08s06).
- Drag and drop.

## 19. Verification Script (Step-by-Step)

1. Run `make run`, sign in, and open **Categorias**.
2. Create the expense group "Pets", then add "Ração" to it. Both appear.
3. Click "Lazer" and rename it to "Diversão".
4. Click "Streaming" and move it to "Diversão". It appears at the end of "Diversão".
5. Try moving "Mercado" to "Receitas". It's refused with the message.
6. Use ↑/↓ on a group and on a category. Their order changes.
7. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S06.
- Decisions: `specs/planning-context.yaml` → `domain.budget`, `domain.invariants.budget` (B1, B2).
- Code and tests carry `// story: e05s02`.
