// story: e13s01
package ledger

import (
	"context"
	"fmt"
)

// Stats is the ledger's public aggregate: numbers only.
type Stats struct {
	Accounts      int64 // user-created accounts (system accounts excluded)
	Transactions  int64 // live transactions (deleted excluded)
	ImportedLines int64
}

// PublicStats counts across all users (the X2 exception: aggregate numbers
// only, nothing per user, nothing decrypted).
func (s *Service) PublicStats(ctx context.Context) (Stats, error) {
	var st Stats
	var err error
	if st.Accounts, err = s.q.CountUserAccounts(ctx); err != nil {
		return Stats{}, fmt.Errorf("ledger: public stats: accounts: %w", err)
	}
	if st.Transactions, err = s.q.CountLiveTransactions(ctx); err != nil {
		return Stats{}, fmt.Errorf("ledger: public stats: transactions: %w", err)
	}
	if st.ImportedLines, err = s.q.CountImportLines(ctx); err != nil {
		return Stats{}, fmt.Errorf("ledger: public stats: import lines: %w", err)
	}
	return st, nil
}
