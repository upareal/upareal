// story: e07s01
package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

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
	Summary string
	Lines   []queueLineView
}

type queueLineView struct{ Link, When, Account, Description, Amount, Conta string }

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
		lines, err := importer.ParseGenericCSV(file)
		if err != nil {
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
		queue, err := ledg.ReviewQueue(r.Context(), u.ID)
		if err != nil {
			renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível carregar a revisão.")
			return
		}
		v := queueView{}
		if imported := r.URL.Query().Get("importadas"); imported != "" {
			v.Summary = fmt.Sprintf("%s importadas, %s já existentes", imported, r.URL.Query().Get("existentes"))
		}
		for _, l := range queue {
			v.Lines = append(v.Lines, queueLineView{
				Link: "/lancamentos/" + l.TransactionID + "?conta=" + l.AccountID,
				When: l.At.In(saoPaulo).Format("02/01/2006 15:04"), Account: l.AccountName,
				Description: orNoDescription(l.Description), Amount: money.FormatBRL(l.Amount), Conta: l.Conta,
			})
		}
		renderHTML(w, r, http.StatusOK, reviewPage(v))
	})
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
		return []string{"O arquivo não tem linhas depois do cabeçalho."}
	}
	return []string{"Não foi possível ler o arquivo."}
}

func importFileError(err error) string {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		return fmt.Sprintf("Arquivo grande demais (máximo %d MB e %d linhas).", importer.MaxBytes>>20, importer.MaxLines)
	}
	return "Escolha um arquivo CSV."
}
