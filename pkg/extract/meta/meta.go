// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package meta provides extract processors to retrieve several meta information
// from a page (meta tags, favicon, pictures...).
package meta

import (
	"bytes"
	"iter"
	"log/slog"
	"maps"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/go-shiori/dom"
	"github.com/gobwas/glob"
	"github.com/itlightning/dateparse"

	"codeberg.org/readeck/readeck/pkg/extract"
	"codeberg.org/readeck/readeck/pkg/extract/meta/tinymeta"
	"codeberg.org/readeck/readeck/pkg/extract/microdata"
)

var rxOpenGraphType = regexp.MustCompile(`^([^:]*:)?(.+?)(\..*|$)`)

// ExtractMeta is a processor that extracts metadata from the
// document and set the Drop values accordingly.
func ExtractMeta(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom || m.Dom == nil || m.Position() > 0 {
		return next
	}

	m.Log().Debug("loading metadata")

	// Set raw meta
	d := m.Extractor.Drop()
	maps.Copy(d.Meta, ParseMeta(m.Dom))
	maps.Copy(d.Properties, ParseProps(m.Dom))

	// Extract JSON-LD and microdata from page
	var md *mdProp
	if x, err := microdata.ParseNode(m.Dom, m.Extractor.URL.String()); err == nil {
		md = &mdProp{x}
	} else {
		m.Log().Warn("parse microdata", slog.Any("err", err))
	}

	if md != nil {
		// Add the raw JSON-LD + microdata to properties
		d.Properties["json-ld"] = md.Raw()

		// fetch relevant information
		if headline, ok := md.getProp("Article.name", "*.headline", "{Movie,VideObject}.name").(string); ok {
			d.Meta.Add("ld.headline", headline)
		}
		if description, ok := md.getProp("{Article,NewsArticle,WebPage}.description", "*.description").(string); ok {
			d.Meta.Add("ld.description", description)
		}
		if image, ok := md.getProp("*.{image,image.url,thumbnailUrl}").(string); ok {
			d.Meta.Add("ld.image", image)
		}

		if lang, ok := md.getProp("*.{inLanguage,inLanguage.alternateName}").(string); ok {
			d.Meta.Add("ld.lang", lang[0:2])
		}
		if published, ok := md.getProp("*.datePublished").(string); ok {
			d.Meta.Add("ld.published", published)
		}
		if publisher, ok := md.getProp("{*.publisher,*.publisher.name,{Blog,WebSite}.name}").(string); ok {
			d.Meta.Add("ld.publisher", publisher)
		}

		// note: this will stop at the first matches
		// (if we have 3 entries in *.author.name, it won't check Person.name)
		for x := range md.iterProps("*.author.name", "Person.name") {
			if x, ok := x.(string); ok {
				d.Meta.Add("ld.author", x)
			}
		}
	}

	d.Title = d.Meta.LookupGet(
		"ld.headline",
		"graph.title",
		"twitter.title",
		"html.title",
	)

	d.Description = d.Meta.LookupGet(
		"ld.description",
		"graph.description",
		"twitter.description",
		"html.description",
	)

	// Keep a short description (60 words)
	if parts := strings.Fields(d.Description); len(parts) > 60 {
		d.Description = strings.Join(parts[:60], " ") + "..."
	}

	d.AddAuthors(d.Meta.Lookup(
		"ld.author",
		"graph.author",
		"dc.creator",
		"html.author",
		"html.byl",
		"fediverse.creator",
	)...)

	for _, site := range d.Meta.Lookup(
		"ld.publisher",
		"graph.site_name",
	) {
		if site != "" {
			d.Site = site
			break
		}
	}

	if lang := d.Meta.LookupGet(
		"ld.lang",
		"html.lang",
		"html.language",
	); len(lang) >= 2 {
		d.Lang = lang[0:2]
	}

	d.TextDirection = d.Meta.LookupGet("html.dir")

	m.Log().Debug("metadata loaded", slog.Int("count", len(d.Meta)))
	return next
}

