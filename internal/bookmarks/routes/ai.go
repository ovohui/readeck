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
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"codeberg.org/readeck/readeck/internal/ai"
	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/bookmarks/dataset"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/internal/server/urls"
)

func aiConfig(ctx context.Context) ai.Config {
	u := auth.GetUser(ctx)
	if u == nil {
		return ai.Config{}
	}
	s := u.Settings.AI
	return ai.Config{
		Provider:    s.Provider,
		BaseURL:     s.BaseURL,
		APIKey:      s.APIKey,
		Model:       s.Model,
		VisionModel: s.VisionModel,
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
	aiCtx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
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
	case "clean":
		base := "按原文的标题层级整理正文：保留 h2/h3、段落、列表、引用和图片；删除广告、导航、来源声明、作者卡、相关推荐、评论、二维码等无关元素。"
		if instruction != "" {
			base += "\n用户额外要求：" + instruction
		}
		instruction = base
	case "ocr":
		var imageURLs []string
		for _, img := range images {
			if len(imageURLs) >= 8 {
				break
			}
			imageURLs = append(imageURLs, urls.AbsoluteURL(r, img.URL).String())
		}
		if len(imageURLs) == 0 {
			fail("这篇文章没有图片。")
			return
		}
		system := "你是 OCR 助手。只提取图片中的文字，按图片顺序和版面层级输出纯文本；不要描述画面，不要编造。"
		prompt := "识别这些图片中的文字，并按标题、段落、列表的层级整理。"
		if instruction != "" {
			prompt += "\n用户要求：" + instruction
		}
		answer, err := client.AskVision(r.Context(), "", system, prompt, imageURLs)
		if err != nil {
			server.Log(r).Warn("ai ocr", slog.Any("err", err))
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
	case "chat":
		if instruction == "" {
			fail("请先输入想让 AI 做什么。")
			return
		}
		system := "你是阅读助手。只根据下面的文章内容回答用户，不要编造；用中文，简洁。"
		context := fmt.Sprintf("标题: %s\nURL: %s\n\n正文:\n%s\n\n图片:\n%s",
			b.Title, b.URL, outline, aiImageList(images))
		answer, err := client.Ask(aiCtx, system, context+"\n\n用户要求:\n"+instruction)
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
	case "tag":
		system := "你是书签标签助手。只输出 3 到 6 个简短中文标签，用英文逗号分隔，不要解释，不要编号，每个标签不超过 8 个字。"
		context := fmt.Sprintf("标题: %s\n站点: %s\n\n正文:\n%s", b.Title, b.Site, outline)
		rawLabels, err := client.Ask(aiCtx, system, context)
		if err != nil {
			server.Log(r).Warn("ai tag", slog.Any("err", err))
			fail(server.Locale(r.Context()).Gettext("AI cleanup failed: %s", err.Error()))
			return
		}
		labels := parseAILabels(rawLabels)
		if len(labels) == 0 {
			fail(server.Locale(r.Context()).Gettext("AI did not return any labels."))
			return
		}
		for _, label := range labels {
			if !slices.Contains(b.Labels, label) {
				b.Labels = append(b.Labels, label)
			}
		}
		if err := b.Update(goqu.Record{"labels": b.Labels, "updated": time.Now().UTC()}); err != nil {
			fail(server.Locale(r.Context()).Gettext("Could not save labels."))
			return
		}
		if jsonMode {
			writeAIJSON(w, http.StatusOK, aiJSONResponse{OK: true, Labels: b.Labels})
			return
		}
		_ = server.AddFlash(w, r, "success", server.Locale(r.Context()).Gettext("Labels added: %s", strings.Join(labels, ", ")))
		server.Redirect(w, r, "/bookmarks/"+b.UID)
		return
	}

	result, err := client.CleanWithInstruction(aiCtx, b.Title, b.URL, outline, images, instruction)
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
		body = renderAIBlocks(result.Blocks, images, aiImageColumns(instruction))
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

func renderAIBlocks(blocks []ai.Block, images []ai.Image, columnArg ...int) string {
	columns := 2
	if len(columnArg) > 0 && columnArg[0] > 0 {
		columns = columnArg[0]
	}
	if columns > 4 {
		columns = 4
	}

	var b strings.Builder
	used := map[int]bool{}

	imgHTML := func(index int, caption string) string {
		if index < 0 || index >= len(images) || used[index] {
			return ""
		}
		used[index] = true
		var x strings.Builder
		x.WriteString(`<figure><img loading="lazy" src="`)
		x.WriteString(stdhtml.EscapeString(images[index].URL))
		x.WriteString(`" alt="`)
		x.WriteString(stdhtml.EscapeString(images[index].Alt))
		x.WriteString(`">`)
		if c := strings.TrimSpace(caption); c != "" {
			x.WriteString(`<figcaption>`)
			x.WriteString(stdhtml.EscapeString(c))
			x.WriteString(`</figcaption>`)
		}
		x.WriteString(`</figure>`)
		return x.String()
	}

	flushImages := func(queue []ai.Block) {
		parts := make([]string, 0, len(queue))
		for _, block := range queue {
			if x := imgHTML(block.Index, block.Caption); x != "" {
				parts = append(parts, x)
			}
		}
		if len(parts) == 0 {
			return
		}
		if columns > 1 && len(parts) > 1 {
			fmt.Fprintf(&b, `<div class="ai-image-grid" style="--ai-cols:%d">`, columns)
			for _, x := range parts {
				b.WriteString(x)
			}
			b.WriteString(`</div>`)
			return
		}
		for _, x := range parts {
			b.WriteString(x)
		}
	}

	var imageQueue []ai.Block
	for _, block := range blocks {
		text := strings.TrimSpace(block.Text)
		typ := strings.ToLower(block.Type)
		if typ == "img" || typ == "image" {
			imageQueue = append(imageQueue, block)
			continue
		}
		if len(imageQueue) > 0 {
			flushImages(imageQueue)
			imageQueue = imageQueue[:0]
		}
		switch typ {
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
		}
	}
	if len(imageQueue) > 0 {
		flushImages(imageQueue)
	}

	// The model sometimes drops pictures while shortening a post. Put every
	// omitted picture back at the end in the original order; group them too.
	var missing []ai.Block
	for i := range images {
		if !used[i] {
			missing = append(missing, ai.Block{Type: "img", Index: i})
		}
	}
	if len(missing) > 0 {
		flushImages(missing)
	}
	return b.String()
}

func aiImageColumns(instruction string) int {
	if m := regexp.MustCompile(`(?i)(?:每行|一行|每排|每列)\s*([1-4一二三四两])\s*(?:张|列|个|幅)`).FindStringSubmatch(instruction); m != nil {
		return chineseColumn(m[1])
	}
	if m := regexp.MustCompile(`(?i)([1-4])\s*(?:张|列|个|幅)\s*(?:一行|每行|一排|每排)`).FindStringSubmatch(instruction); m != nil {
		return chineseColumn(m[1])
	}
	return 2
}

func chineseColumn(s string) int {
	switch s {
	case "一":
		return 1
	case "二", "两":
		return 2
	case "三":
		return 3
	case "四":
		return 4
	}
	n, _ := strconv.Atoi(s)
	if n < 1 {
		n = 1
	}
	if n > 4 {
		n = 4
	}
	return n
}

func parseAILabels(raw string) []string {
	raw = strings.NewReplacer("，", ",", "、", ",", "\n", ",", "#", "").Replace(raw)
	seen := map[string]struct{}{}
	labels := []string{}
	for _, part := range strings.Split(raw, ",") {
		label := strings.Trim(strings.TrimSpace(part), `"'“”`)
		if label == "" || len([]rune(label)) > 20 {
			continue
		}
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		labels = append(labels, label)
		if len(labels) == 6 {
			break
		}
	}
	return labels
}
