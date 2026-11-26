// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"bytes"
	"path"
	"strconv"
	"strings"

	"github.com/go-shiori/dom"
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
		cleanArticleChrome(n)
		removeEmptyFigures(n)
		stripLeadingDuplicateHeading(n, title)
		setVideoPoster(n, poster)
		normalizeArticleImages(n)
		if photoPost {
			wrapLooseImages(n)
			groupPhotoFigures(n)
		}
	}
	if showHero(item) {
		for _, n := range nodes {
			removeCoverDuplicate(n, item)
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

// cleanArticleChrome removes the parts of a saved page that are interface, not
// article: author boxes, player chrome, forum avatars and one-word control
// paragraphs. It is intentionally conservative - a short list of containers
// and exact labels - so it cannot eat a paragraph of the article.
func cleanArticleChrome(node *html.Node) {
	if node.Type != html.ElementNode {
		return
	}

	for _, sel := range []string{
		`[id*="article-author"]`, `[class*="article-author"]`,
		`[id*="video-player"]`, `[class*="video-player"]`,
		`[id*="thumbplayer"]`, `[class*="thumbplayer"]`,
		`[id*="favatar"]`, `[id*="userinfo"]`, `[id*="authicon"]`,
		`[class~="sign"]`, `[class~="rate"]`, `[class~="pstatus"]`,
	} {
		for _, n := range dom.QuerySelectorAll(node, sel) {
			removeNode(n)
		}
	}

	for _, img := range dom.QuerySelectorAll(node, "img") {
		alt := strings.ToLower(dom.GetAttribute(img, "alt"))
		if !strings.Contains(alt, "头像") && !strings.Contains(alt, "avatar") &&
			!strings.Contains(alt, "thumbplayer poster") {
			continue
		}
		if px := imagePixelSize(dom.GetAttribute(img, "width")); px == 0 || px <= 200 {
			removeNode(img)
		}
	}

	chromeText := map[string]bool{
		"关注": true, "打开循环播放": true, "正在加载 视频播放器。": true,
		"广告": true, "视频加载中...": true, "点击查看大图": true,
	}
	for _, p := range dom.QuerySelectorAll(node, "p") {
		text := strings.TrimSpace(textContent(p))
		if len([]rune(text)) <= 16 && chromeText[text] {
			removeNode(p)
			continue
		}
		// Discuz attachment metadata: it sits between two pictures and breaks
		// the gallery run into a long vertical list.
		if strings.Contains(text, "下载次数") ||
			(strings.Contains(text, "上传") && len([]rune(text)) <= 24) {
			removeNode(p)
		}
	}
}

// removeEmptyFigures drops picture wrappers left behind when the archive
// removed a protected image. They otherwise render as blank gaps.
func removeEmptyFigures(node *html.Node) {
	if node.Type != html.ElementNode {
		return
	}
	for _, fig := range dom.QuerySelectorAll(node, "figure") {
		if dom.QuerySelector(fig, "img, video, audio, iframe, svg") != nil {
			continue
		}
		if strings.TrimSpace(textContent(fig)) != "" {
			continue
		}
		removeNode(fig)
	}
}

// normalizeArticleImages gives the article a consistent picture rhythm: wide
// screenshots fill the column, small diagrams keep their size, and 16px
// smilies do not become posters.
func normalizeArticleImages(node *html.Node) {
	if node.Type != html.ElementNode {
		return
	}
	for _, img := range dom.QuerySelectorAll(node, "img") {
		if getAttr(img, "data-iframe-params") != "" {
			continue
		}
		class := "prose-image-natural"
		lowerSrc := strings.ToLower(dom.GetAttribute(img, "src"))
		smallWord := strings.Contains(lowerSrc, "smilie") ||
			strings.Contains(lowerSrc, "smiley") ||
			strings.Contains(lowerSrc, "emoji") ||
			strings.Contains(lowerSrc, "avatar") ||
			strings.Contains(lowerSrc, "icon")
		switch px := imagePixelSize(dom.GetAttribute(img, "width")); {
		case smallWord || (px > 0 && px <= 64):
			class = "prose-image-icon"
		case px == 0 || px >= 600:
			class = "prose-image-full"
		}
		addClass(img, class)
	}
}

// removeCoverDuplicate removes the first article picture that is also the
// bookmark cover. The reader already shows it as a hero; repeating it inside
// the article made one image huge and the next one small.
func removeCoverDuplicate(node *html.Node, item *dataset.Bookmark) {
	if node.Type != html.ElementNode {
		return
	}
	img, ok := item.Resources["image"]
	if !ok || img == nil || img.Src == "" {
		return
	}
	heroBase := path.Base(strings.SplitN(img.Src, "?", 2)[0])

	for _, n := range dom.QuerySelectorAll(node, "img") {
		src := dom.GetAttribute(n, "src")
		if src == "" {
			continue
		}
		if src == img.Src || path.Base(strings.SplitN(src, "?", 2)[0]) == heroBase {
			removeNode(n)
			return
		}
	}
}

func removeNode(n *html.Node) {
	parent := n.Parent
	if parent == nil {
		return
	}
	parent.RemoveChild(n)
	if parent.DataAtom != atom.P && parent.DataAtom != atom.Figure {
		return
	}
	if strings.TrimSpace(textContent(parent)) == "" && !hasMedia([]*html.Node{parent}) {
		if parent.Parent != nil {
			parent.Parent.RemoveChild(parent)
		}
	}
}

func imagePixelSize(v string) int {
	v = strings.TrimSuffix(strings.TrimSpace(v), "px")
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func addClass(n *html.Node, class string) {
	cur := getAttr(n, "class")
	if cur == "" {
		setAttr(n, "class", class)
		return
	}
	for _, c := range strings.Fields(cur) {
		if c == class {
			return
		}
	}
	setAttr(n, "class", cur+" "+class)
}

// groupPhotoFigures wraps runs of two or more consecutive picture figures in a
// gallery container. A single picture keeps the normal prose layout.
// wrapLooseImages turns a paragraph that holds nothing but a picture into a
// figure. Forum and news pages use <p><img></p> or <div><img></div>; without
// this, a 26-picture post stays a column of wildly different natural sizes and
// never reaches the gallery layout.
func wrapLooseImages(node *html.Node) {
	if node.Type != html.ElementNode {
		return
	}

	for _, br := range dom.QuerySelectorAll(node, "br") {
		removeNode(br)
	}

	for _, img := range dom.QuerySelectorAll(node, "img") {
		if px := imagePixelSize(dom.GetAttribute(img, "width")); px > 0 && px < 200 {
			continue
		}
		alt := strings.ToLower(dom.GetAttribute(img, "alt"))
		if strings.Contains(alt, "头像") || strings.Contains(alt, "avatar") {
			continue
		}
		parent := img.Parent
		if parent == nil || (parent.DataAtom != atom.P && parent.DataAtom != atom.Div) {
			continue
		}
		if strings.TrimSpace(textContent(parent)) != "" {
			continue
		}

		fig := &html.Node{Type: html.ElementNode, Data: "figure", DataAtom: atom.Figure}
		container := img
		if parent.DataAtom == atom.P && parent.FirstChild != nil && parent.FirstChild.DataAtom == atom.A {
			container = parent.FirstChild
		}
		parent.RemoveChild(container)
		fig.AppendChild(container)
		if parent.Parent != nil {
			parent.Parent.InsertBefore(fig, parent)
			parent.Parent.RemoveChild(parent)
		}
	}
}

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
		if n.DataAtom == atom.Img && (n.Parent == nil || n.Parent.DataAtom != atom.Figure) {
			if px := imagePixelSize(dom.GetAttribute(n, "width")); px == 0 || px >= 200 {
				count++
			}
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
