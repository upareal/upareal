// story: e07s01 e07s02 e07s04 BUG-001 e14s02
package web

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/upareal/upareal/internal/money"
	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/ledger/importer"
	"github.com/upareal/upareal/users"
)

const multipartOverhead = 1 << 20 // form fields and boundaries around the file

type importView struct {
	AccountID, AccountName string
	Errors                 []string
}

type queueView struct {
	Summary, Error    string
	Lines             []queueLineView
	Expenses, Incomes []ledger.Account
}

type queueLineView struct {
	ID, Link, When, Account, Description, Amount, Conta string
	SuggestID, SuggestName, SuggestWhy                  string // e14s02: a suggestion, never applied
	Out                                                 bool // money left: offer expense accounts
}

func importGet(svc *users.Service, ledg *ledger.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		renderImport(w, r, ledg, u.ID, r.PathValue("id"), http.StatusOK, nil)
	})
}

func importPost(svc *users.Service, ledg *ledger.Service, limit *ipLimiter) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		id := r.PathValue("id")
		if !limit.allow(u.ID) {
			w.Header().Set("Retry-After", "60")
			renderError(w, r, http.StatusTooManyRequests, "Muitas importações", "Muitas importações. Aguarde um minuto e tente novamente.")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, importer.MaxBytes+multipartOverhead)
		file, _, err := r.FormFile("arquivo")
		if err != nil {
			renderImport(w, r, ledg, u.ID, id, http.StatusBadRequest, []string{importFileError(err)})
			return
		}
		defer func() { _ = file.Close() }()
		lines, format, err := importer.Parse(file) // OFX or CSV, detected by content
		if err != nil {
			kind, rejected := importFailureKind(err)
			// IDs and kinds only: parser messages can quote amounts or descriptions.
			slog.WarnContext(r.Context(), "import failed", "user_id", u.ID, "account_id", id,
				"format", format, "kind", kind, "rejected_lines", rejected)
			renderImport(w, r, ledg, u.ID, id, http.StatusBadRequest, parseErrors(err))
			return
		}
		res, err := ledg.Import(r.Context(), u.ID, id, lines)
		if err != nil {
			status, msg := entryError(err)
			renderImport(w, r, ledg, u.ID, id, status, []string{msg})
			return
		}
		q := url.Values{"importadas": {strconv.Itoa(res.Imported)}, "existentes": {strconv.Itoa(res.Skipped)}}
		http.Redirect(w, r, "/revisao?"+q.Encode(), http.StatusSeeOther)
	})
}

func reviewGet(svc *users.Service, ledg *ledger.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		summary := ""
		if imported := r.URL.Query().Get("importadas"); imported != "" {
			summary = fmt.Sprintf("%s importadas, %s já existentes", imported, r.URL.Query().Get("existentes"))
		}
		renderReview(w, r, ledg, u.ID, http.StatusOK, summary, "")
	})
}

// reviewAssign gives a queued line's whole amount to one expense/income account.
func reviewAssign(svc *users.Service, ledg *ledger.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		id := r.PathValue("id")
		tx, err := ledg.Transaction(r.Context(), u.ID, id)
		if err != nil {
			status, msg := editError(err)
			renderReview(w, r, ledg, u.ID, status, "", msg)
			return
		}
		var amount int64
		for _, l := range tx.Legs {
			if l.SystemRole != "" {
				amount = l.Amount
			}
		}
		if amount < 0 {
			amount = -amount
		}
		if err := ledg.Categorize(r.Context(), u.ID, id, []ledger.Part{{CounterID: r.FormValue("contra"), Amount: amount}}); err != nil {
			status, msg := editError(err)
			var mm ledger.SplitMismatch
			if errors.As(err, &mm) {
				msg = "Escolha uma conta de despesa para saídas e de receita para entradas."
			}
			renderReview(w, r, ledg, u.ID, status, "", msg)
			return
		}
		http.Redirect(w, r, "/revisao", http.StatusSeeOther)
	})
}

