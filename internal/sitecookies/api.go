// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package sitecookies

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/internal/server"
)

// API is the site cookies management API router.
type API struct {
	chi.Router
}

// newAPI sets up the site cookies API routes.
func newAPI() *API {
	r := server.AuthenticatedRouter()
	api := &API{r}

	r.With(server.WithPermission("api:sitecookies", "read")).Get("/", api.list)
	r.With(server.WithPermission("api:sitecookies", "read")).Get("/cookiecloud", api.getCookieCloud)
	r.With(server.WithPermission("api:sitecookies", "write")).Post("/", api.create)
	r.With(server.WithPermission("api:sitecookies", "write")).Put("/{uid}", api.update)
	r.With(server.WithPermission("api:sitecookies", "write")).Delete("/{uid}", api.delete)
	r.With(server.WithPermission("api:sitecookies", "write")).Post("/cookiecloud", api.saveCookieCloud)
	r.With(server.WithPermission("api:sitecookies", "write")).Post("/cookiecloud/sync", api.syncCookieCloud)

	return api
}

// SetupRoutes mounts the site cookies routes on the server.
func SetupRoutes(s *server.Server) {
	s.AddRoute("/api/sitecookies", newAPI())
	s.AddRoute("/cookies", newViews())
}

func (api *API) list(w http.ResponseWriter, r *http.Request) {
	items, err := All()
	if err != nil {
		server.Err(w, r, err)
		return
	}
	server.Render(w, r, http.StatusOK, items)
}

func (api *API) create(w http.ResponseWriter, r *http.Request) {
	var sc SiteCookie
	if err := json.NewDecoder(r.Body).Decode(&sc); err != nil {
		server.TextMsg(w, r, http.StatusBadRequest, "invalid JSON")
		return
	}
	sc.UID = ""
	if err := validateCookie(&sc); err != nil {
		server.TextMsg(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if err := SaveCookie(&sc); err != nil {
		server.Err(w, r, err)
		return
	}
	server.Render(w, r, http.StatusCreated, sc)
}

func (api *API) update(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "uid")
	sc, found, err := Get(uid)
	if err != nil {
		server.Err(w, r, err)
		return
	}
	if !found {
		server.Status(w, r, http.StatusNotFound)
		return
	}
	var input SiteCookie
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		server.TextMsg(w, r, http.StatusBadRequest, "invalid JSON")
		return
	}
	sc.Domain = input.Domain
	sc.Name = input.Name
	sc.Cookies = input.Cookies
	sc.Headers = input.Headers
	if err := validateCookie(sc); err != nil {
		server.TextMsg(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if err := SaveCookie(sc); err != nil {
		server.Err(w, r, err)
		return
	}
	server.Render(w, r, http.StatusOK, sc)
}

func (api *API) delete(w http.ResponseWriter, r *http.Request) {
	if err := DeleteCookie(chi.URLParam(r, "uid")); err != nil {
		server.Err(w, r, err)
		return
	}
	server.Status(w, r, http.StatusNoContent)
}

func (api *API) getCookieCloud(w http.ResponseWriter, r *http.Request) {
	cfg, err := GetCookieCloudConfig()
	if err != nil {
		server.Err(w, r, err)
		return
	}
	// Never expose the password to the browser.
	cfg.Password = ""
	server.Render(w, r, http.StatusOK, cfg)
}

func (api *API) saveCookieCloud(w http.ResponseWriter, r *http.Request) {
	var cfg CookieCloudConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		server.TextMsg(w, r, http.StatusBadRequest, "invalid JSON")
		return
	}
	// Keep the existing password when the form submits an empty value.
	current, err := GetCookieCloudConfig()
	if err != nil {
		server.Err(w, r, err)
		return
	}
	if cfg.Password == "" {
		cfg.Password = current.Password
	}
	if err := UpdateCookieCloudConfig(cfg); err != nil {
		server.Err(w, r, err)
		return
	}
	server.Render(w, r, http.StatusOK, map[string]bool{"ok": true})
}

func (api *API) syncCookieCloud(w http.ResponseWriter, r *http.Request) {
	count, err := SyncCookieCloud()
	if err != nil {
		slog.Warn("CookieCloud sync failed", slog.Any("err", err))
		server.TextMsg(w, r, http.StatusBadGateway, err.Error())
		return
	}
	server.Render(w, r, http.StatusOK, map[string]int{"domains": count})
}

func validateCookie(sc *SiteCookie) error {
	if normalizeDomain(sc.Domain) == "" {
		return errInvalidDomain
	}
	if sc.Cookies == "" && len(sc.Headers) == 0 {
		return errEmptyCookie
	}
	return nil
}
