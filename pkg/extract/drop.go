// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package extract

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/go-shiori/dom"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/net/html/charset"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"

	"codeberg.org/readeck/readeck/pkg/bleach"
)

var (
	rxAuthor      = regexp.MustCompile(`^(?i)by(\s*:)?\s+`)
	rxSpaces      = regexp.MustCompile(`\s+`)
	rxTitleSpaces = regexp.MustCompile(`[_-]+`)
	rxSrcsetURL   = regexp.MustCompile(`(?i)(\S+)(\s+[\d.]+[xw])?(\s*(?:,|$))`)

	mediaTypes = []string{"photo", "video", "audio", "music"}
)

// SiteCookieLookup reports whether a login cookie is stored for a host. It is
// set at startup by the site cookies package, which cannot be imported here
// without a cycle. A refusal reads very differently depending on the answer:
// "you never saved a cookie" and "the cookie you saved is not accepted" have
// nothing in common as a next step.
var SiteCookieLookup func(host string) bool

// WithoutSiteCookie returns a context that suppresses the saved site cookies
// for one request. Set at startup by the site cookies package, for the same
// reason as SiteCookieLookup.
var WithoutSiteCookie func(ctx context.Context) context.Context

// Drop is the result of a content extraction of one resource.
type Drop struct {
	URL          *url.URL
	Domain       string
	ContentType  string
	Charset      string
	DocumentType string

	Title         string
	Description   string
	Authors       []string
	Site          string
	Lang          string
	TextDirection string
	Date          time.Time

	Header     http.Header
	Meta       DropMeta
	Properties DropProperties
	Body       []byte `json:"-"`

	Pictures map[string]*Picture
}

// NewDrop returns a Drop instance.
func NewDrop(src *url.URL) *Drop {
	d := &Drop{
		Meta:       DropMeta{},
		Properties: DropProperties{},
		Authors:    []string{},
		Body:       []byte{},
		Pictures:   map[string]*Picture{},
	}
	d.SetURL(src)
	return d
}

// SetURL sets the Drop's URL and Domain properties in their unicode versions.
func (d *Drop) SetURL(src *url.URL) {
	if src == nil {
		d.URL = nil
		d.Domain = ""
		d.Site = ""
		return
	}

	uri := new(url.URL)
	*uri = *src

	// Remove port when it's not needed
	// Note: only numeric ports are valid in [url.URL].
	port := uri.Port()
	if uri.Scheme == "http" && port == "80" || uri.Scheme == "https" && port == "443" {
		port = ""
		// we want to keep the brackets on ipv6 here
		uri.Host = uri.Host[:strings.LastIndexByte(uri.Host, ':')]
	}

	hostname := uri.Hostname()

	if ip, err := netip.ParseAddr(hostname); err == nil {
		// Hostname is an IP address. Shorten the address and use it as the domain.
		s := ip.String()
		if ip.Is6() {
			uri.Host = "[" + s + "]"
		} else {
			uri.Host = s
		}
		if port != "" {
			uri.Host += ":" + port
		}

		d.Domain = s
	} else {
		// Always encode the URL to unicode
		if host, err := idna.ToUnicode(uri.Host); err == nil {
			uri.Host = host
		}
		d.Domain, _ = publicsuffix.EffectiveTLDPlusOne(uri.Hostname())
	}

	if d.Domain == "" {
		d.Domain = hostname
	}

	d.URL = uri
}

// jsChallengeMarkers are fragments that only appear in the interstitial page a
// site serves to a client it takes for a bot. They are specific enough that a
// real article body never contains them.
var jsChallengeMarkers = []string{
	"zh-zse-ck",           // Zhihu
	"acw_sc__v2",          // Alibaba Cloud WAF
	"waf_bd8ce2ce37",      // Xueqiu
	"__jsl_clearance",     // generic JavaScript challenge CDN
	"sina visitor system", // Weibo
}

// detectJSChallenge reads the head of a refusal body and returns the marker of
// the JavaScript anti-bot challenge it found. The body is consumed: the caller
// only reaches this point when it is about to fail anyway.
func detectJSChallenge(body io.Reader) string {
	if body == nil {
		return ""
	}
	head, _ := io.ReadAll(io.LimitReader(body, 8192))
	lower := strings.ToLower(string(head))
	for _, marker := range jsChallengeMarkers {
		if strings.Contains(lower, marker) {
			return marker
		}
	}
	return ""
}