// ProcessRefreshRedirect looks for a Refresh header or its metadata counterpart.
// When found with a "URL=" parameter, this resets the extraction process with
// the new URL.
func ProcessRefreshRedirect(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom || m.Position() > 0 {
		return next
	}

	refresh := m.Extractor.Drop().Header.Get("refresh")
	var refreshURL string

	if refresh == "" {
		for k, v := range tinymeta.Scan(bytes.NewReader(m.Extractor.Drop().Body)) {
			if strings.ToLower(k) == "refresh" {
				refresh = v
				break
			}
		}
	}

	if refresh != "" {
		_, s, _ := strings.Cut(refresh, ";")
		s = strings.TrimSpace(s)
		if len(s) > 4 && strings.ToLower(s[0:4]) == "url=" {
			refreshURL = strings.TrimSpace(s[4:])
		}
	}

	if refreshURL == "" {
		return next
	}

	u, err := m.Extractor.Drop().URL.Parse(refreshURL)
	if err != nil {
		return next
	}
	u.Fragment = ""

	// Same URL, carry on
	origin := m.Extractor.Drop().URL.String()
	if u.String() == origin {
		return next
	}

	// Already seen
	if m.Extractor.Visited.IsPresent(u) {
		m.Log().Debug("page already visited", slog.String("url", u.String()))
		return next
	}

	// Proceed with reset
	m.Log().Debug("found redirection through refresh", slog.String("url", refreshURL))
	if err = m.Extractor.ReplaceDrop(u); err != nil {
		m.Log().Error("cannot replace page", slog.Any("err", err))
		return nil
	}

	// Set referer header
	if t, ok := m.Extractor.Client().Transport.(interface{ SetHeader(func(http.Header)) }); ok {
		t.SetHeader(func(h http.Header) {
			h.Set("Referer", origin)
		})
	}

	m.ResetPosition()
	return nil
}

// SetDropProperties will set some Drop properties bases on the retrieved
// metadata. It must be run after ExtractMeta and ExtractOembed.
func SetDropProperties(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom || m.Position() > 0 {
		return next
	}

	d := m.Extractor.Drop()

	// Set publication date
	for _, htmlDate := range d.Meta.Lookup("ld.published", "graph.published_time", "html.date") {
		if dt, err := dateparse.ParseLocal(htmlDate); err == nil && !dt.IsZero() {
			d.Date = dt.UTC()
			break
		}
	}

	// We might have a picture here
	if d.Meta.LookupGet("dc.type") == "image" {
		d.DocumentType = "photo"
	}

	// Set document type from opengraph value
	if d.DocumentType == "" {
		ogt := d.Meta.LookupGet("graph.type")
		if ogt != "" {
			d.DocumentType = rxOpenGraphType.ReplaceAllString(ogt, "$2")
		}
	}

	// If no authors, try to get them from oembed
	if len(d.Authors) == 0 {
		d.AddAuthors(d.Meta.Lookup("oembed.author_name")...)
	}

	// Same for website name
	if d.Site == "" || d.Site == d.URL.Hostname() {
		if site := d.Meta.LookupGet("oembed.provider_name"); site != "" {
			d.Site = site
		}
	}

	// If we have a picture type, we force the type and set a new meta
	// for the picture url
	otype := d.Meta.LookupGet("oembed.type")
	if d.DocumentType == "photo" || otype == "photo" {
		d.DocumentType = "photo"

		if otype == "photo" {
			d.Meta.Add("x.picture_url", d.Meta.LookupGet("oembed.url"))
		}
	}

	if otype == "video" {
		d.DocumentType = otype
	}

	// Document type is only a predefined set and nothing more
	switch d.DocumentType {
	case "article", "photo", "video":
		// Valid values
	default:
		d.DocumentType = "article"
	}

	m.Log().Info("document type", slog.String("type", d.DocumentType))
	return next
}

