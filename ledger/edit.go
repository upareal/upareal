// story: e06s04
package ledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/upareal/upareal/internal/vault"
	"github.com/upareal/upareal/ledger/internal/store"
)

const fieldHistory = "history"

// Errors for edits and deletes.
var (
	ErrTransactionNotFound = errors.New("ledger: transaction not found")
	ErrNotEditable         = errors.New("ledger: transactions on system accounts cannot be edited; delete or adjust the balance")
)

// Transaction is a decrypted, active transaction with typed legs.
type Transaction struct {
	ID          string
	At          time.Time
	Description string
	Legs        []TxLeg
}

// TxLeg is one leg with its account's type and system role ("" for user accounts).
type TxLeg struct {
	AccountID   string
	AccountType string
	SystemRole  string
	Amount      int64
}

// HistoryEntry is one past version of a transaction.
type HistoryEntry struct {
	At       time.Time   // when it changed
	Action   string      // "edit" or "delete"
	Previous Transaction // the version before the change (legs without types)
}

// snapshot is the sealed JSON stored in history.
type snapshot struct {
	At          time.Time `json:"at"`
	Description string    `json:"description"`
	Legs        []Leg     `json:"legs"`
}

// Transaction loads one of userID's active transactions.
func (s *Service) Transaction(ctx context.Context, userID, id string) (Transaction, error) {
	return s.loadTransaction(ctx, s.q, userID, id)
}