// Load loads the remote URL and retrieve data.
//
// A share link from a mobile app often lands on an intermediate page that does
// not redirect with an HTTP status but with a <meta refresh> or a
// `location.replace(...)` call. Without following those, the extractor archives
// the stub and the real page is never reached. They are followed (at most
// [maxRedirectHops] times) only when the document really is a stub.
func (d *Drop) Load(client *http.Client) error {
	if d.URL == nil {
		return errors.New("No document URL")
	}

	if len(d.Body) > 0 {
		// If we have a body already, we don't load anything and
		// just go with it.
		d.Site = d.URL.Hostname()
		d.ContentType = "text/html"
		d.Charset = "utf-8"
		return nil
	}

	if client == nil {
		client = http.DefaultClient
		defer client.CloseIdleConnections()
	}

	for hop := 0; ; hop++ {
		if err := d.load(client); err != nil {
			return err
		}
		if hop >= maxRedirectHops {
			return nil
		}
		next := d.shareRedirect()
		if next == nil {
			return nil
		}
		slog.Debug(
			"following client side redirect",
			slog.String("from", d.URL.String()),
			slog.String("to", next.String()),
		)
		d.Body = nil
		d.Domain = "" // let SetURL recompute it for the new host
		d.SetURL(next)
	}
}

// fetchPage performs the page request. When noSaved is true the saved site
// cookies are left out, which is how the caller checks whether a stale cookie
// is what the site is objecting to.
func (d *Drop) fetchPage(client *http.Client, noSaved bool) (*http.Response, error) {
	ctx := WithRequestType(context.Background(), PageRequest)
	if noSaved && WithoutSiteCookie != nil {
		ctx = WithoutSiteCookie(ctx)
	}
	return Fetch(ctx, client, d.URL.String())
}

