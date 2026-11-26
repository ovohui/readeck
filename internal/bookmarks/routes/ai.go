// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"context"
	stdhtml "html"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"codeberg.org/readeck/readeck/internal/ai"
	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/bookmarks/dataset"
	"codeberg.org/readeck/readeck/internal/server"
)

func aiConfig(ctx context.Context) ai.Config {
	u := auth.GetUser(ctx)
	if u == nil {
		return ai.Config{}
	}
	s := u.Settings.AI
	return ai.Config{
		Provider: s.Provider,
		BaseURL:  s.BaseURL,
		APIKey:   s.APIKey,
		Model:    s.Model,
	}
}

func aiEnabled(ctx context.Context) bool {
	u := auth.GetUser(ctx)
	if u == nil || !u.Settings.AI.Enabled {
		return false
	}
	return ai.New(aiConfig(ctx)).Configured()
}

// aiClean runs the synchronous cleanup. It is deliberately a normal POST: the
// user sees a page reload with the result, and no API key ever reaches the
// browser.
func (h *viewsRouter) aiClean(w http.ResponseWriter, r *http.Request) {
	b := getBookmark(r.Context())
	cfg := aiConfig(r.Context())
	client := ai.New(cfg)
	if !client.Configured() {
		_ = server.AddFlash(w, r, "error", server.Locale(r.Context()).Gettext("AI is not configured."))
		server.Redirect(w, r, "/bookmarks/"+b.UID)
		return
	}

	bi := dataset.NewBookmark(r.Context(), b)
	reader, err := bi.GetArticle()
	if err != nil {
		_ = server.AddFlash(w, r, "error", server.Locale(r.Context()).Gettext("Could not read the saved article."))
		server.Redirect(w, r, "/bookmarks/"+b.UID)
		return
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		_ = server.AddFlash(w, r, "error", server.Locale(r.Context()).Gettext("Could not read the saved article."))
		server.Redirect(w, r, "/bookmarks/"+b.UID)
		return
	}

	outline, images := buildAIOutline(string(raw))
	if strings.TrimSpace(outline) == "" {
		_ = server.AddFlash(w, r, "error", server.Locale(r.Context()).Gettext("There is no text to clean."))
		server.Redirect(w, r, "/bookmarks/"+b.UID)
		return
	}

	result, err := client.Clean(r.Context(), b.Title, b.URL, outline, images)
	if err != nil {
		server.Log(r).Warn("ai cleanup", slog.Any("err", err))
		_ = server.AddFlash(w, r, "error",
			server.Locale(r.Context()).Gettext("AI cleanup failed: %s", err.Error()))
		server.Redirect(w, r, "/bookmarks/"+b.UID)
		return
	}

	body := renderAIBlocks(result.Blocks, images)
	if strings.TrimSpace(body) == "" {
		_ = server.AddFlash(w, r, "error", server.Locale(r.Context()).Gettext("AI returned an empty article."))
		server.Redirect(w, r, "/bookmarks/"+b.UID)
		return
	}
	if err := b.RewriteArticle(body); err != nil {
		server.Log(r).Error("ai rewrite", slog.Any("err", err))
		_ = server.AddFlash(w, r, "error", server.Locale(r.Context()).Gettext("Could not save the cleaned article."))
		server.Redirect(w, r, "/bookmarks/"+b.UID)
		return
	}

	if t := strings.TrimSpace(result.Title); t != "" && len([]rune(t)) <= 512 {
		b.Title = t
	}
	if s := strings.TrimSpace(result.Summary); s != "" {
		b.Description = s
	}
	for _, label := range result.Labels {
		label = strings.TrimSpace(label)
		if label == "" || len([]rune(label)) > 48 {
			continue
		}
		b.Labels = append(b.Labels, label)
	}
	b.Labels = append(b.Labels, "AI整理")
	slices.Sort(b.Labels)
	b.Labels = slices.Compact(b.Labels)
	b.Updated = time.Now().UTC()
	if err := b.Save(); err != nil {
		server.Log(r).Error("ai save", slog.Any("err", err))
		_ = server.AddFlash(w, r, "error", server.Locale(r.Context()).Gettext("The article was cleaned but the bookmark could not be updated."))
		server.Redirect(w, r, "/bookmarks/"+b.UID)
		return
	}

	_ = server.AddFlash(w, r, "success", server.Locale(r.Context()).Gettext("AI cleanup finished."))
	server.Redirect(w, r, "/bookmarks/"+b.UID)
}

