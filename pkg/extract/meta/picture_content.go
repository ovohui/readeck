// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package meta

import (
	"bytes"
	"context"
	"log/slog"
	"path"
	"strings"

	"github.com/go-shiori/dom"
	"golang.org/x/net/html"

	"codeberg.org/readeck/readeck/pkg/extract"
)

// Words that almost always point at chrome around the article: avatars,
// emoji, site logos, spacer and placeholder images. A cover made of these is
// worse than the empty state it replaces.
var coverImageSkipWords = []string{
	"avatar", "icon", "logo", "emoji", "emoticon", "smiley", "smilies",
	"spacer", "blank", "pixel", "placeholder", "loading", "none.gif",
	"emp.gif", "gravatar", "captcha", "badge", "button", "arrow", "bg.png",
}

// ExtractContentPicture uses the first meaningful picture of the article as
// the bookmark cover when the page itself declares none.
//
// A lot of sites have no og:image at all - forums, Chinese Q&A, mobile share
// pages and every page saved by the browser extension - so the bookmark list
// ends up showing a grey placeholder while the article below is full of
// pictures. Inline pictures (data:image/webp;base64,...) count: on Xiaohongshu
// the whole note is inlined, so skipping them means the card has no cover at
// all. Video pages are included too, their only picture being the cover the
// player shows before it starts.
// Must run after the body is rendered, that is, at StepFinish in the
// processor chain.
func ExtractContentPicture(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepFinish || m.Position() > 0 {
		return next
	}

	d := m.Extractor.Drop()
	switch d.DocumentType {
	case "", "article", "photo", "video":
	default:
		return next
	}
	if len(d.Body) == 0 {
		return next
	}

	// A declared cover wins, unless it is too small to be artwork: sites that
	// have no og:image often point it at a 200px logo or a share badge, and
	// the card then shows the logo instead of the picture the article is
	// about.
	old, hasOld := d.Pictures["image"]
	if hasOld && !weakCover(old) {
		return next
	}

	href := firstBodyImage(d.Body)
	if href == "" {
		return next
	}

	size := uint(extract.ImageSizeRegular)
	if d.DocumentType == "photo" {
		size = extract.ImageSizeWide
	}

	picture, err := extract.NewPicture(href, d.URL)
	if err != nil {
		return next
	}

	ctx := extract.WithReferrer(context.Background(), d.URL)
	if err = picture.Load(ctx, m.Extractor.Client(), size, ""); err != nil {
		m.Log().Debug(
			"no usable picture in content",
			slog.Any("err", err),
			slog.String("url", shortHref(href)),
		)
		return next
	}

	// Keep whichever is bigger. A cover that is only slightly better than the
	// one already there is not worth a second download.
	if hasOld && old.Size[0] >= picture.Size[0] {
		return next
	}

	d.Pictures["image"] = picture
	if thumbnail, err := picture.Copy(extract.ImageSizeThumbnail, ""); err == nil {
		d.Pictures["thumbnail"] = thumbnail
	}
	m.Log().Debug(
		"picture taken from content",
		slog.String("url", shortHref(href)),
		slog.Any("size", picture.Size[:]),
	)

	return next
}

// minCoverWidth is the width below which a declared cover is treated as an
// icon rather than artwork.
const minCoverWidth = 400

// weakCover reports whether an already declared cover is worth replacing.
// A picture whose size is still unknown is left alone: it may not have been
// loaded yet, and guessing would throw away a good image.
func weakCover(p *extract.Picture) bool {
	return p == nil || (p.Size[0] > 0 && p.Size[0] < minCoverWidth)
}

// firstBodyImage returns the first image URL of a rendered article that looks
// like a real picture rather than an interface element.
func firstBodyImage(body []byte) string {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return ""
	}

	for _, img := range dom.GetElementsByTagName(doc, "img") {
		if href := coverImageHref(img); href != "" {
			return href
		}
	}

	return ""
}

func coverImageHref(img *html.Node) string {
	// Lazy loading leaves an inline placeholder in src and the real URL in a
	// data attribute, so a data: src must not end the search.
	src := strings.TrimSpace(dom.GetAttribute(img, "src"))
	inline := isInlinePicture(src)

	href := src
	if href == "" || inline || strings.HasPrefix(href, "blob:") {
		href = ""
		for _, attr := range []string{"data-src", "data-original", "data-actualsrc", "file"} {
			v := strings.TrimSpace(dom.GetAttribute(img, attr))
			if v != "" && !isInlinePicture(v) {
				href = v
				break
			}
		}
	}
	if href == "" {
		if !inline {
			return ""
		}
		// The picture never existed as a URL: the page inlined it
		// (data:image/webp;base64,...), which is what Xiaohongshu and every
		// page saved by the browser extension do. Keep it, but only after the
		// size attributes have had their say, and skip the URL heuristics
		// below: base64 is random bytes and will happily contain "icon".
		for _, attr := range []string{"width", "height"} {
			if px, ok := pixelSize(dom.GetAttribute(img, attr)); ok && px < 200 {
				return ""
			}
		}
		return src
	}

	// An explicitly small image is a bullet point, an avatar or an emoji.
	for _, attr := range []string{"width", "height"} {
		if px, ok := pixelSize(dom.GetAttribute(img, attr)); ok && px < 200 {
			return ""
		}
	}

	lower := strings.ToLower(href)
	if strings.HasSuffix(strings.SplitN(lower, "?", 2)[0], ".svg") {
		return ""
	}
	for _, word := range coverImageSkipWords {
		if strings.Contains(lower, word) {
			return ""
		}
	}
	for _, attr := range []string{"class", "id", "alt"} {
		v := strings.ToLower(dom.GetAttribute(img, attr))
		for _, word := range []string{"avatar", "emoji", "emoticon", "icon", "logo", "badge"} {
			if strings.Contains(v, word) {
				return ""
			}
		}
	}

	// The last path segment should look like a file name, which keeps out
	// tracking pixels served from a bare endpoint.
	if name := path.Base(strings.SplitN(lower, "?", 2)[0]); name == "." || name == "/" || name == "" {
		return ""
	}

	return href
}

// isInlinePicture reports whether a src attribute carries the picture itself.
func isInlinePicture(s string) bool {
	return len(s) > 11 && strings.EqualFold(s[:11], "data:image/")
}

// shortHref keeps a base64 payload out of the log.
func shortHref(href string) string {
	if len(href) <= 90 {
		return href
	}
	return href[:90] + "..."
}

// pixelSize parses a width/height attribute, ignoring CSS units and values
// such as "100%".
func pixelSize(v string) (int, bool) {
	v = strings.TrimSuffix(strings.TrimSpace(v), "px")
	if v == "" {
		return 0, false
	}
	n := 0
	for _, c := range v {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}