// ParseMeta parses page metadata.
func ParseMeta(doc *html.Node) extract.DropMeta {
	res := extract.DropMeta{}

	for node := range eachElement(doc) {
		switch node.DataAtom {
		case atom.Html, atom.Body:
			// Extract "dir" and "lang" attributes from <html> and <body>
			for _, attr := range node.Attr {
				switch attr.Key {
				case "dir":
					dir := strings.ToLower(strings.TrimSpace(attr.Val))
					switch dir {
					case "rtl", "ltr":
						res.Set("html.dir", dir)
					}
				case "lang":
					res.Set("html.lang", strings.TrimSpace(attr.Val))
				}
			}
		case atom.Title:
			// Extract the document <title>
			res.Set("html.title", strings.TrimSpace(dom.TextContent(node)))
		case atom.Link:
			// Extract "rel" and "href" attributes from <link> other than stylesheets and favicon
			var href string
			var rel string
			for _, attr := range node.Attr {
				switch attr.Key {
				case "href":
					href = strings.TrimSpace(attr.Val)
				case "rel":
					rel = strings.TrimSpace(attr.Val)
				}
			}
			if href != "" && rel != "" && !strings.Contains(rel, "stylesheet") && !strings.Contains(strings.ToLower(rel), "icon") {
				res.Add("link."+rel, href)
			}
		case atom.Meta:
			// Extract "content" value from <meta> tags according to:
			// - old HTML document conventions,
			// - Open Graph protocol,
			// - Twitter cards,
			// - Mastodon "fediverse:creator" convention,
			// - Dublin Core
			var content string
			var name string
			var nameOrProperty string
			for _, attr := range node.Attr {
				switch attr.Key {
				case "content":
					content = strings.TrimSpace(attr.Val)
				case "name":
					name = strings.TrimSpace(attr.Val)
					fallthrough
				case "property":
					nameOrProperty = strings.TrimSpace(attr.Val)
				}
			}
			if content == "" {
				continue
			}
			switch name {
			case "author", "byl", "copyright", "date", "description", "keywords", "language", "subtitle":
				// e.g. "html.author", "html.description"
				res.Add("html."+name, content)
			default:
				switch {
				case strings.HasPrefix(name, "dc.") || strings.HasPrefix(name, "DC."):
					// e.g. "dc.creator"
					res.Add("dc."+name[3:], content)
				case strings.HasPrefix(name, "twitter:"):
					// e.g. "twitter:title" → "twitter.title"
					res.Add(strings.Replace(name, ":", ".", 1), content)
				case strings.HasPrefix(name, "fediverse:"):
					// e.g. "fediverse:creator" → "fediverse.creator"
					res.Add(strings.Replace(name, ":", ".", 1), content)
				case strings.HasPrefix(nameOrProperty, "og:"):
					// e.g. "og:image" → "graph.image"
					res.Add("graph."+nameOrProperty[3:], content)
				case strings.HasPrefix(nameOrProperty, "article:"):
					// e.g. "article:published_time" → "graph.published_time"
					res.Add("graph."+nameOrProperty[8:], content)
				}
			}
		}
	}

	return res
}

// eachElement iterates over all elements contained in doc, with the exception of the descendants of
// SVG and MATH elements which are skipped.
func eachElement(doc *html.Node) iter.Seq[*html.Node] {
	return func(yield func(*html.Node) bool) {
		var traverse func(*html.Node) bool
		traverse = func(parent *html.Node) bool {
			for node := parent.FirstChild; node != nil; node = node.NextSibling {
				if node.Type != html.ElementNode {
					continue
				}
				if !yield(node) {
					return false
				}
				switch node.DataAtom {
				case atom.Svg, atom.Math:
					continue
				}
				if !traverse(node) {
					return false
				}
			}
			return true
		}
		traverse(doc)
	}
}

var mdGlobCache sync.Map

type mdProp struct {
	*microdata.Microdata
}

// iterProps returns an iterator over [microdata.Node] of type [microdata.Property].
// It uses a list of glob matchers. The loop stops after the first matcher that yields
// results.
// That way, if you need to yield all the result for Article.name and WebSite.name, you
// call iterProps("{Article,WebSite}.name").
// If you want to stop on the first match, you then call iterProps("Article.name", "WebSite.name").
func (md *mdProp) iterProps(matchers ...string) iter.Seq[any] {
	patterns := []glob.Glob{}
	for _, x := range matchers {
		if g, ok := mdGlobCache.Load(x); ok {
			patterns = append(patterns, g.(glob.Glob))
		} else {
			g := glob.MustCompile(x, '.')
			patterns = append(patterns, g)
			mdGlobCache.Store(x, g)
		}
	}

	return func(yield func(any) bool) {
		for _, g := range patterns {
			match := false
			for node := range md.All(func(n *microdata.Node) bool { return n.Type == microdata.Property }) {
				if g.Match(node.Path) {
					if !yield(node.Data) {
						return
					}
					match = true
				}
			}
			if match {
				return
			}
		}
	}
}

// getProp returns the first property found using [mdProp.iterProps].
func (md *mdProp) getProp(names ...string) any {
	for res := range md.iterProps(names...) {
		return res
	}
	return nil
}