// load performs one request and stores the response.
func (d *Drop) load(client *http.Client) error {
	var err error
	var rsp *http.Response

	if rsp, err = d.fetchPage(client, false); err != nil {
		return err
	}

	// A refusal on a site that has a saved cookie may be the cookie's fault.
	// Anti-bot cookies are dated (Zhihu's __zse_ck lasts about a week) and the
	// site answers a stale one with a challenge it would not have sent to an
	// anonymous visitor. Try once without the saved cookies before blaming
	// them; when that page loads, the saved set is the thing to replace.
	if (rsp.StatusCode == http.StatusUnauthorized || rsp.StatusCode == http.StatusForbidden) &&
		WithoutSiteCookie != nil && SiteCookieLookup != nil && SiteCookieLookup(d.URL.Hostname()) {
		retry, rerr := d.fetchPage(client, true)
		switch {
		case rerr != nil:
			slog.Debug("retry without saved site cookies failed", slog.Any("err", rerr))
		case retry.StatusCode/100 == 2:
			slog.Debug(
				"the page loaded without the saved site cookies",
				slog.String("host", d.URL.Hostname()),
				slog.Int("status", rsp.StatusCode),
			)
			rsp.Body.Close() //nolint:errcheck
			rsp = retry
		default:
			retry.Body.Close() //nolint:errcheck
		}
	}

	defer rsp.Body.Close() //nolint:errcheck

	// Save headers
	d.Header = rsp.Header

	// Set final URL in case it was redirected
	d.SetURL(rsp.Request.URL)

	// Set mime type
	d.ContentType, _, _ = mime.ParseMediaType(rsp.Header.Get("content-type"))

	// A few sites send no Content-Type at all (cls.cn answers a news page with
	// the header missing). Without a type the document is neither HTML nor
	// media, and loading fails on a perfectly good page. Sniff the first bytes
	// and put them back untouched.
	if d.ContentType == "" && rsp.Body != nil {
		br := bufio.NewReaderSize(rsp.Body, 4096)
		head, _ := br.Peek(1024)
		if ct := http.DetectContentType(head); strings.HasPrefix(ct, "text/html") {
			d.ContentType = "text/html"
		}
		rsp.Body = bodyReader{Reader: br, Closer: rsp.Body}
	}

	// Set site
	d.Site = d.URL.Hostname()

	if rsp.StatusCode/100 != 2 {
		// A 401 or 403 almost always means the page wants a session: Chinese
		// Q&A and forum sites answer anonymous requests with a login wall
		// rather than a redirect. Saying so saves a round of guessing.
		if rsp.StatusCode == http.StatusUnauthorized || rsp.StatusCode == http.StatusForbidden {
			host := d.URL.Hostname()

			// Some sites answer with an anti-bot challenge instead of a login
			// wall. That case must be reported apart: the site hands the value
			// out through JavaScript and only accepts it back as a cookie, so
			// "export a fresher cookie" is the fix - but the user has to know
			// which cookie to keep, or they will export the same broken set
			// again.
			if marker := detectJSChallenge(rsp.Body); marker != "" {
				return fmt.Errorf(
					"Invalid status code (%d): %s answered with a JavaScript anti-bot"+
						" challenge (%s) instead of the page. The site expects the cookie"+
						" its own script writes (Zhihu's __zse_ck, valid about 7 days);"+
						" without it every request is refused before the article is read."+
						" Re-export the site cookie from a logged-in browser tab on this"+
						" domain (Settings > Site cookies), or save the page with the"+
						" Readeck browser extension, which sends the rendered page",
					rsp.StatusCode, host, marker,
				)
			}

			hint := "no login cookie is saved for " + host +
				" yet, add one in Settings > Site cookies"
			if SiteCookieLookup != nil && SiteCookieLookup(host) {
				hint = "the cookie saved for " + host +
					" was refused, export a fresh one from a logged-in browser session" +
					" (Settings > Site cookies)"
			}
			return fmt.Errorf(
				"Invalid status code (%d): the site refused the request, %s",
				rsp.StatusCode,
				hint,
			)
		}
		return fmt.Errorf("Invalid status code (%d)", rsp.StatusCode)
	}

	switch {
	case d.IsHTML():
		return d.loadBody(rsp)
	case d.ContentType == "text/plain":
		return d.loadTextPlain(rsp)
	case strings.HasPrefix(d.ContentType, "image/"):
		return d.loadImage(rsp)
	default:
		return fmt.Errorf("unsupported content-type: %q", rsp.Header.Get("content-type"))
	}
}

// maxRedirectHops bounds the number of client side redirects followed from a
// single page.
const maxRedirectHops = 2

var (
	rxJSRedirectAssign = regexp.MustCompile(`(?i)location(?:\s*\.\s*href)?\s*=\s*["']([^"']+)["']`)
	rxJSRedirectCall   = regexp.MustCompile(`(?i)location\s*\.\s*(?:replace|assign)\s*\(\s*["']([^"']+)["']`)
)

// shareRedirect returns the target of a client side redirect when the loaded
// document is nothing but one, and nil otherwise.
func (d *Drop) shareRedirect() *url.URL {
	if !d.IsHTML() || len(d.Body) == 0 || len(d.Body) > 8192 {
		return nil
	}

	doc, err := html.Parse(bytes.NewReader(d.Body))
	if err != nil {
		return nil
	}

	target := ""
	for _, m := range dom.GetElementsByTagName(doc, "meta") {
		if !strings.EqualFold(dom.GetAttribute(m, "http-equiv"), "refresh") {
			continue
		}
		content := dom.GetAttribute(m, "content")
		if i := strings.Index(strings.ToLower(content), "url="); i >= 0 {
			target = strings.Trim(content[i+4:], `'" `)
			break
		}
	}

	// Script redirects are only trusted on a page without any text, otherwise
	// a single "location.href = …" in a real article would send us elsewhere.
	if target == "" && domTextLength(doc) < 400 {
		for _, rx := range []*regexp.Regexp{rxJSRedirectCall, rxJSRedirectAssign} {
			if m := rx.FindStringSubmatch(string(d.Body)); m != nil {
				target = m[1]
				break
			}
		}
	}

	if target == "" {
		return nil
	}

	u, err := d.URL.Parse(target)
	if err != nil || u.Hostname() == "" {
		return nil
	}
	if u.String() == d.URL.String() {
		return nil
	}
	return u
}

