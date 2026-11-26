// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	stdctx "context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"

	"codeberg.org/readeck/readeck/internal/ai"
	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/server"
)

type aiListItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Site        string `json:"site"`
	Domain      string `json:"domain,omitempty"`
	Description string `json:"description"`
	Text        string `json:"text"`
}

type aiListResponse struct {
	OK         bool         `json:"ok"`
	Error      string       `json:"error,omitempty"`
	Answer     string       `json:"answer,omitempty"`
	Labels     []string     `json:"labels,omitempty"`
	Matches    []aiListItem `json:"matches,omitempty"`
	FilterURL  string       `json:"filter_url,omitempty"`
	AutoFilter bool         `json:"auto_filter,omitempty"`
}

func writeAIList(w http.ResponseWriter, status int, res aiListResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(res)
}

// aiList answers questions about the current selection or the current filtered
// list, and can add labels to the selected bookmarks.
func (h *viewsRouter) aiList(w http.ResponseWriter, r *http.Request) {
	if !aiEnabled(r.Context()) {
		writeAIList(w, http.StatusBadRequest, aiListResponse{Error: "AI is not configured."})
		return
	}

	action := strings.ToLower(strings.TrimSpace(r.FormValue("action")))
	if action == "" {
		action = "ask"
	}
	question := strings.TrimSpace(r.FormValue("question"))
	if action == "ask" && question == "" {
		writeAIList(w, http.StatusBadRequest, aiListResponse{Error: "请先输入问题。"})
		return
	}

	items := aiListItems(r)
	if len(items) == 0 {
		writeAIList(w, http.StatusBadRequest, aiListResponse{Error: "当前列表没有可用的书签。"})
		return
	}
	if len(items) > 25 {
		items = items[:25]
	}

	var context strings.Builder
	for _, item := range items {
		text := []rune(strings.TrimSpace(item.Text))
		if len(text) > 700 {
			text = text[:700]
		}
		context.WriteString("ID: ")
		context.WriteString(item.ID)
		context.WriteString("\n标题: ")
		context.WriteString(item.Title)
		context.WriteString("\n站点: ")
		context.WriteString(item.Site)
		context.WriteString("\n摘要: ")
		context.WriteString(item.Description)
		context.WriteString("\n正文: ")
		context.WriteString(string(text))
		context.WriteString("\n---\n")
	}

	system := `你是一个中文阅读助手。只根据用户给出的收藏列表回答，不要编造。
返回 JSON 对象，不要 markdown 代码块：
{"answer":"回答/摘要","ids":["与问题最相关的书签ID"]}
ids 只能从列表里的 ID 选择；不确定时返回空数组。如果用户只是让总结，把总结放在 answer；如果让打标签，改用 {"labels":["标签"]}。`

	var user strings.Builder
	user.WriteString("收藏列表:\n")
	user.WriteString(context.String())
	user.WriteString("\n用户要求: ")
	if action == "summary" {
		user.WriteString("用 3-6 句话总结这些收藏的主要主题和结论。")
	} else if action == "tag" {
		user.WriteString("为这些选中的收藏生成 2-5 个中文标签。只返回 {\"labels\":[...]}。")
	} else {
		user.WriteString(question)
	}

	client := ai.New(aiConfig(r.Context()))
	aiCtx, cancel := stdctx.WithTimeout(stdctx.Background(), 150*time.Second)
	defer cancel()
	answer, err := client.Ask(aiCtx, system, user.String())
	if err != nil {
		server.Log(r).Warn("ai list", "err", err)
		writeAIList(w, http.StatusBadGateway, aiListResponse{Error: err.Error()})
		return
	}

	var parsed struct {
		Answer string   `json:"answer"`
		IDs    []string `json:"ids"`
		Labels []string `json:"labels"`
	}
	if i := strings.Index(answer, "{"); i >= 0 {
		answer = answer[i:]
	}
	if i := strings.LastIndex(answer, "}"); i >= 0 {
		answer = answer[:i+1]
	}
	if err := json.Unmarshal([]byte(answer), &parsed); err != nil {
		parsed.Answer = strings.TrimSpace(answer)
	}

	if action == "tag" {
		if len(parsed.Labels) == 0 {
			writeAIList(w, http.StatusBadGateway, aiListResponse{Error: "AI 没有返回标签。"})
			return
		}
		if len(parsed.Labels) > 8 {
			parsed.Labels = parsed.Labels[:8]
		}
		for _, item := range items {
			b, err := bookmarks.Bookmarks.GetOne(
				goqu.C("uid").Eq(item.ID),
				goqu.C("user_id").Eq(auth.GetUser(r.Context()).ID),
			)
			if err != nil {
				continue
			}
			for _, label := range parsed.Labels {
				label = strings.TrimSpace(label)
				if label != "" && len([]rune(label)) <= 48 {
					b.Labels = append(b.Labels, label)
				}
			}
			b.Labels = dedupeLabels(b.Labels)
			_ = b.Update(goqu.Record{"labels": b.Labels, "updated": time.Now().UTC()})
		}
		writeAIList(w, http.StatusOK, aiListResponse{OK: true, Labels: parsed.Labels})
		return
	}

	var matches []aiListItem
	seen := map[string]bool{}
	for _, id := range parsed.IDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		for _, item := range items {
			if item.ID == id {
				matches = append(matches, item)
				break
			}
		}
	}
	if parsed.Answer == "" {
		parsed.Answer = "AI 没有返回内容。"
	}
	filterURL := ""
	autoFilter := false
	if len(matches) > 0 {
		domain := matches[0].Domain
		sameDomain := domain != ""
		for _, m := range matches {
			if m.Domain != domain {
				sameDomain = false
				break
			}
		}
		if sameDomain {
			q := strings.ToLower(question)
			if strings.Contains(q, "筛选") || strings.Contains(q, "只看") || strings.Contains(q, "过滤") || strings.Contains(q, "展示") || strings.Contains(q, "列出") || strings.Contains(q, "来自") {
				filterURL = "/bookmarks?bf=1&site=" + url.QueryEscape(domain)
				autoFilter = true
			}
		}
	}
	writeAIList(w, http.StatusOK, aiListResponse{OK: true, Answer: parsed.Answer, Matches: matches, FilterURL: filterURL, AutoFilter: autoFilter})
}