// History returns the past versions of one of userID's transactions, newest first.
func (s *Service) History(ctx context.Context, userID, id string) ([]HistoryEntry, error) {
	rows, err := s.q.ListHistory(ctx, store.ListHistoryParams{TransactionID: id, UserID: userID})
	if err != nil {
		return nil, fmt.Errorf("ledger: history: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	dk, err := s.userKey(ctx, s.q, userID)
	if err != nil {
		return nil, err
	}
	out := make([]HistoryEntry, 0, len(rows))
	for _, r := range rows {
		raw, err := dk.Open(fieldHistory, r.Snapshot)
		if err != nil {
			return nil, err
		}
		var snap snapshot
		if err := json.Unmarshal([]byte(raw), &snap); err != nil {
			return nil, fmt.Errorf("ledger: history snapshot: %w", err)
		}
		at, err := time.Parse(time.RFC3339, r.ChangedAt)
		if err != nil {
			return nil, fmt.Errorf("ledger: history time %q: %w", r.ChangedAt, err)
		}
		prev := Transaction{ID: id, At: snap.At, Description: snap.Description}
		for _, l := range snap.Legs {
			prev.Legs = append(prev.Legs, TxLeg{AccountID: l.AccountID, Amount: l.Amount})
		}
		out = append(out, HistoryEntry{At: at, Action: r.Action, Previous: prev})
	}
	return out, nil
}

func (s *Service) loadTransaction(ctx context.Context, q *store.Queries, userID, id string) (Transaction, error) {
	row, err := q.GetTransaction(ctx, store.GetTransactionParams{ID: id, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return Transaction{}, ErrTransactionNotFound
	}
	if err != nil {
		return Transaction{}, fmt.Errorf("ledger: load transaction: %w", err)
	}
	dk, err := s.userKey(ctx, q, userID)
	if err != nil {
		return Transaction{}, err
	}
	desc, err := dk.Open(fieldDescription, row.Description)
	if err != nil {
		return Transaction{}, err
	}
	at, err := time.Parse(time.RFC3339, row.OccurredAt)
	if err != nil {
		return Transaction{}, fmt.Errorf("ledger: transaction time %q: %w", row.OccurredAt, err)
	}
	legs, err := q.TransactionLegs(ctx, store.TransactionLegsParams{TransactionID: id, UserID: userID})
	if err != nil {
		return Transaction{}, fmt.Errorf("ledger: transaction legs: %w", err)
	}
	tx := Transaction{ID: id, At: at, Description: desc}
	for _, l := range legs {
		tx.Legs = append(tx.Legs, TxLeg{AccountID: l.AccountID, AccountType: l.Type, SystemRole: l.SystemRole.String, Amount: l.Amount})
	}
	return tx, nil
}

// recordHistory seals the current version of tx before it changes.
func (s *Service) recordHistory(ctx context.Context, q *store.Queries, dk *vault.DataKey, userID string, tx Transaction, action string) error {
	snap := snapshot{At: tx.At, Description: tx.Description}
	for _, l := range tx.Legs {
		snap.Legs = append(snap.Legs, Leg{AccountID: l.AccountID, Amount: l.Amount})
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("ledger: history snapshot: %w", err)
	}
	id, sealed, err := newSealedField(dk, fieldHistory, string(raw))
	if err != nil {
		return err
	}
	return q.InsertHistory(ctx, store.InsertHistoryParams{
		ID: id, TransactionID: tx.ID, UserID: userID, Action: action, ChangedAt: stamp(s.now()), Snapshot: sealed,
	})
}

// EditEntry replaces an expense/income transaction (one or more lines).
func (s *Service) EditEntry(ctx context.Context, userID, id string, e SplitEntry) error {
	if e.Total <= 0 {
		return fmt.Errorf("%w: total must be greater than zero", ErrInvalidEntry)
	}
	if err := validEntryText(e.At, e.Description); err != nil {
		return err
	}
	return s.inTx(ctx, func(q *store.Queries) error {
		cur, err := s.editableTransaction(ctx, q, userID, id)
		if err != nil {
			return err
		}
		legs, err := s.entryLegs(ctx, q, userID, e, 1)
		if err != nil {
			return err
		}
		return s.replaceTransaction(ctx, q, userID, cur, e.At, e.Description, legs)
	})
}

// EditTransfer replaces a transfer between own asset/liability accounts.
func (s *Service) EditTransfer(ctx context.Context, userID, id string, e TransferEntry) error {
	return s.inTx(ctx, func(q *store.Queries) error {
		cur, err := s.editableTransaction(ctx, q, userID, id)
		if err != nil {
			return err
		}
		legs, err := s.transferLegs(ctx, q, userID, e)
		if err != nil {
			return err
		}
		return s.replaceTransaction(ctx, q, userID, cur, e.At, e.Description, legs)
	})
}

// editableTransaction loads one of userID's active transactions first (so a
// foreign ID never reveals anything about accounts) and refuses those that
// touch system accounts.
func (s *Service) editableTransaction(ctx context.Context, q *store.Queries, userID, id string) (Transaction, error) {
	cur, err := s.loadTransaction(ctx, q, userID, id)
	if err != nil {
		return Transaction{}, err
	}
	for _, l := range cur.Legs {
		if l.SystemRole != "" {
			return Transaction{}, ErrNotEditable
		}
	}
	return cur, nil
}

// replaceTransaction is the edit path: it seals the previous version into
// history, then replaces time, description and legs under the same checks as
// postTransaction.
func (s *Service) replaceTransaction(ctx context.Context, q *store.Queries, userID string, cur Transaction, at time.Time, description string, legs []Leg) error {
	id := cur.ID
	if err := s.validateLegs(ctx, q, userID, legs); err != nil {
		return err
	}
	dk, err := s.userKey(ctx, q, userID)
	if err != nil {
		return err
	}
	if err := s.recordHistory(ctx, q, dk, userID, cur, "edit"); err != nil {
		return err
	}
	sealed, err := dk.Seal(fieldDescription, description)
	if err != nil {
		return err
	}
	if err := q.UpdateTransaction(ctx, store.UpdateTransactionParams{OccurredAt: stamp(at), Description: sealed, ID: id, UserID: userID}); err != nil {
		return fmt.Errorf("ledger: update transaction: %w", err)
	}
	if err := q.DeleteLegs(ctx, id); err != nil {
		return fmt.Errorf("ledger: replace legs: %w", err)
	}
	return writeLegs(ctx, q, id, legs)
}

// DeleteTransaction soft-deletes one of userID's transactions: it leaves every
// balance and register, and its previous version goes to history. Any
// transaction can be deleted, including opening balances and adjustments.
func (s *Service) DeleteTransaction(ctx context.Context, userID, id string) error {
	return s.inTx(ctx, func(q *store.Queries) error {
		cur, err := s.loadTransaction(ctx, q, userID, id)
		if err != nil {
			return err
		}
		dk, err := s.userKey(ctx, q, userID)
		if err != nil {
			return err
		}
		if err := s.recordHistory(ctx, q, dk, userID, cur, "delete"); err != nil {
			return err
		}
		return q.SoftDeleteTransaction(ctx, store.SoftDeleteTransactionParams{
			DeletedAt: sql.NullString{String: stamp(s.now()), Valid: true}, ID: id, UserID: userID,
		})
	})
}