// domTextLength returns the length of the text found in the document's body,
// ignoring script and style contents.
func domTextLength(doc *html.Node) int {
	bodies := dom.GetElementsByTagName(doc, "body")
	if len(bodies) == 0 {
		return 0
	}

	var n int
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && (node.DataAtom == atom.Script || node.DataAtom == atom.Style) {
			return
		}
		if node.Type == html.TextNode {
			n += len(strings.TrimSpace(node.Data))
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(bodies[0])
	return n
}

// bodyReader keeps the buffered reader and the original closer together, so a
// peeked body can still be closed and drained normally.
type bodyReader struct {
	*bufio.Reader
	io.Closer
}

// IsHTML returns true when the MIME type of the resource is one of the known HTML types, and also
// when it's "application/xml" since that might be an XHTML document.
func (d *Drop) IsHTML() bool {
	switch d.ContentType {
	case "text/html", "application/xhtml+xml", "application/xml":
		return true
	default:
		return false
	}
}

// IsMedia returns true when the document type is a media type.
func (d *Drop) IsMedia() bool {
	return slices.Contains(mediaTypes, d.DocumentType)
}

// UnescapedURL returns the Drop's URL unescaped, for storage.
func (d *Drop) UnescapedURL() string {
	var (
		u   string
		err error
	)
	if u, err = url.PathUnescape(d.URL.String()); err != nil {
		return d.URL.String()
	}

	return u
}

// AddAuthors add authors to the author list, ignoring potential
// duplicates.
func (d *Drop) AddAuthors(values ...string) {
	keys := map[string]string{}
	for _, v := range d.Authors {
		keys[strings.ToLower(v)] = v
	}
	for _, v := range values {
		v = strings.TrimSpace(v)
		v = rxSpaces.ReplaceAllLiteralString(v, " ")
		v = rxAuthor.ReplaceAllString(v, "")
		if _, ok := keys[strings.ToLower(v)]; !ok {
			keys[strings.ToLower(v)] = v
		}
	}
	res := make([]string, len(keys))
	i := 0
	for _, v := range keys {
		res[i] = v
		i++
	}

	sort.Strings(res)
	d.Authors = res
}

// loadBody loads the document body and try to convert
// it to UTF-8 when encoding is different.
func (d *Drop) loadBody(rsp *http.Response) error {
	r, encName, err := HTMLReader(rsp.Body, rsp.Header.Get("content-type"))
	if err != nil {
		return fmt.Errorf("HTMLReader error: %w", err)
	}

	var body []byte
	if body, err = io.ReadAll(r); err != nil {
		return err
	}

	// Eventually set the original charset and UTF8 body
	d.Charset = encName
	d.Body = []byte(bleach.SanitizeString(string(body)))

	return nil
}

// loadTextPlain loads a plain text content and wraps it in an HTML document with
// a title.
func (d *Drop) loadTextPlain(rsp *http.Response) error {
	err := d.loadBody(rsp)
	if err != nil {
		return err
	}

	title := strings.TrimSuffix(path.Base(d.URL.Path), path.Ext(d.URL.Path))
	title = rxTitleSpaces.ReplaceAllLiteralString(title, " ")
	d.Body = fmt.Appendf(
		nil, "<html><head><title>%s</title><body><pre>%s",
		title,
		string(d.Body),
	)
	d.ContentType = "text/html"

	return nil
}

// loadImage sets the type to "photo", try to get a title and set the picture URL
// to the drop's URL.
func (d *Drop) loadImage(_ *http.Response) error {
	d.Meta.Add("x.picture_url", d.URL.String())
	d.Title = strings.TrimSuffix(path.Base(d.URL.Path), path.Ext(d.URL.Path))
	d.Title = rxTitleSpaces.ReplaceAllLiteralString(d.Title, " ")
	d.DocumentType = "photo"

	return nil
}

// fixRelativeURIs normalizes every href, src, srcset and poster
// URI values.
// It uses <base href> when present.
func (d *Drop) fixRelativeURIs(m *ProcessMessage) {
	top := m.Dom
	if top == nil {
		return
	}

	attrs := []string{"href", "src", "poster"}
	baseURL := &url.URL{}
	*baseURL = *d.URL

	m.Log().Debug("fix relative links", slog.String("base", baseURL.String()))

	// <base href> exists, we resolve its URL and set the new baseURL.
	if baseMeta := dom.QuerySelector(top, "base[href]"); baseMeta != nil {
		b := dom.GetAttribute(baseMeta, "href")
		if b != "" {
			if buri, err := url.Parse(b); err == nil {
				baseURL = baseURL.ResolveReference(buri)
				m.Log().Debug("found base tag", slog.String("url", baseURL.String()))
			}
		}
	}

	// walk through anything with href, src, poster attribute.
	for _, attr := range attrs {
		dom.ForEachNode(dom.QuerySelectorAll(top, "["+attr+"]"), func(n *html.Node, _ int) {
			newURI := toAbsoluteURI(dom.GetAttribute(n, attr), baseURL)
			dom.SetAttribute(n, attr, newURI)
		})
	}

	// srcset handler
	dom.ForEachNode(dom.QuerySelectorAll(top, "[srcset]"), func(n *html.Node, _ int) {
		srcset := dom.GetAttribute(n, "srcset")
		if srcset == "" {
			return
		}
		newSrcSet := rxSrcsetURL.ReplaceAllStringFunc(srcset, func(s string) string {
			p := rxSrcsetURL.FindStringSubmatch(s)
			return toAbsoluteURI(p[1], baseURL) + p[2] + p[3]
		})
		dom.SetAttribute(n, "srcset", newSrcSet)
	})

	dom.ForEachNode(dom.QuerySelectorAll(top, "object[data]"), func(n *html.Node, _ int) {
		newURI := toAbsoluteURI(dom.GetAttribute(n, "data"), baseURL)
		dom.SetAttribute(n, "data", newURI)
	})

	// make fragments to the same document relative
	dom.ForEachNode(dom.QuerySelectorAll(top, "a[href]"), func(n *html.Node, _ int) {
		attr := dom.GetAttribute(n, "href")
		if attr == "" {
			return
		}
		if strings.HasPrefix(attr, "#") {
			return
		}
		uri, err := url.Parse(dom.GetAttribute(n, "href"))
		if err != nil {
			return
		}
		if fragment := uri.Fragment; fragment != "" {
			if uri.RawFragment != "" {
				fragment = uri.RawFragment
			}
			uri.Fragment = ""
			tmp := new(url.URL)
			*tmp = *baseURL
			tmp.Fragment = ""
			if uri.String() == tmp.String() {
				dom.SetAttribute(n, "href", "#"+fragment)
			}
		}
	})
}

func toAbsoluteURI(uri string, base *url.URL) string {
	if uri == "" {
		return uri
	}
	if strings.HasPrefix(uri, "data:") {
		return uri
	}

	tmp, err := url.Parse(uri)
	if err != nil {
		return uri
	}

	if tmp.Scheme != "" {
		return uri
	}
	return base.ResolveReference(tmp).String()
}

// scanForCharset scans HTML tags from r until it finds the name of the character set declared in
// either the "charset" attribute or by a "http-equiv" tag.
func scanForCharset(r io.Reader) string {
	z := html.NewTokenizer(r)
	for {
		switch z.Next() {
		case html.ErrorToken:
			return ""
		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			if t.DataAtom != atom.Meta {
				continue
			}
			isContentType := false
			var metaContent string
			for _, attr := range t.Attr {
				switch attr.Key {
				case "charset":
					return attr.Val
				case "content":
					metaContent = attr.Val
				case "http-equiv":
					isContentType = strings.EqualFold(attr.Val, "content-type")
				}
			}
			if !isContentType || metaContent == "" {
				continue
			}
			if _, params, err := mime.ParseMediaType(metaContent); err == nil {
				if cs, ok := params["charset"]; ok {
					return cs
				}
			}
		}
	}
}

func scanXML(r io.Reader, isTranscoded bool) (encoding.Encoding, string, bool) {
	xd := xml.NewDecoder(r)

	var isHTML bool
	var enc encoding.Encoding
	var encName string

	// This callback will activate when a processing instruction with "encoding=..." is found.
	xd.CharsetReader = func(charsetName string, input io.Reader) (io.Reader, error) {
		if isTranscoded {
			return input, nil
		}
		enc, encName = charset.Lookup(charsetName)
		return transform.NewReader(input, enc.NewDecoder()), nil
	}

	// Read XML tokens until the first element.
	for {
		t, err := xd.Token()
		if err != nil {
			break
		}
		if el, ok := t.(xml.StartElement); ok {
			isHTML = el.Name.Local == "html" && el.Name.Space == "http://www.w3.org/1999/xhtml"
			break
		}
	}

	return enc, encName, isHTML
}

// HTMLReader wraps r in a reader that automatically transcodes a HTML document from non-UTF-8
// encoding to UTF-8. The original encoding is either determined from contentType value, or by
// scanning the initial 3kB from the document to look for things like BOM or for the meta charset
// declaration.
func HTMLReader(r io.Reader, contentType string) (io.Reader, string, error) {
	enc := encoding.Nop
	var encName string
	certain := false

	mimeType, params, err := mime.ParseMediaType(contentType)
	if err == nil {
		if cs, ok := params["charset"]; ok {
			if e, name := charset.Lookup(cs); e != nil {
				enc = e
				encName = name
				certain = true
			}
		}
	}

	var buf bytes.Buffer
	if _, err := io.CopyN(&buf, r, 1024*3); err != nil && err != io.EOF {
		return nil, "", err
	}

	switch mimeType {
	case "application/xml", "application/xhtml+xml":
		var xmlReader io.Reader = bytes.NewReader(buf.Bytes())
		if certain && enc != encoding.Nop {
			xmlReader = transform.NewReader(xmlReader, enc.NewDecoder())
		}
		e, name, isHTML := scanXML(xmlReader, certain)
		if !isHTML && mimeType == "application/xml" {
			return nil, "", fmt.Errorf("html tag not found in %s document", mimeType)
		}
		if e != nil && !certain {
			enc = e
			encName = name
			certain = true
		}
	}

	if !certain {
		// Determine encoding (fast way)
		enc, encName, certain = charset.DetermineEncoding(buf.Bytes(), contentType)
	}

	// When encoding is not 100% certain, we resort to find the charset
	// parsing part of the received HTML. More than recommended
	// by the HTMLWG, since 1024 bytes is often not enough.
	if !certain {
		if cs := scanForCharset(bytes.NewReader(buf.Bytes())); cs != "" {
			if e, name := charset.Lookup(cs); e != nil {
				enc = e
				encName = name
				certain = true
			}
		}
	}

	newReader := io.MultiReader(&buf, r)
	// No explicit encoding marker detected. Assume that page contents is already UTF-8 and that it
	// doesn't need to get transcoded.
	if !certain {
		return newReader, "utf-8", nil
	}

	if enc != encoding.Nop {
		newReader = transform.NewReader(newReader, enc.NewDecoder())
	}
	return newReader, encName, nil
}

// DropProperties contains the raw properties of an extracted page.
type DropProperties map[string]any

// DropMeta is a map of list of strings that contains the
// collected metadata.
type DropMeta map[string][]string

// Set sets a key-value pair in the raw metadata list, overriding any previous value for that key.
func (m DropMeta) Set(name, value string) {
	m[name] = []string{value}
}

// Add adds a value to the raw metadata list.
func (m DropMeta) Add(name, value string) {
	_, ok := m[name]
	if ok {
		m[name] = append(m[name], value)
	} else {
		m[name] = []string{value}
	}
}

// Lookup returns all the found values for the
// provided metadata names.
func (m DropMeta) Lookup(names ...string) []string {
	for _, x := range names {
		v, ok := m[x]
		if ok {
			return v
		}
	}

	return []string{}
}

// LookupGet returns the first value found for the
// provided metadata names.
func (m DropMeta) LookupGet(names ...string) string {
	r := m.Lookup(names...)
	if len(r) > 0 {
		return r[0]
	}
	return ""
}
