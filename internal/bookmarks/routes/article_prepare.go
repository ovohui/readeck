// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"codeberg.org/readeck/readeck/internal/bookmarks/dataset"
)

// prepareArticleHTML applies the presentation fixes that should not be baked
// into the archive: they must also apply to bookmarks saved months ago, and
// the archived document has to stay untouched for exports.
//
//   - The archived video gets the bookmark cover as its poster. The archiver
//     only kept the poster attribute of the placeholder, and the Bilibili /
//     Xiaohongshu placeholders carry their cover in src, so old archives would
//     otherwise open on a black frame.
//   - A leading heading that repeats the title is removed. WeChat, Tencent and
//     Hupu articles start with their own <h2> and the page already shows the
//     title above.
//   - A photo post's figures are grouped in a grid instead of one huge image
//     per screen.
func prepareArticleHTML(item *dataset.Bookmark, raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}

	ctx := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := html.ParseFragment(strings.NewReader(raw), ctx)
	if err != nil {
		return raw
	}

	title := displayTitle(item)
	poster := ""
	if img, ok := item.Resources["image"]; ok && img != nil {
		poster = img.Src
	}

	photoPost := item.Type == "photo" || isPhotoPost(item, nodes)
	for _, n := range nodes {
		stripLeadingDuplicateHeading(n, title)
		setVideoPoster(n, poster)
		if photoPost {
			groupPhotoFigures(n)
		}
	}

	if visibleText(nodes) == "" && !hasMedia(nodes) {
		return ""
	}

	buf := &bytes.Buffer{}
	for _, n := range nodes {
		if err := html.Render(buf, n); err != nil {
			return raw
		}
	}
	return buf.String()
}

// stripLeadingDuplicateHeading removes the first heading of the article when it
// is the title again. It only looks at the first element of the first
// container, never deeper, so a heading in the middle of the text is safe.
func stripLeadingDuplicateHeading(node *html.Node, title string) {
	if node.Type != html.ElementNode {
		return
	}
	if title == "" {
		return
	}

	for child := firstElementChild(node); child != nil; child = nextElementSibling(child) {
		switch child.DataAtom {
		case atom.H1, atom.H2, atom.H3:
			if headingIsTitle(child, title) {
				node.RemoveChild(child)
			}
			return
		case atom.Article, atom.Section, atom.Div, atom.Main:
			stripLeadingDuplicateHeading(child, title)
			return
		default:
			return
		}
	}
}

func headingIsTitle(h *html.Node, title string) bool {
	text := compareText(textContent(h))
	want := compareText(title)
	if text == "" || want == "" {
		return false
	}
	if text == want {
		return true
	}
	// The title may keep a suffix the heading does not have (or the other way
	// around), but it must still be the same sentence, not a short section
	// heading that happens to share a few words.
	if strings.HasPrefix(text, want) || strings.HasPrefix(want, text) {
		short, long := len([]rune(text)), len([]rune(want))
		if short > long {
			short, long = long, short
		}
		return short >= long*8/10
	}
	return false
}

func setVideoPoster(node *html.Node, poster string) {
	if node.Type != html.ElementNode {
		return
	}
	if node.DataAtom == atom.Video && poster != "" {
		if getAttr(node, "poster") == "" {
			setAttr(node, "poster", poster)
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		setVideoPoster(child, poster)
	}
}

// groupPhotoFigures wraps runs of two or more consecutive picture figures in a
// gallery container. A single picture keeps the normal prose layout.
func groupPhotoFigures(node *html.Node) {
	if node.Type != html.ElementNode {
		return
	}

	for child := node.FirstChild; child != nil; {
		if isPictureFigure(child) {
			run := []*html.Node{}
			for child != nil && isPictureFigure(child) {
				run = append(run, child)
				child = child.NextSibling
			}
			if len(run) >= 2 && node.DataAtom != atom.Figure {
				gallery := &html.Node{
					Type:     html.ElementNode,
					Data:     "div",
					DataAtom: atom.Div,
					Attr:     []html.Attribute{{Key: "class", Val: "photo-gallery"}},
				}
				node.InsertBefore(gallery, run[0])
				for _, fig := range run {
					node.RemoveChild(fig)
					gallery.AppendChild(fig)
				}
			}
			continue
		}

		next := child.NextSibling
		groupPhotoFigures(child)
		child = next
	}
}

// isPhotoPost decides whether the figures should be laid out as a gallery.
// Xiaohongshu notes are saved as "article" documents by their content script,
// so the document type alone is not enough: a short text with three or more
// pictures is a photo note whatever the site calls it.
func isPhotoPost(item *dataset.Bookmark, nodes []*html.Node) bool {
	count := 0
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		if n.DataAtom == atom.Figure && isPictureFigure(n) {
			count++
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	if count < 3 {
		return false
	}
	if len([]rune(visibleText(nodes))) <= 800 {
		return true
	}
	return strings.Contains(item.Site, "xiaohongshu") ||
		strings.Contains(item.Site, "douban") ||
		strings.Contains(item.Site, "weibo")
}

func isPictureFigure(n *html.Node) bool {
	if n.Type != html.ElementNode || n.DataAtom != atom.Figure {
		return false
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.DataAtom == atom.Img {
			return true
		}
	}
	return false
}

func firstElementChild(n *html.Node) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			return c
		}
	}
	return nil
}

func nextElementSibling(n *html.Node) *html.Node {
	for c := n.NextSibling; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			return c
		}
	}
	return nil
}

func textContent(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		if node.Type == html.ElementNode && (node.DataAtom == atom.Script || node.DataAtom == atom.Style) {
			return
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func visibleText(nodes []*html.Node) string {
	var b strings.Builder
	for _, n := range nodes {
		b.WriteString(textContent(n))
	}
	return strings.TrimSpace(strings.Join(strings.Fields(b.String()), " "))
}

func hasMedia(nodes []*html.Node) bool {
	var found bool
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		switch n.DataAtom {
		case atom.Video, atom.Img, atom.Iframe, atom.Audio:
			found = true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	return found
}

func getAttr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, name) {
			return a.Val
		}
	}
	return ""
}

func setAttr(n *html.Node, name, value string) {
	for i := range n.Attr {
		if strings.EqualFold(n.Attr[i].Key, name) {
			n.Attr[i].Val = value
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: name, Val: value})
}
