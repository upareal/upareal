// story: e10s01
package planning

import (
	"context"
	"fmt"
)

// Export is every planning row of one user.
type Export struct {
	Mappings []ExportMapping `json:"mappings"`
}

// ExportMapping links a ledger account to a budget category.
type ExportMapping struct {
	AccountID  string `json:"account_id"`
	CategoryID string `json:"category_id"`
	UpdatedAt  string `json:"updated_at"`
}

// ExportUserData returns every planning row of userID.
func (s *Service) ExportUserData(ctx context.Context, userID string) (Export, error) {
	rows, err := s.q.ExportMappings(ctx, userID)
	if err != nil {
		return Export{}, fmt.Errorf("planning: export mappings: %w", err)
	}
	ex := Export{Mappings: make([]ExportMapping, 0, len(rows))}
	for _, r := range rows {
		ex.Mappings = append(ex.Mappings, ExportMapping{AccountID: r.AccountID, CategoryID: r.CategoryID, UpdatedAt: r.UpdatedAt})
	}
	return ex, nil
}
