// story: e10s01 e16s01
package budget

import (
	"context"
	"fmt"
)

// Export is every budget row of one user, names decrypted. The wrapped data
// key is not exported.
type Export struct {
	Groups     []ExportGroup    `json:"groups"`
	Categories []ExportCategory `json:"categories"`
	Months     []ExportMonth    `json:"months"`
	Amounts    []ExportAmount   `json:"amounts"`
	Targets    []ExportTarget   `json:"targets"`
}

// ExportGroup is one group, archived ones included.
type ExportGroup struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Position   int64  `json:"position"`
	ArchivedAt string `json:"archived_at,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// ExportCategory is one category, archived ones included.
type ExportCategory struct {
	ID         string `json:"id"`
	GroupID    string `json:"group_id"`
	Name       string `json:"name"`
	Position   int64  `json:"position"`
	ArchivedAt string `json:"archived_at,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// ExportMonth is a month the user opened.
type ExportMonth struct {
	Month    string `json:"month"`
	OpenedAt string `json:"opened_at"`
}

// ExportAmount is the planned amount of a category in a month.
type ExportAmount struct {
	CategoryID     string `json:"category_id"`
	Month          string `json:"month"`
	AmountCentavos int64  `json:"amount_centavos"`
}

// ExportTarget is a category's savings target (e16s01).
type ExportTarget struct {
	CategoryID     string `json:"category_id"`
	AmountCentavos int64  `json:"amount_centavos"`
	DueMonth       string `json:"due_month"`
	CreatedAt      string `json:"created_at"`
}

// ExportUserData returns every budget row of userID, decrypted.
func (s *Service) ExportUserData(ctx context.Context, userID string) (Export, error) {
	ex := Export{Groups: []ExportGroup{}, Categories: []ExportCategory{}, Months: []ExportMonth{}, Amounts: []ExportAmount{}, Targets: []ExportTarget{}}
	dk, err := s.existingKey(ctx, userID)
	if err != nil || dk == nil {
		return ex, err
	}
	fail := func(what string, err error) (Export, error) {
		return Export{}, fmt.Errorf("budget: export %s: %w", what, err)
	}
	groups, err := s.q.ExportGroups(ctx, userID)
	if err != nil {
		return fail("groups", err)
	}
	for _, g := range groups {
		name, err := dk.Open(fieldName, g.Name)
		if err != nil {
			return fail("group name", err)
		}
		ex.Groups = append(ex.Groups, ExportGroup{ID: g.ID, Kind: g.Kind, Name: name, Position: g.Position, ArchivedAt: g.ArchivedAt.String, CreatedAt: g.CreatedAt})
	}
	cats, err := s.q.ExportCategories(ctx, userID)
	if err != nil {
		return fail("categories", err)
	}
	for _, c := range cats {
		name, err := dk.Open(fieldName, c.Name)
		if err != nil {
			return fail("category name", err)
		}
		ex.Categories = append(ex.Categories, ExportCategory{ID: c.ID, GroupID: c.GroupID, Name: name, Position: c.Position, ArchivedAt: c.ArchivedAt.String, CreatedAt: c.CreatedAt})
	}
	months, err := s.q.ExportMonths(ctx, userID)
	if err != nil {
		return fail("months", err)
	}
	for _, m := range months {
		ex.Months = append(ex.Months, ExportMonth{Month: m.Month, OpenedAt: m.OpenedAt})
	}
	amounts, err := s.q.ExportAmounts(ctx, userID)
	if err != nil {
		return fail("amounts", err)
	}
	for _, a := range amounts {
		ex.Amounts = append(ex.Amounts, ExportAmount{CategoryID: a.CategoryID, Month: a.Month, AmountCentavos: a.Amount})
	}
	targets, err := s.q.ExportTargets(ctx, userID)
	if err != nil {
		return fail("targets", err)
	}
	for _, t := range targets {
		ex.Targets = append(ex.Targets, ExportTarget{CategoryID: t.CategoryID, AmountCentavos: t.Amount, DueMonth: t.DueMonth, CreatedAt: t.CreatedAt})
	}
	return ex, nil
}
