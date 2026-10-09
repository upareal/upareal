// story: e08s02 e08s03 e08s04 e08s05 e14s01
package planning

import (
	"context"
	"fmt"
	"time"

	"github.com/upareal/upareal/budget"
	"github.com/upareal/upareal/ledger"
)

// CategoryPlan is one category in a month. Expense categories use Spent,
// CarryOver and Available; income categories use Received.
type CategoryPlan struct {
	Amount    int64 // planned (budget)
	CarryOver int64 // expense: Σ (amount - spent) of earlier months; negative = overspent (e08s03)
	Spent     int64 // expense: activity on mapped accounts; refunds lower it
	Available int64 // expense: Amount + CarryOver - Spent
	Received  int64 // income: money in on mapped accounts
}

// MonthPlan is the computed envelope view of a month (P4: never stored).
// Received, Assigned and ToBeAssigned are cumulative through the month.
type MonthPlan struct {
	Categories       map[string]CategoryPlan // by category ID
	UnmappedSpent    int64                   // Sem categoria this month, expense side (P3)
	UnmappedReceived int64                   // Sem categoria this month, income side (P3)
	Received         int64                   // money in: income + Saldo inicial inflows
	Assigned         int64                   // amounts assigned to expense categories
	ToBeAssigned     int64                   // Received - Assigned - Sem categoria spending
	Cards            []CardPlan              // cards with debt at the end of the month (e08s04)
	Cash             int64                   // on-budget non-card balances at the end of the month
	Uncovered        int64                   // card debt that Cash cannot back (Sem cobertura)
}

// CardPlan is a credit card's payment envelope: its debt, already set aside
// because card spending reduced its categories and A distribuir excludes it.
type CardPlan struct {
	AccountID, Name string
	Due             int64 // Para pagar
}

// tally is ledger activity grouped by envelope.
type tally struct {
	byCategory                      map[string]int64 // expense: spent; income: received
	unmappedSpent, unmappedReceived int64
	moneyIn                         int64 // income + equity (Saldo inicial) + off-budget inflows
}

// Month computes userID's plan for m from budget amounts, mappings and
// ledger activity on every income, expense and equity account, including
// archived and system ones. Nothing is stored, so editing a past month
// recomputes every later one (P4).
func (s *Service) Month(ctx context.Context, userID string, m budget.Month) (MonthPlan, error) {
	amounts, err := s.bud.OpenMonth(ctx, userID, m)
	if err != nil {
		return MonthPlan{}, fmt.Errorf("planning: amounts: %w", err)
	}
	return s.month(ctx, userID, m, amounts)
}

// PeekMonth computes the same plan as Month without opening the month, so it
// never writes: an unopened month has no planned amounts. opened reports
// whether the month was opened in the budget (e14s01).
func (s *Service) PeekMonth(ctx context.Context, userID string, m budget.Month) (p MonthPlan, opened bool, err error) {
	amounts, opened, err := s.bud.Amounts(ctx, userID, m)
	if err != nil {
		return MonthPlan{}, false, fmt.Errorf("planning: amounts: %w", err)
	}
	p, err = s.month(ctx, userID, m, amounts)
	return p, opened, err
}

func (s *Service) month(ctx context.Context, userID string, m budget.Month, amounts map[string]int64) (MonthPlan, error) {
	before, err := s.bud.AmountsBefore(ctx, userID, m)
	if err != nil {
		return MonthPlan{}, fmt.Errorf("planning: earlier amounts: %w", err)
	}
	groups, err := s.bud.Categories(ctx, userID)
	if err != nil {
		return MonthPlan{}, fmt.Errorf("planning: categories: %w", err)
	}
	accounts, err := s.allAccounts(ctx, userID)
	if err != nil {
		return MonthPlan{}, err
	}
	byID := make(map[string]ledger.Account, len(accounts))
	for _, a := range accounts {
		byID[a.ID] = a
	}
	kinds := map[string]string{}
	for _, g := range groups {
		for _, c := range g.Categories {
			kinds[c.ID] = g.Kind
		}
	}
	mapped, err := s.validMappings(ctx, userID, kinds, byID)
	if err != nil {
		return MonthPlan{}, err
	}
	now, err := s.tally(ctx, userID, byID, mapped, m.Start(), m.Next().Start())
	if err != nil {
		return MonthPlan{}, err
	}
	past, err := s.tally(ctx, userID, byID, mapped, time.Time{}, m.Start())
	if err != nil {
		return MonthPlan{}, err
	}
	balances, err := s.ledg.Activity(ctx, userID, time.Time{}, m.Next().Start())
	if err != nil {
		return MonthPlan{}, fmt.Errorf("planning: balances: %w", err)
	}

	p := MonthPlan{Categories: map[string]CategoryPlan{}, UnmappedSpent: now.unmappedSpent, UnmappedReceived: now.unmappedReceived}
	for id, kind := range kinds {
		c := CategoryPlan{Amount: amounts[id]}
		if kind == "income" {
			c.Received = now.byCategory[id]
		} else {
			c.CarryOver = before[id] - past.byCategory[id]
			c.Spent = now.byCategory[id]
			c.Available = c.Amount + c.CarryOver - c.Spent
			p.Assigned += before[id] + amounts[id]
		}
		p.Categories[id] = c
	}
	p.Received = past.moneyIn + now.moneyIn
	p.ToBeAssigned = p.Received - p.Assigned - past.unmappedSpent - now.unmappedSpent
	p.addCards(accounts, balances)
	return p, nil
}

