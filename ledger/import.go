// story: e07s01
package ledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/upareal/upareal/ledger/importer"
	"github.com/upareal/upareal/ledger/internal/store"
)

const (
	importIDPurpose = "import-id"
	fieldImport     = "import"
)

// ImportResult says how many lines became transactions and how many were
// already imported into this account before.
type ImportResult struct{ Imported, Skipped int }

// QueuedLine is a transaction still waiting on an "A categorizar" account.
type QueuedLine struct {
	TransactionID string
	AccountID     string
	AccountName   string
	At            time.Time
	Description   string
	Amount        int64  // seen from AccountID: negative = money left
	Conta         string // the imported "conta" text, if any
}

// importRaw is the sealed extra data of an imported line.
type importRaw struct {
	Conta string `json:"conta,omitempty"`
}

// Import posts every line not imported into accountID before, against the
// matching "A categorizar" account, all-or-nothing. Duplicates are detected
// per account by a blind index of the line's id, even after a delete.
func (s *Service) Import(ctx context.Context, userID, accountID string, lines []importer.Line) (ImportResult, error) {
	var res ImportResult
	err := s.inTx(ctx, func(q *store.Queries) error {
		acc, err := s.account(ctx, q, userID, accountID)
		if err != nil {
			return err
		}
		if acc.Type != "asset" && acc.Type != "liability" {
			return fmt.Errorf("%w: statements are imported into asset or liability accounts", ErrInvalidEntry)
		}
		toExpense, err := s.systemAccount(ctx, q, userID, roleUncategorizedExpense)
		if err != nil {
			return err
		}
		toIncome, err := s.systemAccount(ctx, q, userID, roleUncategorizedIncome)
		if err != nil {
			return err
		}
		dk, err := s.userKey(ctx, q, userID)
		if err != nil {
			return err
		}
		for _, l := range lines {
			if l.Amount == 0 || l.ImportID == "" {
				return fmt.Errorf("%w: line %q needs an id and a non-zero amount", ErrInvalidEntry, l.ImportID)
			}
			idx := s.kr.BlindIndex(importIDPurpose, accountID+"\x00"+l.ImportID)
			n, err := q.ImportLineExists(ctx, store.ImportLineExistsParams{AccountID: accountID, IDIdx: idx})
			if err != nil {
				return fmt.Errorf("ledger: check import line: %w", err)
			}
			if n > 0 {
				res.Skipped++
				continue
			}
			counter := toIncome
			if l.Amount < 0 {
				counter = toExpense
			}
			txID, err := s.postTransaction(ctx, q, userID, l.At, l.Description, []Leg{
				{AccountID: accountID, Amount: l.Amount}, {AccountID: counter, Amount: -l.Amount},
			})
			if err != nil {
				return err
			}
			raw, err := json.Marshal(importRaw{Conta: l.Account})
			if err != nil {
				return err
			}
			lineID, sealed, err := newSealedField(dk, fieldImport, string(raw))
			if err != nil {
				return err
			}
			err = q.InsertImportLine(ctx, store.InsertImportLineParams{
				ID: lineID, UserID: userID, AccountID: accountID, TransactionID: txID, IDIdx: idx, Raw: sealed, CreatedAt: stamp(s.now()),
			})
			if err != nil {
				return fmt.Errorf("ledger: record import line: %w", err)
			}
			res.Imported++
		}
		return nil
	})
	if err != nil {
		return ImportResult{}, err
	}
	return res, nil
}

// ReviewQueue lists userID's transactions still on "A categorizar", newest
// first. It is derived from the ledger: assigning an account removes a line.
func (s *Service) ReviewQueue(ctx context.Context, userID string) ([]QueuedLine, error) {
	rows, err := s.q.ReviewQueue(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("ledger: review queue: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	dk, err := s.userKey(ctx, s.q, userID)
	if err != nil {
		return nil, err
	}
	out := make([]QueuedLine, 0, len(rows))
	for _, r := range rows {
		desc, err := dk.Open(fieldDescription, r.Description)
		if err != nil {
			return nil, err
		}
		name, err := dk.Open(fieldName, r.AccountName)
		if err != nil {
			return nil, err
		}
		at, err := time.Parse(time.RFC3339, r.OccurredAt)
		if err != nil {
			return nil, fmt.Errorf("ledger: transaction time %q: %w", r.OccurredAt, err)
		}
		ql := QueuedLine{TransactionID: r.ID, AccountID: r.AccountID, AccountName: name, At: at, Description: desc, Amount: r.Amount}
		if len(r.Raw) > 0 {
			raw, err := dk.Open(fieldImport, r.Raw)
			if err != nil {
				return nil, err
			}
			var extra importRaw
			if err := json.Unmarshal([]byte(raw), &extra); err != nil {
				return nil, fmt.Errorf("ledger: import line data: %w", err)
			}
			ql.Conta = extra.Conta
		}
		out = append(out, ql)
	}
	return out, nil
}

// systemAccount returns the ID of userID's account with a system role.
func (s *Service) systemAccount(ctx context.Context, q *store.Queries, userID, role string) (string, error) {
	id, err := q.GetSystemAccountID(ctx, store.GetSystemAccountIDParams{UserID: userID, SystemRole: sql.NullString{String: role, Valid: true}})
	if err != nil {
		return "", fmt.Errorf("ledger: %s account (run EnsureDefaults first): %w", role, err)
	}
	return id, nil
}
