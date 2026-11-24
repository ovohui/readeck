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
// A lot of sites have no og:image at all - forums, Chinese Q&A and mobile
// share pages - so the bookmark list ends up showing a grey placeholder while
// the article below is full of pictures. Must run after the body is rendered,
// that is, at StepFinish in the processor chain.
func ExtractContentPicture(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepFinish || m.Position() > 0 {
		return next
	}

	d := m.Extractor.Drop()
	if _, ok := d.Pictures["image"]; ok {
		return next
	}
	switch d.DocumentType {
	case "", "article", "photo":
	default:
		return next
	}
	if len(d.Body) == 0 {
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
			slog.String("url", href),
		)
		return next
	}

	d.Pictures["image"] = picture
	if thumbnail, err := picture.Copy(extract.ImageSizeThumbnail, ""); err == nil {
		d.Pictures["thumbnail"] = thumbnail
	}
	m.Log().Debug(
		"picture taken from content",
		slog.String("url", href),
		slog.Any("size", picture.Size[:]),
	)

	return next
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
	href := strings.TrimSpace(dom.GetAttribute(img, "src"))
	if href == "" || strings.HasPrefix(href, "data:") || strings.HasPrefix(href, "blob:") {
		href = ""
		for _, attr := range []string{"data-src", "data-original", "data-actualsrc", "file"} {
			v := strings.TrimSpace(dom.GetAttribute(img, attr))
			if v != "" && !strings.HasPrefix(v, "data:") {
				href = v
				break
			}
		}
	}
	if href == "" {
		return ""
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
