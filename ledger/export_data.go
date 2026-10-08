// story: e10s01
package ledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// Export is every ledger row of one user, decrypted (LGPD access and
// portability). Wrapped keys and blind indexes are not exported.
type Export struct {
	Accounts     []ExportAccount     `json:"accounts"`
	Transactions []ExportTransaction `json:"transactions"`
	History      []ExportHistory     `json:"history"`
	ImportLines  []ExportImportLine  `json:"import_lines"`
}

// ExportAccount is one account, archived and system ones included.
type ExportAccount struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Name       string `json:"name"`
	Position   int64  `json:"position"`
	SystemRole string `json:"system_role,omitempty"`
	OnBudget   bool   `json:"on_budget"`
	IsCard     bool   `json:"is_card"`
	ArchivedAt string `json:"archived_at,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// ExportTransaction is one transaction with its legs, deleted ones included.
type ExportTransaction struct {
	ID          string      `json:"id"`
	OccurredAt  string      `json:"occurred_at"`
	Description string      `json:"description"`
	CreatedAt   string      `json:"created_at"`
	DeletedAt   string      `json:"deleted_at,omitempty"`
	Legs        []ExportLeg `json:"legs"`
}

// ExportLeg is one leg: debit-positive centavos on an account.
type ExportLeg struct {
	AccountID      string `json:"account_id"`
	AmountCentavos int64  `json:"amount_centavos"`
}

// ExportHistory is one previous version of a transaction.
type ExportHistory struct {
	ID            string          `json:"id"`
	TransactionID string          `json:"transaction_id"`
	Action        string          `json:"action"`
	ChangedAt     string          `json:"changed_at"`
	Snapshot      json.RawMessage `json:"snapshot"`
}

// ExportImportLine is one imported statement line's extra fields.
type ExportImportLine struct {
	ID            string          `json:"id"`
	AccountID     string          `json:"account_id"`
	TransactionID string          `json:"transaction_id"`
	Raw           json.RawMessage `json:"raw"`
	CreatedAt     string          `json:"created_at"`
}

// ExportUserData returns every ledger row of userID, decrypted.
func (s *Service) ExportUserData(ctx context.Context, userID string) (Export, error) {
	ex := Export{Accounts: []ExportAccount{}, Transactions: []ExportTransaction{}, History: []ExportHistory{}, ImportLines: []ExportImportLine{}}
	wrapped, err := s.q.GetUserKey(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return ex, nil // never used the ledger
	}
	if err != nil {
		return Export{}, fmt.Errorf("ledger: export: load data key: %w", err)
	}
	dk, err := s.kr.UnwrapDataKey(userID, wrapped)
	if err != nil {
		return Export{}, err
	}
	fail := func(what string, err error) (Export, error) { return Export{}, fmt.Errorf("ledger: export %s: %w", what, err) }

	accounts, err := s.q.ExportAccounts(ctx, userID)
	if err != nil {
		return fail("accounts", err)
	}
	for _, a := range accounts {
		name, err := dk.Open(fieldName, a.Name)
		if err != nil {
			return fail("account name", err)
		}
		ex.Accounts = append(ex.Accounts, ExportAccount{ID: a.ID, Type: a.Type, Name: name, Position: a.Position,
			SystemRole: a.SystemRole.String, OnBudget: a.OnBudget == 1, IsCard: a.IsCard == 1, ArchivedAt: a.ArchivedAt.String, CreatedAt: a.CreatedAt})
	}

	legs, err := s.q.ExportLegs(ctx, userID)
	if err != nil {
		return fail("legs", err)
	}
	byTx := map[string][]ExportLeg{}
	for _, l := range legs {
		byTx[l.TransactionID] = append(byTx[l.TransactionID], ExportLeg{AccountID: l.AccountID, AmountCentavos: l.Amount})
	}
	txs, err := s.q.ExportTransactions(ctx, userID)
	if err != nil {
		return fail("transactions", err)
	}
	for _, t := range txs {
		desc, err := dk.Open(fieldDescription, t.Description)
		if err != nil {
			return fail("description", err)
		}
		ex.Transactions = append(ex.Transactions, ExportTransaction{ID: t.ID, OccurredAt: t.OccurredAt, Description: desc,
			CreatedAt: t.CreatedAt, DeletedAt: t.DeletedAt.String, Legs: byTx[t.ID]})
	}

	history, err := s.q.ExportHistory(ctx, userID)
	if err != nil {
		return fail("history", err)
	}
	for _, h := range history {
		snap, err := dk.Open(fieldHistory, h.Snapshot)
		if err != nil {
			return fail("history snapshot", err)
		}
		ex.History = append(ex.History, ExportHistory{ID: h.ID, TransactionID: h.TransactionID, Action: h.Action, ChangedAt: h.ChangedAt, Snapshot: json.RawMessage(snap)})
	}

	lines, err := s.q.ExportImportLines(ctx, userID)
	if err != nil {
		return fail("import lines", err)
	}
	for _, l := range lines {
		raw, err := dk.Open(fieldImport, l.Raw)
		if err != nil {
			return fail("import line", err)
		}
		ex.ImportLines = append(ex.ImportLines, ExportImportLine{ID: l.ID, AccountID: l.AccountID, TransactionID: l.TransactionID, Raw: json.RawMessage(raw), CreatedAt: l.CreatedAt})
	}
	return ex, nil
}
