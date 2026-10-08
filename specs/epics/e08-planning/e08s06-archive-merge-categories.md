# e08s06 — Archive and merge categories

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 5 · **scope:** S24

## 1. Business narrative

Budgets change. "Streaming" and "Assinaturas" become one category. "Pet" isn't needed any more. The user **merges** a category into another: its accounts, its planned amounts and its history move over, and nothing is lost. Or the user **archives** a category once its envelope is empty, and can **restore** it later.

## 2. Actors

- **User**: on `/categorias` and `/categorias/{id}`.

## 3. Preconditions

- e08s01 to e08s05 are done. Planning derives everything from current mappings, so an account's whole history follows its category.

## 4. Trigger

The user clicks **Arquivar**, **Juntar com…** or **Restaurar**.

## 5. Main flow

1. **Merge** `planning.MergeCategory(user, source, target)`:
   - Source and target must be active, different and of the same kind (B2).
   - Every mapping from source is moved to target.
   - `budget.MergeCategory` then, atomically:
     - adds each month's amount of source to target, creating the month row when missing;
     - deletes source's amounts;
     - archives source.
2. **Archive** `planning.ArchiveCategory(user, category)`:
   - An **expense** category needs `Disponível = 0` in the current São Paulo month. Otherwise the result is `ErrEnvelopeNotEmpty`. Income categories can always be archived.
   - Then its mappings are removed (its accounts become Sem categoria) and `budget.ArchiveCategory` sets `archived_at`.
3. **Restore** `budget.RestoreCategory`: clears `archived_at`, and the category comes back at the end of its group. The group must be active.
4. `/categorias` lists **Arquivadas** with a Restaurar button. The category page offers **Arquivar** and a **Juntar com** picker limited to same-kind active categories.

## 6. Alternative flows

- **6a. Archiving a non-empty expense envelope:** 400 "Só dá para arquivar com Disponível zero. Mova o dinheiro ou junte com outra categoria."
- **6b. Merging into itself, across kinds, or with an unknown, foreign or archived category:** 400 or 404, and nothing changes.
- **6c. The merge fails after the mappings moved** (two modules, two transactions): the identity still holds and the user retries. Mappings are moved first because that step is idempotent.

## 7. Business rules

- **Identity preserved:** a merge leaves `A distribuir + Σ Disponível + Σ Para pagar = Dinheiro` and A distribuir unchanged in **every** month. An archive leaves the current month's A distribuir unchanged.
- **P4 and X1:** nothing derived is stored. The mappings of an archived category are dropped.

## 8. Data

No schema change (`archived_at` already exists). There are new budget queries (archive, restore, list archived, merge amounts) and new planning queries (move and delete mappings by category).

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `(*budget.Service).ArchiveCategory / RestoreCategory(ctx, userID, id string) error` | |
| `(*budget.Service).ArchivedCategories(ctx, userID string) ([]Category, error)` | Names decrypted |
| `(*budget.Service).MergeCategory(ctx, userID, sourceID, targetID string) error` | `ErrKindMismatch`, `ErrNotFound`, `ErrSameCategory` |
| `(*planning.Service).ArchiveCategory(ctx, userID, id string) error` | `ErrEnvelopeNotEmpty` |
| `(*planning.Service).MergeCategory(ctx, userID, sourceID, targetID string) error` | |
| `POST /categorias/{id}/arquivar`, `/juntar` (`destino`), `/restaurar` | 303 to `/categorias` |

## 10. Non-functional

A merge runs one budget transaction and one planning update.

## 11. Security & privacy

- Everything filters by `user_id`, and both categories must belong to the user. There's an isolation test.

## 12. Dependencies (slopcheck)

None new.

## 13. Requirements

#### ADDED: R1: Merge a category into another of the same kind, moving accounts and amounts
#### ADDED: R2: Archive only an empty envelope; restore later

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `planning.ArchiveCategory` / `MergeCategory` | The envelope rule needs planning, but the category rows belong to budget. Planning orchestrates the two, and budget stays unaware of envelopes. |

## 15. Implementation steps

1. Budget archive, restore, list archived, merge (amounts summed per month), with isolation. → verify: `go test -race ./budget/... -run 'TestArchiveCategory|TestMergeCategory'`
2. Planning archive (empty-envelope guard, drop mappings) and merge (move mappings, then budget merge), with the identity preserved. → verify: `go test -race ./planning/... -run 'TestArchive|TestMerge'`
3. Web: Arquivar, Juntar com, and the Arquivadas list with Restaurar. → verify: `go test -race ./web/... -run TestArchiveMergeCategories`
4. Full gate. → verify: `make preflight`

## 16. Risks

- **The merge spans two modules without a shared transaction** (6c). The steps are ordered so that a partial failure stays consistent.

## 17. Acceptance criteria

```gherkin
Feature: Archive and merge categories

  Scenario: Merge
    Given "Streaming" R$ 50,00/month with Netflix mapped and "Assinaturas" R$ 30,00/month
    When Streaming is merged into Assinaturas
    Then Assinaturas has R$ 80,00 per month, Netflix maps to Assinaturas, Streaming is archived
    And A distribuir is unchanged in every month

  Scenario: Archive guard
    Given Pet has Disponível R$ 20,00
    When the user archives Pet
    Then it is refused until Disponível is zero
```

## 18. Out of scope

- Archiving and merging groups.
- Deleting categories (archive keeps history).

## 19. Verification Script (Step-by-Step)

1. Map an expense account (e.g. Netflix) to **Streaming** (create it if needed) and set Streaming to 50,00 this month. Note A distribuir.
2. Open Streaming in **Categorias**, choose **Juntar com → Assinaturas**. Assinaturas now shows the sum, Netflix maps to Assinaturas in **Mapa**, and A distribuir is unchanged.
3. Set **Pet** to 20,00 this month and click **Arquivar**. It's refused. Set Pet to 0,00 and archive it. It moves to **Arquivadas**.
4. Click **Restaurar** on Pet. It's back in its group.
5. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S24.
- Decisions: `specs/planning-context.yaml` → `domain.budget` ("archive categories only when their planning envelope is zero … merge moves amounts to the target category"), X1, P4.
- Code and tests carry `// story: e08s06`.
