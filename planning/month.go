// story: e08s02
package planning

import (
	"context"
	"fmt"

	"github.com/upareal/upareal/budget"
)

// CategoryPlan is one category in a month. Expense categories use Spent and
// Available; income categories use Received.
type CategoryPlan struct {
	Amount    int64 // planned (budget)
	Spent     int64 // expense: activity on mapped accounts; refunds lower it
	Available int64 // expense: Amount - Spent; negative = overspent
	Received  int64 // income: money in on mapped accounts
}

// MonthPlan is the computed envelope view of a month (P4: never stored).
type MonthPlan struct {
	Categories       map[string]CategoryPlan // by category ID
	UnmappedSpent    int64                   // Sem categoria, expense side (P3)
	UnmappedReceived int64                   // Sem categoria, income side (P3)
	Received         int64                   // all income received in the month
	Assigned         int64                   // sum of expense category amounts
	ToBeAssigned     int64                   // Received - Assigned (e08s03 carries it over)
}

// Month computes userID's plan for m from budget amounts, mappings and the
// month's ledger activity on every income/expense account, including
// archived and system ("A categorizar") ones.
func (s *Service) Month(ctx context.Context, userID string, m budget.Month) (MonthPlan, error) {
	amounts, err := s.bud.OpenMonth(ctx, userID, m)
	if err != nil {
		return MonthPlan{}, fmt.Errorf("planning: amounts: %w", err)
	}
	groups, err := s.bud.Categories(ctx, userID)
	if err != nil {
		return MonthPlan{}, fmt.Errorf("planning: categories: %w", err)
	}
	types, err := s.accountTypes(ctx, userID)
	if err != nil {
		return MonthPlan{}, err
	}
	activity, err := s.ledg.Activity(ctx, userID, m.Start(), m.Next().Start())
	if err != nil {
		return MonthPlan{}, fmt.Errorf("planning: activity: %w", err)
	}
	rows, err := s.q.ListMappings(ctx, userID)
	if err != nil {
		return MonthPlan{}, fmt.Errorf("planning: mappings: %w", err)
	}

	p := MonthPlan{Categories: map[string]CategoryPlan{}}
	kinds := map[string]string{}
	for _, g := range groups {
		for _, c := range g.Categories {
			kinds[c.ID] = g.Kind
			p.Categories[c.ID] = CategoryPlan{Amount: amounts[c.ID]}
			if g.Kind == "expense" {
				p.Assigned += amounts[c.ID]
			}
		}
	}
	mapped := map[string]string{}
	for _, r := range rows {
		if kinds[r.CategoryID] == types[r.AccountID] { // stale or mismatched mappings are ignored (X1)
			mapped[r.AccountID] = r.CategoryID
		}
	}
	for accountID, amount := range activity {
		typ := types[accountID]
		if typ != "income" && typ != "expense" {
			continue
		}
		catID, ok := mapped[accountID]
		c := p.Categories[catID]
		switch {
		case typ == "income":
			p.Received -= amount
			if !ok {
				p.UnmappedReceived -= amount
				continue
			}
			c.Received -= amount
		case !ok:
			p.UnmappedSpent += amount
			continue
		default:
			c.Spent += amount
		}
		p.Categories[catID] = c
	}
	for id, c := range p.Categories {
		if kinds[id] == "expense" {
			c.Available = c.Amount - c.Spent
			p.Categories[id] = c
		}
	}
	p.ToBeAssigned = p.Received - p.Assigned
	return p, nil
}

// accountTypes maps every account of userID, active or archived, to its type.
func (s *Service) accountTypes(ctx context.Context, userID string) (map[string]string, error) {
	active, err := s.ledg.Accounts(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("planning: accounts: %w", err)
	}
	archived, err := s.ledg.ArchivedAccounts(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("planning: archived accounts: %w", err)
	}
	types := make(map[string]string, len(active)+len(archived))
	for _, a := range append(active, archived...) {
		types[a.ID] = a.Type
	}
	return types, nil
}
