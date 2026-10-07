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