// addCards fills Cards, Cash and Uncovered from end-of-month balances.
func (p *MonthPlan) addCards(accounts []ledger.Account, balances map[string]int64) {
	var due int64
	for _, a := range accounts {
		if (a.Type != "asset" && a.Type != "liability") || !a.OnBudget {
			continue
		}
		balance := balances[a.ID]
		if a.IsCard && balance < 0 {
			p.Cards = append(p.Cards, CardPlan{AccountID: a.ID, Name: a.Name, Due: -balance})
			due -= balance
			continue
		}
		p.Cash += balance // includes an overpaid card's credit
	}
	reserved := p.Cash - max(p.ToBeAssigned, 0)
	for _, c := range p.Categories {
		reserved -= max(c.Available, 0)
	}
	p.Uncovered = min(max(due-reserved, 0), due)
}

// validMappings returns account → category for mappings whose category is
// active and of the account's kind; stale ones are ignored (X1).
func (s *Service) validMappings(ctx context.Context, userID string, kinds map[string]string, accounts map[string]ledger.Account) (map[string]string, error) {
	rows, err := s.q.ListMappings(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("planning: mappings: %w", err)
	}
	mapped := map[string]string{}
	for _, r := range rows {
		if a, ok := accounts[r.AccountID]; ok && kinds[r.CategoryID] == categoryKind(a) {
			mapped[r.AccountID] = r.CategoryID
		}
	}
	return mapped, nil
}

// tally groups the money crossing the budget boundary in [from, to) by
// envelope (e08s05). Expense: debit-positive spending; income and equity:
// money in; off-budget accounts: debits are spending, credits money in.
func (s *Service) tally(ctx context.Context, userID string, accounts map[string]ledger.Account, mapped map[string]string, from, to time.Time) (tally, error) {
	flows, err := s.ledg.BudgetFlows(ctx, userID, from, to)
	if err != nil {
		return tally{}, fmt.Errorf("planning: budget flows: %w", err)
	}
	t := tally{byCategory: map[string]int64{}}
	spend := func(accountID string, amount int64) {
		if catID, ok := mapped[accountID]; ok {
			t.byCategory[catID] += amount
		} else {
			t.unmappedSpent += amount
		}
	}
	for accountID, f := range flows {
		net := f.Debit + f.Credit
		switch accounts[accountID].Type {
		case "equity":
			t.moneyIn -= net
		case "income":
			t.moneyIn -= net
			if catID, ok := mapped[accountID]; ok {
				t.byCategory[catID] -= net
			} else {
				t.unmappedReceived -= net
			}
		case "expense":
			spend(accountID, net)
		case "asset", "liability": // off-budget (on-budget legs are never flows)
			spend(accountID, f.Debit)
			t.moneyIn -= f.Credit
		}
	}
	return t, nil
}

// allAccounts returns every account of userID, active first, then archived.
func (s *Service) allAccounts(ctx context.Context, userID string) ([]ledger.Account, error) {
	active, err := s.ledg.Accounts(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("planning: accounts: %w", err)
	}
	archived, err := s.ledg.ArchivedAccounts(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("planning: archived accounts: %w", err)
	}
	return append(active, archived...), nil
}
