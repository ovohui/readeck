// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package sitecookies

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/internal/server"
)

// Views is the site cookies settings page renderer.
type Views struct{}

// newViews sets up the settings page routes.
func newViews() chi.Router {
	r := server.AuthenticatedRouter(server.WithRedirectLogin)
	v := Views{}
	r.With(server.WithPermission("web:sitecookies", "read")).Get("/", func(w http.ResponseWriter, r *http.Request) {
		server.RenderComponent(w, r, http.StatusOK, v.page())
	})
	return r
}
