// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package dashboard provides routes for Readeck's dashboard.
package dashboard

import (
	"hash"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/bookmarks/dataset"
	"codeberg.org/readeck/readeck/internal/db"
	"codeberg.org/readeck/readeck/internal/db/exp"
	"codeberg.org/readeck/readeck/internal/db/types"
	"codeberg.org/readeck/readeck/internal/server"
)

type dashboardViews struct {
	chi.Router
}

// SetupRoutes mounts the routes for the auth domain.
func SetupRoutes(s *server.Server) {
	s.AddRoute("/", newDashboardViews())
}

func newDashboardViews() *dashboardViews {
	r := server.AuthenticatedRouter(server.WithRedirectLogin)
	v := &dashboardViews{r}

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		server.Redirect(w, r, "/dashboard")
	})
	r.With(server.WithPermission("profile", "read")).Get("/dashboard", v.main)

	return v
}

type dashboardItems struct {
	Total             int64
	LastUpdate        types.DateTime
	NewBookmarks      *dataset.BookmarkIterator
	FavoriteBookmarks *dataset.BookmarkIterator
	ArchivedBookmarks *dataset.BookmarkIterator
	Articles          *dataset.BookmarkIterator
	Pictures          *dataset.BookmarkIterator
	Videos            *dataset.BookmarkIterator
}

func (di *dashboardItems) GetLastModified() []time.Time {
	return []time.Time{time.Time(di.LastUpdate)}
}

func (di *dashboardItems) UpdateEtag(h hash.Hash) {
	io.WriteString(h, strconv.FormatInt(time.Time(di.LastUpdate).UnixNano(), 10))
}

func (v *dashboardViews) main(w http.ResponseWriter, r *http.Request) {
	ds := bookmarks.Bookmarks.Query().
		Where(
			goqu.C("user_id").Eq(auth.GetUser(r.Context()).ID),
		).
		Order(goqu.C("created").Desc()).
		Limit(5)

	var err error
	items := &dashboardItems{}

	// get last modified
	du := db.Q().Select(goqu.MAX("time")).From(
		ds.
			ClearOrder().ClearLimit().
			Select(goqu.MAX(exp.DateTime(goqu.C("updated"))).As("time")).
			Union(
				db.Q().From(goqu.T(db.TableBookmarkRemoved)).
					Select(goqu.MAX(exp.DateTime(goqu.C("deleted"))).As("time")).
					Where(
						goqu.C("user_id").Eq(auth.GetUser(r.Context()).ID),
					),
			),
	)

	if _, err = du.ScanVal(&items.LastUpdate); err != nil {
		server.Err(w, r, err)
		return
	}
	server.WriteLastModified(w, r, items)
	server.WriteEtag(w, r, items)
	if server.HandleCaching(w, r) {
		return
	}

	if items.Total, err = ds.ClearOrder().Count(); err != nil {
		server.Err(w, r, err)
		return
	}
	items.NewBookmarks = dataset.NewBookmarkIterator(
		r.Context(),
		ds.Where(goqu.C("is_archived").Is(false)),
	)
	items.FavoriteBookmarks = dataset.NewBookmarkIterator(
		r.Context(),
		ds.Where(goqu.C("is_marked").Is(true)),
	)
	items.ArchivedBookmarks = dataset.NewBookmarkIterator(
		r.Context(),
		ds.Where(goqu.C("is_archived").Is(true)),
	)
	items.Articles = dataset.NewBookmarkIterator(
		r.Context(),
		ds.Where(goqu.C("type").Eq("article")),
	)
	items.Pictures = dataset.NewBookmarkIterator(
		r.Context(),
		ds.Where(goqu.C("type").Eq("photo")),
	)
	items.Videos = dataset.NewBookmarkIterator(
		r.Context(),
		ds.Where(goqu.C("type").Eq("video")),
	)

	server.RenderComponent(w, r, http.StatusOK, Views{}.dashboard(items))
}
