// story: e03s03
package web

import (
	"net/http"

	"github.com/upareal/upareal/users"
)

// CurrentTermsVersion identifies the current Termos de Uso + Política de
// Privacidade. Bump it whenever either text changes: every user is asked again.
const CurrentTermsVersion = "2026-10-07"

// appPage renders an app page for the current visitor. Signed-in users who
// have not accepted CurrentTermsVersion go to /termos/aceitar first.
func appPage(svc *users.Service, render func(w http.ResponseWriter, r *http.Request, u users.User, signedIn bool)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, signedIn, err := currentUser(r, svc)
		if err != nil {
			renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Algo deu errado. Tente novamente em instantes.")
			return
		}
		if signedIn && u.TermsVersion != CurrentTermsVersion {
			http.Redirect(w, r, "/termos/aceitar", http.StatusSeeOther)
			return
		}
		render(w, r, u, signedIn)
	}
}

func acceptTermsForm(svc *users.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, signedIn, err := currentUser(r, svc); err != nil || !signedIn {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		renderHTML(w, r, http.StatusOK, acceptTermsPage(""))
	}
}

func acceptTermsSubmit(svc *users.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, signedIn, err := currentUser(r, svc)
		if err != nil || !signedIn {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		if r.FormValue("aceito") != "on" {
			renderHTML(w, r, http.StatusBadRequest, acceptTermsPage("Marque a caixa para continuar."))
			return
		}
		if err := svc.AcceptTerms(r.Context(), u.ID, CurrentTermsVersion); err != nil {
			renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível registrar o aceite. Tente novamente.")
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}
