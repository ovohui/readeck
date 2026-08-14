// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package preferences provides client-bound preferences.
// The preferences are stored in and retrieved from an encrypted cookie.
// Most of them are unsigned integers so we can minimize the payload size as
// much as possible.
package preferences

import (
	"hash"
	"io"
	"net/http"
	"path"
	"strconv"
	"time"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/server/urls"
	"codeberg.org/readeck/readeck/pkg/http/securecookie"
)

var cookieHandler *securecookie.Handler

var ttl = int((time.Hour * 24 * 400).Seconds()) // 400 days is the maximum cookie lifetime on Chrome.

// InitHandler initializes the preferences' [securecookie.Handler].
func InitHandler() {
	cookieHandler = securecookie.NewHandler(
		securecookie.Key(configs.Keys.PreferencesKey()),
		securecookie.WithPath(path.Join(urls.Prefix())),
		securecookie.WithMaxAge(ttl),
		securecookie.WithName("rd_prefs"),
	)
}

// Data contains all the client-bound preferences.
type Data struct {
	LastUpdate   int64            `json:"lu"`
	BookmarkList BookmarkListView `json:"bl"`
	Bookmark     BookmarkView     `json:"b"`
}

// BookmarkView contains preferences for the bookmark view.
type BookmarkView struct {
	ReaderWidth readerWidth `json:"w"`
	FontFamily  fontFamily  `json:"f"`
	FontSize    fontSize    `json:"fs"`
	LineHeight  lineHeight  `json:"lh"`
	Justified   justified   `json:"j"`
	Hyphenated  hyphenated  `json:"h"`
	HideSidebar bool        `json:"hs"`
}

// BookmarkListView contains the preferences for the bookmark list view.
type BookmarkListView struct {
	Layout listLayout `json:"l"`
}

// New returns a new instance of [Data] with default values.
func New() *Data {
	return &Data{
		BookmarkList: BookmarkListView{
			Layout: ListGrid,
		},
		Bookmark: BookmarkView{
			ReaderWidth: WidthRegular,
			FontFamily:  FontLora,
			FontSize:    FontSize3,
			LineHeight:  LineHeight3,
			Justified:   false,
			Hyphenated:  false,
			HideSidebar: false,
		},
	}
}

// Load loads the preferences from the cookie.
func Load(r *http.Request) *Data {
	p := New()
	_ = cookieHandler.Load(r, p)
	return p
}

// Save writes the preferences cookie.
func (p *Data) Save(w http.ResponseWriter, r *http.Request) error {
	p.LastUpdate = time.Now().UTC().UnixMilli()
	return cookieHandler.Save(w, r, p)
}

// GetLastModified implements [server.LastModer].
func (p *Data) GetLastModified() []time.Time {
	return []time.Time{time.UnixMilli(p.LastUpdate).UTC()}
}

// UpdateEtag implements [server.Etagger].
func (p *Data) UpdateEtag(h hash.Hash) {
	io.WriteString(h, strconv.Itoa(int(p.LastUpdate)))
}