func renderReview(w http.ResponseWriter, r *http.Request, ledg *ledger.Service, userID string, status int, summary, errMsg string) {
	queue, err := ledg.ReviewQueue(r.Context(), userID)
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível carregar a revisão.")
		return
	}
	accounts, err := ledg.Accounts(r.Context(), userID)
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível carregar as contas.")
		return
	}
	v := queueView{Summary: summary, Error: errMsg}
	for _, a := range accounts {
		if a.SystemRole != "" {
			continue
		}
		switch a.Type {
		case "expense":
			v.Expenses = append(v.Expenses, a)
		case "income":
			v.Incomes = append(v.Incomes, a)
		}
	}
	suggestions, err := ledg.SuggestCounters(r.Context(), userID, queue)
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível carregar a revisão.")
		return
	}
	for _, l := range queue {
		sg := suggestions[l.TransactionID]
		why := ""
		if sg.Matches == 1 {
			why = "1 lançamento parecido"
		} else if sg.Matches > 1 {
			why = fmt.Sprintf("%d lançamentos parecidos", sg.Matches)
		}
		v.Lines = append(v.Lines, queueLineView{
			SuggestID: sg.AccountID, SuggestName: sg.AccountName, SuggestWhy: why,
			ID: l.TransactionID, Link: "/lancamentos/" + l.TransactionID + "?conta=" + l.AccountID,
			When: l.At.In(saoPaulo).Format("02/01/2006 15:04"), Account: l.AccountName,
			Description: orNoDescription(l.Description), Amount: money.FormatBRL(l.Amount), Conta: l.Conta, Out: l.Amount < 0,
		})
	}
	renderHTML(w, r, status, reviewPage(v))
}

func renderImport(w http.ResponseWriter, r *http.Request, ledg *ledger.Service, userID, id string, status int, errs []string) {
	acc, _, err := findAnyAccount(r, ledg, userID, id)
	if err != nil || (acc.Type != "asset" && acc.Type != "liability") {
		renderError(w, r, http.StatusNotFound, "Conta", "Conta não encontrada.")
		return
	}
	renderHTML(w, r, status, importPage(importView{AccountID: acc.ID, AccountName: acc.Name, Errors: errs}))
}

// parseErrors turns parser errors into pt-BR messages, one per bad line.
func parseErrors(err error) []string {
	var errs importer.Errors
	switch {
	case errors.As(err, &errs):
		out := make([]string, len(errs))
		for i, e := range errs {
			out[i] = fmt.Sprintf("Linha %d: %s", e.Line, e.Reason)
		}
		return out
	case errors.Is(err, importer.ErrTooLarge):
		return []string{fmt.Sprintf("Arquivo grande demais (máximo %d MB e %d linhas).", importer.MaxBytes>>20, importer.MaxLines)}
	case errors.Is(err, importer.ErrNoLines):
		return []string{"O arquivo não tem lançamentos."}
	case errors.Is(err, importer.ErrUnreadableOFX):
		// The user's own file: showing the parser's message is safe and lets
		// them (or us) see the cause (BUG-001).
		detail := strings.TrimPrefix(err.Error(), importer.ErrUnreadableOFX.Error()+": ")
		return []string{"Não foi possível ler o OFX. Baixe o extrato de novo no site do banco.", "Detalhe técnico: " + detail}
	}
	return []string{"Não foi possível ler o arquivo."}
}

// importFailureKind classifies a parse error for the log, with the number of
// rejected lines when there are any.
func importFailureKind(err error) (kind string, rejected int) {
	var errs importer.Errors
	switch {
	case errors.As(err, &errs):
		return "line_errors", len(errs)
	case errors.Is(err, importer.ErrTooLarge):
		return "too_large", 0
	case errors.Is(err, importer.ErrNoLines):
		return "no_lines", 0
	case errors.Is(err, importer.ErrUnreadableOFX):
		return "unreadable_ofx", 0
	}
	return "unreadable", 0
}

func importFileError(err error) string {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		return fmt.Sprintf("Arquivo grande demais (máximo %d MB e %d linhas).", importer.MaxBytes>>20, importer.MaxLines)
	}
	return "Escolha um arquivo CSV."
}