// buildAIOutline flattens the archived article into a compact outline with
// [IMG n] placeholders, so the model never has to output arbitrary HTML.
func buildAIOutline(raw string) (string, []ai.Image) {
	ctx := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := html.ParseFragment(strings.NewReader(raw), ctx)
	if err != nil {
		return "", nil
	}

	var b strings.Builder
	images := []ai.Image{}
	seen := map[string]bool{}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			text := strings.Join(strings.Fields(n.Data), " ")
			if text != "" {
				b.WriteString(text)
				b.WriteByte(' ')
			}
			return
		}
		if n.Type != html.ElementNode {
			return
		}

		switch n.DataAtom {
		case atom.Script, atom.Style, atom.Noscript, atom.Svg, atom.Video, atom.Audio, atom.Iframe:
			return
		case atom.Img:
			src := getAttr(n, "src")
			if src == "" || strings.HasPrefix(src, "data:") {
				return
			}
			if px := imagePixelSize(getAttr(n, "width")); px > 0 && px < 64 {
				return
			}
			alt := strings.ToLower(getAttr(n, "alt"))
			if strings.Contains(alt, "avatar") || strings.Contains(alt, "头像") ||
				strings.Contains(alt, "smilie") {
				return
			}
			if seen[src] {
				return
			}
			seen[src] = true
			images = append(images, ai.Image{
				URL:    src,
				Alt:    getAttr(n, "alt"),
				Width:  imagePixelSize(getAttr(n, "width")),
				Height: imagePixelSize(getAttr(n, "height")),
			})
			b.WriteString("\n[IMG ")
			b.WriteString(strconv.Itoa(len(images) - 1))
			b.WriteString("]\n")
			return
		case atom.P, atom.Li, atom.Blockquote, atom.H1, atom.H2, atom.H3, atom.H4:
			b.WriteByte('\n')
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			b.WriteByte('\n')
			return
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for _, n := range nodes {
		walk(n)
	}

	out := strings.TrimSpace(b.String())
	out = strings.ReplaceAll(out, " [IMG ", "\n[IMG ")
	out = strings.ReplaceAll(out, "] ", "]\n")
	for strings.Contains(out, "\n\n\n") {
		out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	}
	if len([]rune(out)) > 30000 {
		out = string([]rune(out)[:30000])
	}
	return out, images
}

func renderAIBlocks(blocks []ai.Block, images []ai.Image) string {
	var b strings.Builder
	used := map[int]bool{}

	writeImg := func(index int, caption string) {
		if index < 0 || index >= len(images) || used[index] {
			return
		}
		used[index] = true
		b.WriteString(`<figure><img loading="lazy" src="`)
		b.WriteString(stdhtml.EscapeString(images[index].URL))
		b.WriteString(`" alt="`)
		b.WriteString(stdhtml.EscapeString(images[index].Alt))
		b.WriteString(`">`)
		if c := strings.TrimSpace(caption); c != "" {
			b.WriteString(`<figcaption>`)
			b.WriteString(stdhtml.EscapeString(c))
			b.WriteString(`</figcaption>`)
		}
		b.WriteString(`</figure>`)
	}

	for _, block := range blocks {
		text := strings.TrimSpace(block.Text)
		switch strings.ToLower(block.Type) {
		case "p", "paragraph":
			if text != "" {
				b.WriteString("<p>")
				b.WriteString(stdhtml.EscapeString(text))
				b.WriteString("</p>")
			}
		case "h2":
			if text != "" {
				b.WriteString("<h2>")
				b.WriteString(stdhtml.EscapeString(text))
				b.WriteString("</h2>")
			}
		case "h3":
			if text != "" {
				b.WriteString("<h3>")
				b.WriteString(stdhtml.EscapeString(text))
				b.WriteString("</h3>")
			}
		case "quote", "blockquote":
			if text != "" {
				b.WriteString("<blockquote><p>")
				b.WriteString(stdhtml.EscapeString(text))
				b.WriteString("</p></blockquote>")
			}
		case "list", "ul", "ol":
			items := block.Items
			if len(items) == 0 && text != "" {
				items = strings.Split(text, "\n")
			}
			if len(items) > 0 {
				b.WriteString("<ul>")
				for _, item := range items {
					item = strings.TrimSpace(item)
					if item == "" {
						continue
					}
					b.WriteString("<li>")
					b.WriteString(stdhtml.EscapeString(item))
					b.WriteString("</li>")
				}
				b.WriteString("</ul>")
			}
		case "img", "image":
			writeImg(block.Index, block.Caption)
		}
	}

	// The model sometimes drops pictures while shortening a post. Put every
	// omitted picture back at the end in its original order; no image is lost.
	var missing []ai.Block
	for i := range images {
		if !used[i] {
			missing = append(missing, ai.Block{Type: "img", Index: i})
		}
	}
	if len(missing) > 0 {
		b.WriteString(`<div class="photo-gallery">`)
		for _, block := range missing {
			writeImg(block.Index, "")
		}
		b.WriteString(`</div>`)
	}

	return b.String()
}
