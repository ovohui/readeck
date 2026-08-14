// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package oidc

import (
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"slices"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"
	"golang.org/x/oauth2"

	"codeberg.org/readeck/readeck/components"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/internal/sessions"
	"codeberg.org/readeck/readeck/pkg/forms"
)

// SetupRoutes mounts the routes for the auth domain.
func SetupRoutes(srv *server.Server) {
	srv.AddRoute("/login/oidc", newOIDCHandler())
}

type oidcHandler struct {
	chi.Router
}

type callbackForm struct {
	forms.Form
	Code      forms.TextField `json:"code"              validate:"trim"`
	State     forms.TextField `json:"state"             validate:"trim"`
	Error     forms.TextField `json:"error"`
	ErrorDesc forms.TextField `json:"error_description"`
}

func newOIDCHandler() *oidcHandler {
	r := chi.NewRouter()
	r.Use(
		server.Csrf,
		server.WithSession(),
		server.WithCustomErrorComponent(components.AuthError),
	)

	h := &oidcHandler{r}

	h.Post("/", h.oidcRedirect)
	h.Get("/", h.oidcCallback)

	return h
}

func (h *oidcHandler) oidcRedirect(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	cf, err := Providers.get(r.Context(), id)
	if err != nil {
		server.Err(w, r, newError(err, withAttrs(slog.String("provider", id))))
		return
	}

	tracker := &cookie{
		ID:    id,
		State: randText(10),
		Nonce: randText(10),
		Redir: r.FormValue("r"),
	}

	if slices.Contains(cf.Config.CodeChallengeMethodsSupported, "S256") {
		tracker.PKCEVerifier = randText(56)
	}

	if err = tracker.Save(w, r); err != nil {
		server.Err(w, r, newError(err))
		return
	}

	options := []oauth2.AuthCodeOption{oidc.Nonce(tracker.Nonce)}
	if tracker.PKCEVerifier != "" {
		options = append(options, oauth2.S256ChallengeOption(tracker.PKCEVerifier))
	}
	redir := cf.OAuth.AuthCodeURL(tracker.State, options...)

	http.Redirect(w, r, redir, http.StatusFound)
}

func (h *oidcHandler) oidcCallback(w http.ResponseWriter, r *http.Request) {
	f := forms.New[callbackForm](r.Context())
	forms.BindValues(r.URL.Query(), f)

	if f.Error.Value() != "" {
		server.Err(w, r, newError(
			errors.New(f.Error.Value()),
			withStatus(http.StatusUnauthorized),
			withAttrs(
				slog.String("description", f.ErrorDesc.Value()),
			),
		))
		return
	}

	if f.Code.IsEmpty() || f.State.IsEmpty() {
		server.Err(w, r, errNoAuthCodes)
		return
	}

	tracker, err := newCookie(r)
	if err != nil {
		server.Err(w, r, newError(err))
		return
	}

	if subtle.ConstantTimeCompare([]byte(tracker.State), []byte(f.State.Value())) == 0 {
		server.Err(w, r, errState)
		return
	}

	provider, err := Providers.get(r.Context(), tracker.ID)
	if err != nil {
		server.Err(w, r, newError(err, withAttrs(slog.String("provider", tracker.ID))))
		return
	}

	// Here is the underwhelming aspect of OIDC.
	// There are as many ways to do things as there are OIDC servers.
	// Some will give you all the information you need in the ID Token
	// claims, others, maybe the email addresse, or nothing.
	// We first check and validate the claims. If it contains an email and
	// some groups, we can avoid querying the user info endpoint.

	// Exchange token and get user information from ID Token
	token, info, err := provider.Exchange(r.Context(), f.Code.Value(), tracker.Nonce, tracker.PKCEVerifier)
	if err != nil {
		server.Err(w, r, newError(err))
		return
	}

	// it might not be enough information so fetch the user if we can
	if info.Username == "" || info.Email == "" || info.Groups == nil {
		// Get user information
		info, err = provider.GetUser(r.Context(), token)
		if err != nil {
			server.Err(w, r, newError(err))
			return
		}
	}

	// We don't need the oidc cookie anymore
	redir := tracker.Redir
	tracker.Delete(w, r)

	// Provisioning user
	form := info.Form()
	slog.Debug(
		"OIDC user",
		slog.Any("id", form.ID.Value()),
		slog.Any("username", form.Username.Value()),
		slog.Any("email", form.Email.Value()),
		slog.String("group", form.Group.Value()),
	)

	if !form.IsValid() {
		server.Err(w, r, newError(form.errors(), withStatus(http.StatusInternalServerError)))
		return
	}

	user, err := form.Execute()
	if err != nil {
		server.Err(w, r, err)
		return
	}

	// Encode and add the session cookie to the request
	sess, _ := sessions.New(server.SessionHandler(), r)
	sess.Payload.User = user.ID
	sess.Payload.Seed = user.Seed
	sess.Payload.RequiresMFA = user.RequiresMFA()
	sess.Save(w, r)

	if sess.Payload.RequiresMFA {
		server.RedirectToMFA(w, r, redir)
		return
	}

	server.RedirectNoLogin(w, r, redir)
}
