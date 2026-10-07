// story: e06s02 e06s04
package ledger

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/upareal/upareal/internal/money"
	"github.com/upareal/upareal/ledger/internal/store"
)

// ErrSplitMismatch means the lines do not add up to the total; errors.As
// with SplitMismatch gives the remaining amount.
var ErrSplitMismatch = errors.New("ledger: split lines do not add up to the total")

// SplitMismatch carries how much is still to distribute (negative = too much).
type SplitMismatch struct{ Remaining int64 }

func (m SplitMismatch) Error() string {
	return fmt.Sprintf("%v: remaining %s", ErrSplitMismatch, money.FormatBRL(m.Remaining))
}

func (m SplitMismatch) Unwrap() error { return ErrSplitMismatch }

// Part is one line of a split: an expense or income account and a positive amount.
type Part struct {
	CounterID string
	Amount    int64
}

// SplitEntry is one payment from an asset or liability account spread over
// several expense/income accounts. Total is the money that left the account.
type SplitEntry struct {
	AccountID   string
	Total       int64 // > 0
	At          time.Time
	Description string
	Parts       []Part // >= 2, each > 0
}

// RecordSplit posts one transaction: expense lines +A, income lines −A (they
// reduce the total, e.g. cashback), and the paying account −Total. The lines
// must add up to Total exactly; otherwise SplitMismatch says by how much.
func (s *Service) RecordSplit(ctx context.Context, userID string, e SplitEntry) (string, error) {
	if e.Total <= 0 {
		return "", fmt.Errorf("%w: total must be greater than zero", ErrInvalidEntry)
	}
	if err := validEntryText(e.At, e.Description); err != nil {
		return "", err
	}
	var id string
	err := s.inTx(ctx, func(q *store.Queries) error {
		legs, err := s.entryLegs(ctx, q, userID, e, 2)
		if err != nil {
			return err
		}
		id, err = s.postTransaction(ctx, q, userID, e.At, e.Description, legs)
		return err
	})
	return id, err
}

// entryLegs builds the legs of an entry with at least minLines lines: the
// paying account −Total and one leg per line; the lines must add up to Total.
func (s *Service) entryLegs(ctx context.Context, q *store.Queries, userID string, e SplitEntry, minLines int) ([]Leg, error) {
	acc, err := s.account(ctx, q, userID, e.AccountID)
	if err != nil {
		return nil, err
	}
	if acc.Type != "asset" && acc.Type != "liability" {
		return nil, fmt.Errorf("%w: the paying account must be an asset or liability", ErrInvalidEntry)
	}
	if len(e.Parts) < minLines {
		return nil, fmt.Errorf("%w: a split needs at least %d lines", ErrInvalidEntry, minLines)
	}
	legs, remaining, err := s.splitLegs(ctx, q, userID, e.Total, e.Parts)
	if err != nil {
		return nil, err
	}
	if remaining != 0 {
		return nil, SplitMismatch{Remaining: remaining}
	}
	return append(legs, Leg{AccountID: e.AccountID, Amount: -e.Total}), nil
}

// SplitRemaining says how much of total is still to distribute over parts
// (negative = distributed too much), for any number of lines. It writes nothing.
func (s *Service) SplitRemaining(ctx context.Context, userID string, total int64, parts []Part) (int64, error) {
	_, remaining, err := s.splitLegs(ctx, s.q, userID, total, parts)
	return remaining, err
}

// splitLegs turns lines into counter legs (direction from each counter's
// type) and returns total minus what they distribute.
func (s *Service) splitLegs(ctx context.Context, q *store.Queries, userID string, total int64, parts []Part) ([]Leg, int64, error) {
	legs := make([]Leg, 0, len(parts)+1)
	remaining := total
	for _, p := range parts {
		if p.Amount <= 0 {
			return nil, 0, fmt.Errorf("%w: every line must be greater than zero", ErrInvalidEntry)
		}
		counter, err := s.account(ctx, q, userID, p.CounterID)
		if err != nil {
			return nil, 0, err
		}
		signed := p.Amount
		switch counter.Type {
		case "expense":
		case "income":
			signed = -p.Amount
		default:
			return nil, 0, fmt.Errorf("%w: split lines must be income or expense accounts", ErrInvalidEntry)
		}
		next, overflow := addInt64(remaining, -signed)
		if overflow {
			return nil, 0, fmt.Errorf("%w: amounts overflow", ErrInvalidEntry)
		}
		remaining = next
		legs = append(legs, Leg{AccountID: p.CounterID, Amount: signed})
	}
	return legs, remaining, nil
}
