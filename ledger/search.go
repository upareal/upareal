// story: e06s05
package ledger

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/upareal/upareal/ledger/internal/store"
)

const (
	maxSearchResults = 500
	maxSearchYears   = 5
)

// ErrInvalidSearch means the search range or amounts are not usable.
var ErrInvalidSearch = errors.New("ledger: invalid search")

// SearchQuery filters transactions. The range is required (privacy: every
// search is bounded); zero Min/Max and empty AccountID/Text mean "any".
type SearchQuery struct {
	From, To  time.Time
	AccountID string
	Min, Max  int64 // on the transaction amount (sum of positive legs)
	Text      string
}

// SearchResult is one matching transaction.
type SearchResult struct {
	TransactionID string
	At            time.Time
	Description   string
	Accounts      []string // names of every account in the transaction
	Amount        int64    // sum of the positive legs
	LinkAccountID string   // first asset/liability account, for the edit page
}

// Search returns userID's matching transactions, newest first, at most 500;
// the bool reports that more matched.
func (s *Service) Search(ctx context.Context, userID string, q SearchQuery) ([]SearchResult, bool, error) {
	if err := q.validate(); err != nil {
		return nil, false, err
	}
	rows, err := s.q.SearchTransactions(ctx, store.SearchTransactionsParams{
		UserID: userID, FromAt: stamp(q.From), ToAt: stamp(q.To), AccountID: q.AccountID, MinAmount: q.Min, MaxAmount: q.Max,
	})
	if err != nil {
		return nil, false, fmt.Errorf("ledger: search: %w", err)
	}
	if len(rows) == 0 {
		return nil, false, nil
	}
	dk, err := s.userKey(ctx, s.q, userID)
	if err != nil {
		return nil, false, err
	}
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	legs, err := s.q.LegsOfTransactions(ctx, store.LegsOfTransactionsParams{UserID: userID, TransactionIds: ids})
	if err != nil {
		return nil, false, fmt.Errorf("ledger: search legs: %w", err)
	}
	names, links := map[string][]string{}, map[string]string{}
	for _, l := range legs {
		name, err := dk.Open(fieldName, l.Name)
		if err != nil {
			return nil, false, err
		}
		names[l.TransactionID] = append(names[l.TransactionID], name)
		if links[l.TransactionID] == "" && !l.SystemRole.Valid && (l.Type == "asset" || l.Type == "liability") {
			links[l.TransactionID] = l.AccountID
		}
	}
	var out []SearchResult
	for _, r := range rows {
		desc, err := dk.Open(fieldDescription, r.Description)
		if err != nil {
			return nil, false, err
		}
		if !matchesText(q.Text, desc, names[r.ID]) {
			continue
		}
		if len(out) == maxSearchResults {
			return out, true, nil
		}
		at, err := time.Parse(time.RFC3339, r.OccurredAt)
		if err != nil {
			return nil, false, fmt.Errorf("ledger: transaction time %q: %w", r.OccurredAt, err)
		}
		out = append(out, SearchResult{TransactionID: r.ID, At: at, Description: desc, Accounts: names[r.ID], Amount: r.Amount, LinkAccountID: links[r.ID]})
	}
	return out, false, nil
}

func (q SearchQuery) validate() error {
	switch {
	case q.From.IsZero() || q.To.IsZero():
		return fmt.Errorf("%w: a date range is required", ErrInvalidSearch)
	case !q.From.Before(q.To):
		return fmt.Errorf("%w: the start must be before the end", ErrInvalidSearch)
	case q.To.Sub(q.From) > maxSearchYears*366*24*time.Hour:
		return fmt.Errorf("%w: the range is limited to %d years", ErrInvalidSearch, maxSearchYears)
	case q.Min < 0 || q.Max < 0 || (q.Max > 0 && q.Min > q.Max):
		return fmt.Errorf("%w: minimum and maximum must be positive, minimum below maximum", ErrInvalidSearch)
	}
	return nil
}

// matchesText reports whether text appears in the description or any account
// name, ignoring case and accents. Empty text matches everything.
func matchesText(text, description string, accounts []string) bool {
	needle := fold(strings.TrimSpace(text))
	if needle == "" {
		return true
	}
	if strings.Contains(fold(description), needle) {
		return true
	}
	for _, a := range accounts {
		if strings.Contains(fold(a), needle) {
			return true
		}
	}
	return false
}

// fold lowercases s and strips accents ("Água" → "agua") for comparisons.
func fold(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		out = s
	}
	return strings.ToLower(out)
}
