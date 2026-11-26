// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"context"
	"encoding/json"
	"fmt"
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

type aiJSONResponse struct {
	OK      bool     `json:"ok"`
	Error   string   `json:"error,omitempty"`
	Summary string   `json:"summary,omitempty"`
	Title   string   `json:"title,omitempty"`
	Labels  []string `json:"labels,omitempty"`
	HTML    string   `json:"html,omitempty"`
}

func wantsJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json") ||
		r.Header.Get("X-Requested-With") == "XMLHttpRequest"
}

func writeAIJSON(w http.ResponseWriter, status int, res aiJSONResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(res)
}

// aiClean runs summary or cleanup. With an AJAX request it returns JSON so the
// sidebar can show progress and an error instead of a page that appears dead.
func (h *viewsRouter) aiClean(w http.ResponseWriter, r *http.Request) {
	b := getBookmark(r.Context())
	jsonMode := wantsJSON(r)
	fail := func(msg string) {
		if jsonMode {
			writeAIJSON(w, http.StatusBadRequest, aiJSONResponse{Error: msg})
			return
		}
		_ = server.AddFlash(w, r, "error", msg)
		server.Redirect(w, r, "/bookmarks/"+b.UID)
	}

	client := ai.New(aiConfig(r.Context()))
	if !client.Configured() {
		fail(server.Locale(r.Context()).Gettext("AI is not configured."))
		return
	}

	bi := dataset.NewBookmark(r.Context(), b)
	reader, err := bi.GetArticle()
	if err != nil {
		fail(server.Locale(r.Context()).Gettext("Could not read the saved article."))
		return
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		fail(server.Locale(r.Context()).Gettext("Could not read the saved article."))
		return
	}

	outline, images := buildAIOutline(string(raw))
	if strings.TrimSpace(outline) == "" {
		fail(server.Locale(r.Context()).Gettext("There is no text to clean."))
		return
	}

	mode := strings.ToLower(strings.TrimSpace(r.FormValue("mode")))
	if mode == "" {
		mode = "clean"
	}
	instruction := strings.TrimSpace(r.FormValue("instruction"))
	if r.FormValue("deep") == "1" {
		instruction = "先深入分析文章结构和用户要求，再执行。\n" + instruction
	}
	switch mode {
	case "summary":
		instruction = "只做摘要，不要改写正文。用两三句话总结文章；如果正文主要是链接列表，说明它是什么列表。"
	case "links":
		instruction = "删除导航、相关文章、推荐阅读、作者简介、评论和所有链接列表；保留正文段落和图片。不要保留任何多余的超链接。"
	case "chat":
		if instruction == "" {
			fail("请先输入想让 AI 做什么。")
			return
		}
		system := "你是阅读助手。只根据下面的文章内容回答用户，不要编造；用中文，简洁。"
		context := fmt.Sprintf("标题: %s\nURL: %s\n\n正文:\n%s\n\n图片:\n%s",
			b.Title, b.URL, outline, aiImageList(images))
		answer, err := client.Ask(r.Context(), system, context+"\n\n用户要求:\n"+instruction)
		if err != nil {
			server.Log(r).Warn("ai chat", slog.Any("err", err))
			fail(server.Locale(r.Context()).Gettext("AI cleanup failed: %s", err.Error()))
			return
		}
		if jsonMode {
			writeAIJSON(w, http.StatusOK, aiJSONResponse{OK: true, Summary: answer})
			return
		}
		_ = server.AddFlash(w, r, "success", answer)
		server.Redirect(w, r, "/bookmarks/"+b.UID)
		return
	}

	result, err := client.CleanWithInstruction(r.Context(), b.Title, b.URL, outline, images, instruction)
	if err != nil {
		server.Log(r).Warn("ai cleanup", slog.Any("err", err))
		fail(server.Locale(r.Context()).Gettext("AI cleanup failed: %s", err.Error()))
		return
	}

	summary := strings.TrimSpace(result.Summary)
	if summary == "" {
		var parts []string
		for _, block := range result.Blocks {
			if t := strings.TrimSpace(block.Text); t != "" {
				parts = append(parts, t)
			}
		}
		summary = strings.Join(parts, " ")
	}

	var body string
	if mode != "summary" {
		body = renderAIBlocks(result.Blocks, images)
		if strings.TrimSpace(body) == "" {
			fail(server.Locale(r.Context()).Gettext("AI returned an empty article."))
			return
		}
		if err := b.RewriteArticle(body); err != nil {
			server.Log(r).Error("ai rewrite", slog.Any("err", err))
			fail(server.Locale(r.Context()).Gettext("Could not save the cleaned article."))
			return
		}
	}

	if mode != "summary" {
		if t := strings.TrimSpace(result.Title); t != "" && len([]rune(t)) <= 512 {
			b.Title = t
		}
	}
	if summary != "" {
		b.Description = summary
	}
	for _, label := range result.Labels {
		label = strings.TrimSpace(label)
		if label == "" || len([]rune(label)) > 48 {
			continue
		}
		b.Labels = append(b.Labels, label)
	}
	if mode != "summary" {
		b.Labels = append(b.Labels, "AI整理")
	}
	slices.Sort(b.Labels)
	b.Labels = slices.Compact(b.Labels)
	b.Updated = time.Now().UTC()
	if err := b.Save(); err != nil {
		server.Log(r).Error("ai save", slog.Any("err", err))
		fail(server.Locale(r.Context()).Gettext("The article was cleaned but the bookmark could not be updated."))
		return
	}

	if jsonMode {
		writeAIJSON(w, http.StatusOK, aiJSONResponse{
			OK: true, Summary: summary, Title: b.Title, Labels: b.Labels, HTML: body,
		})
		return
	}
	_ = server.AddFlash(w, r, "success", server.Locale(r.Context()).Gettext("AI cleanup finished."))
	server.Redirect(w, r, "/bookmarks/"+b.UID)
}

func aiImageList(images []ai.Image) string {
	var b strings.Builder
	for i, img := range images {
		fmt.Fprintf(&b, "[IMG %d] %s\n", i, img.URL)
	}
	return b.String()
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
