// story: e06s01
package ledger

import (
	"context"
	"fmt"
	"time"

	"github.com/upareal/upareal/ledger/internal/store"
)

// RegisterLine is one transaction as seen from one account.
type RegisterLine struct {
	TransactionID string
	At            time.Time
	Description   string
	Counterparts  []string // names of the other accounts in the transaction
	Amount        int64    // debit-positive change on this account
	Balance       int64    // debit-positive running balance after this line
}

// Register lists the transactions touching one of userID's accounts with
// occurred_at in [from, to), oldest first, with a running balance that starts
// from the balance before from.
func (s *Service) Register(ctx context.Context, userID, accountID string, from, to time.Time) ([]RegisterLine, error) {
	if _, err := s.account(ctx, s.q, userID, accountID); err != nil {
		return nil, err
	}
	dk, err := s.userKey(ctx, s.q, userID)
	if err != nil {
		return nil, err
	}
	running, err := s.q.BalanceBefore(ctx, store.BalanceBeforeParams{UserID: userID, AccountID: accountID, OccurredAt: stamp(from)})
	if err != nil {
		return nil, fmt.Errorf("ledger: opening balance: %w", err)
	}
	rows, err := s.q.RegisterEntries(ctx, store.RegisterEntriesParams{UserID: userID, AccountID: accountID, FromAt: stamp(from), ToAt: stamp(to)})
	if err != nil {
		return nil, fmt.Errorf("ledger: register: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	others, err := s.q.Counterparts(ctx, store.CounterpartsParams{UserID: userID, AccountID: accountID, TransactionIds: ids})
	if err != nil {
		return nil, fmt.Errorf("ledger: counterparts: %w", err)
	}
	names := map[string][]string{}
	for _, o := range others {
		name, err := dk.Open(fieldName, o.Name)
		if err != nil {
			return nil, err
		}
		names[o.TransactionID] = append(names[o.TransactionID], name)
	}
	lines := make([]RegisterLine, 0, len(rows))
	for _, r := range rows {
		desc, err := dk.Open(fieldDescription, r.Description)
		if err != nil {
			return nil, err
		}
		at, err := time.Parse(time.RFC3339, r.OccurredAt)
		if err != nil {
			return nil, fmt.Errorf("ledger: transaction %s time %q: %w", r.ID, r.OccurredAt, err)
		}
		running += r.Amount
		lines = append(lines, RegisterLine{TransactionID: r.ID, At: at, Description: desc,
			Counterparts: names[r.ID], Amount: r.Amount, Balance: running})
	}
	return lines, nil
}