func aiListItems(r *http.Request) []aiListItem {
	userID := auth.GetUser(r.Context()).ID
	if raw := strings.TrimSpace(r.FormValue("ids")); raw != "" {
		ids := strings.Split(raw, ",")
		res := []aiListItem{}
		bs := []*bookmarks.Bookmark{}
		_ = bookmarks.Bookmarks.Query().
			Where(goqu.C("user_id").Table("b").Eq(userID), goqu.C("uid").Table("b").In(ids)).
			ScanStructs(&bs)
		for _, b := range bs {
			if isFolderBlocked(r, b.FolderID) {
				continue
			}
			res = append(res, bookmarkToAIItem(b))
		}
		return res
	}

	bl := getBookmarkList(r.Context())
	if bl == nil {
		return nil
	}
	res := make([]aiListItem, 0, len(bl.Items))
	for _, b := range bl.Items {
		res = append(res, bookmarkToAIItem(b.Bookmark))
	}
	return res
}

func bookmarkToAIItem(b *bookmarks.Bookmark) aiListItem {
	return aiListItem{
		ID:          b.UID,
		Title:       b.Title,
		Site:        b.SiteName,
		Domain:      b.Domain,
		Description: b.Description,
		Text:        b.Text,
	}
}

func dedupeLabels(labels []string) []string {
	seen := map[string]bool{}
	res := labels[:0]
	for _, label := range labels {
		if label == "" || seen[label] {
			continue
		}
		seen[label] = true
		res = append(res, label)
	}
	return res
}
