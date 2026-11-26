// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package importer

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/go-shiori/dom"
	"golang.org/x/net/html"

	"codeberg.org/readeck/readeck/internal/bookmarks/tasks"
	"codeberg.org/readeck/readeck/internal/db/types"
	"codeberg.org/readeck/readeck/pkg/forms"
)

// The HTML page adapter imports a page saved with the browser ("Save page as…")
// as a single bookmark. Unlike the browser adapter, which reads a Netscape
// bookmarks export, this one keeps the page itself: the HTML is handed to the
// extractor as a cached resource, so no second request is made and a page
// behind a login can be imported after the fact.

// maxHTMLPageSize is the largest saved page we accept. A forum page with
// inline base64 pictures can easily reach tens of megabytes.
const maxHTMLPageSize = 64 << 20

type htmlPageAdapter struct {
	Item *htmlPageItem `json:"item"`
}

type htmlPageForm struct {
	FileImportForm
	URL forms.TextField `json:"url" validate:"trim max_len:2048"`
}

type htmlPageItem struct {
	Link        string        `json:"url"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Authors     types.Strings `json:"authors"`
	Lang        string        `json:"lang"`
	Data        []byte        `json:"data"`
}

func (i *htmlPageItem) URL() string {
	return i.Link
}

// EnableReadability is false for saved pages: a forum thread or a page saved
// behind a login often has a layout readability cannot score, and picking the
// wrong container loses the very content the user saved. The full body is
// cleaned by the normal DOM processors instead.
func (i *htmlPageItem) EnableReadability() bool {
	return false
}

func (i *htmlPageItem) Meta() (*BookmarkMeta, error) {
	return &BookmarkMeta{
		Title:       i.Title,
		Description: i.Description,
		Authors:     i.Authors,
		Lang:        i.Lang,
	}, nil
}

// Resources hands the saved HTML to the extractor as the page body. The URL
// must be exactly the bookmark URL so the HTTP cache returns it instead of a
// network request.
func (i *htmlPageItem) Resources() []tasks.MultipartResource {
	return []tasks.MultipartResource{{
		URL: i.Link,
		Header: http.Header{
			"Content-Type": {"text/html; charset=utf-8"},
		},
		Data: i.Data,
	}}
}

func (a *htmlPageAdapter) Name(ctx context.Context) string {
	return forms.GetTranslator(ctx).Gettext("Saved HTML Page")
}

func (a *htmlPageAdapter) Form(ctx context.Context) importBinder {
	return forms.New[htmlPageForm](ctx)
}

func (a *htmlPageAdapter) Params(form forms.FormBinder) ([]byte, error) {
	if !form.IsValid() {
		return nil, nil
	}
	f := form.(*htmlPageForm)

	rc, err := f.Data.Value().Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close() //nolint:errcheck

	data, err := io.ReadAll(io.LimitReader(rc, maxHTMLPageSize+1))
	if err != nil {
		f.Data.AddErrors(forms.Gettext("Unable to read HTML content"), err)
		return nil, nil
	}
	if len(data) > maxHTMLPageSize {
		f.Data.AddErrors(forms.Gettext("This file is too large. The limit is 64 MB."))
		return nil, nil
	}

	root, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		f.Data.AddErrors(forms.Gettext("Unable to read HTML content"), err)
		return nil, nil
	}

	// Read the metadata before trimming: the visible heading is often in the
	// page header, which trimSavedPage removes.
	title := htmlPageTitle(root, f.Data.Value().Filename())
	description := htmlPageDescription(root)
	authors := htmlPageAuthors(root)
	lang := htmlPageLang(root)

	explicitURL := strings.TrimSpace(f.URL.Value())
	link, base := htmlPageURL(root, explicitURL, f.Data.Value().Filename())
	trimmed := trimSavedPage(root)
	if base != nil && explicitURL == "" {
		absolutize(root, base)
	}
	if trimmed || (base != nil && explicitURL == "") {
		if out, err := renderHTML(root); err == nil {
			data = out
		}
	}

	a.Item = &htmlPageItem{
		Link:        link,
		Title:       title,
		Description: description,
		Authors:     authors,
		Lang:        lang,
		Data:        data,
	}

	return json.Marshal(a)
}

func (a *htmlPageAdapter) LoadData(data []byte) error {
	return json.Unmarshal(data, a)
}

func (a *htmlPageAdapter) Next() (BookmarkImporter, error) {
	if a.Item == nil {
		return nil, io.EOF
	}
	item := a.Item
	a.Item = nil
	return item, nil
}

// htmlPageURL finds the page URL, in this order: the URL typed in the form,
// an absolute canonical/og:url, an absolute base plus the canonical path, and
// finally a synthetic URL so the bookmark can still be created offline.
func htmlPageURL(root *html.Node, explicit, filename string) (string, *url.URL) {
	if explicit != "" {
		if u, err := url.Parse(explicit); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
			return u.String(), u
		}
	}

	var canonical, baseHref string
	if n := dom.QuerySelector(root, `link[rel="canonical"][href]`); n != nil {
		canonical = strings.TrimSpace(dom.GetAttribute(n, "href"))
	}
	if n := dom.QuerySelector(root, `meta[property="og:url"][content]`); n != nil && canonical == "" {
		canonical = strings.TrimSpace(dom.GetAttribute(n, "content"))
	}
	if n := dom.QuerySelector(root, `base[href]`); n != nil {
		baseHref = strings.TrimSpace(dom.GetAttribute(n, "href"))
	}

	// An absolute canonical URL is the best source.
	if u, err := url.Parse(canonical); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
		return u.String(), u
	}

	// An absolute <base> resolves a relative canonical path.
	if u, err := url.Parse(baseHref); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
		if canonical != "" {
			if c, err := u.Parse(canonical); err == nil {
				return c.String(), u
			}
		}
		return u.String(), u
	}

	// Nothing absolute: keep the page offline under a placeholder host.
	name := filepath.Base(filename)
	if name == "" || name == "." {
		name = "saved-page.html"
	}
	link := "https://import.local/" + url.PathEscape(name)
	return link, nil
}

func htmlPageTitle(root *html.Node, filename string) string {
	// The visible heading is usually the work's own title; <title> carries
	// the site name and often "Powered by Discuz!".
	for _, sel := range []string{"#thread_subject", ".thread_subject", "h1"} {
		if n := dom.QuerySelector(root, sel); n != nil {
			if title := strings.TrimSpace(dom.TextContent(n)); title != "" {
				return title
			}
		}
	}
	if n := dom.QuerySelector(root, "title"); n != nil {
		if title := strings.TrimSpace(dom.TextContent(n)); title != "" {
			return title
		}
	}
	if n := dom.QuerySelector(root, `meta[property="og:title"][content]`); n != nil {
		if title := strings.TrimSpace(dom.GetAttribute(n, "content")); title != "" {
			return title
		}
	}
	return filepath.Base(filename)
}

func htmlPageDescription(root *html.Node) string {
	for _, sel := range []string{
		`meta[name="description"][content]`,
		`meta[property="og:description"][content]`,
	} {
		if n := dom.QuerySelector(root, sel); n != nil {
			if desc := strings.TrimSpace(dom.GetAttribute(n, "content")); desc != "" {
				return desc
			}
		}
	}
	return ""
}

func htmlPageAuthors(root *html.Node) types.Strings {
	for _, sel := range []string{`meta[name="author"][content]`, `meta[property="article:author"][content]`} {
		if n := dom.QuerySelector(root, sel); n != nil {
			if author := strings.TrimSpace(dom.GetAttribute(n, "content")); author != "" {
				return types.Strings{author}
			}
		}
	}
	return nil
}

func htmlPageLang(root *html.Node) string {
	if n := dom.QuerySelector(root, "html[lang]"); n != nil {
		lang := strings.TrimSpace(dom.GetAttribute(n, "lang"))
		if len(lang) >= 2 {
			return lang[:2]
		}
	}
	return ""
}

// trimSavedPage keeps the main content container of a saved page and drops
// the site header, navigation and footer. The body is replaced with the
// container's HTML; head metadata stays so the extractor still sees the title
// and description.
func trimSavedPage(root *html.Node) bool {
	body := dom.QuerySelector(root, "body")
	if body == nil {
		return false
	}

	for _, sel := range []string{
		"#postlist",   // Discuz threads
		"#js_content", // WeChat
		"article",
		"main",
		"[role=main]",
		"#content",
	} {
		n := dom.QuerySelector(root, sel)
		if n == nil {
			continue
		}
		// Detach the container's children and put them directly in the body.
		// AppendChild moves a node, so the old body children are removed first.
		for c := body.FirstChild; c != nil; {
			next := c.NextSibling
			body.RemoveChild(c)
			c = next
		}
		for c := n.FirstChild; c != nil; {
			next := c.NextSibling
			dom.AppendChild(body, c)
			c = next
		}
		return true
	}

	return false
}

// absolutize rewrites picture URLs that are relative to the page. Without it
// a saved page's /data/attachment/... pictures would point at Readeck instead
// of the original site.
func absolutize(root *html.Node, base *url.URL) {
	for _, n := range dom.QuerySelectorAll(root, "img[src],source[src],video[poster]") {
		attr := "src"
		if dom.TagName(n) == "video" {
			attr = "poster"
		}
		raw := strings.TrimSpace(dom.GetAttribute(n, attr))
		if raw == "" || strings.HasPrefix(raw, "data:") || strings.HasPrefix(raw, "blob:") {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.IsAbs() {
			continue
		}
		if abs, err := base.Parse(raw); err == nil {
			dom.SetAttribute(n, attr, abs.String())
		}
	}
}

func renderHTML(root *html.Node) ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := html.Render(buf, root); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
