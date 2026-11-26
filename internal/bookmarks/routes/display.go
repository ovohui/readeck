// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"codeberg.org/readeck/readeck/internal/bookmarks/dataset"
)

// The reading page shows two fields that come straight from the page and are
// therefore as noisy as the page wants them to be: the title (usually with the
// site name glued on) and the description (often a keyword dump, or - on video
// pages - the description followed by view counts and the titles of every
// related video). Both are cleaned for display only: the stored bookmark keeps
// what the extractor found, so nothing is lost and an edit in the UI still
// shows exactly what was typed.

// titleSiteLabels are the site names that pages append to document.title.
// They are matched case-insensitively after the last separator.
var titleSiteLabels = map[string]bool{
	// Chinese sites
	"哔哩哔哩": true, "b站": true, "知乎": true, "知乎专栏": true, "知乎回答": true,
	"小红书": true, "微博": true, "微信公众平台": true, "微信公众号": true,
	"什么值得买": true, "财新网": true, "界面新闻": true, "界面": true, "快讯": true,
	"网易": true, "网易政务": true, "网易新闻": true, "澎湃新闻": true, "澎湃": true,
	"中国政库": true, "虎扑": true, "虎扑社区": true, "豆瓣": true, "豆瓣小组": true,
	"掘金": true, "博客园": true, "腾讯新闻": true, "搜狐": true, "新浪": true,
	"凤凰网": true, "央视网": true, "人民网": true, "新华网": true, "观察者网": true,
	"今日头条": true, "头条": true, "抖音": true, "快手": true, "简书": true,
	"少数派": true, "开源中国": true, "36氪": true, "IT之家": true, "爱范儿": true,
	"百度": true, "京东": true, "淘宝": true,
	// ASCII site names that really do appear as a suffix
	"bilibili": true, "zhihu": true, "xiaohongshu": true, "weibo": true,
	"smzdm": true, "caixin": true, "thepaper": true, "the paper": true,
	"oschina": true, "ithome": true, "sspai": true, "infoq": true,
	"cnblogs": true, "juejin": true, "douban": true, "hupu": true,
	"36kr": true, "ifanr": true, "v2ex": true, "github": true, "youtube": true,
}

// displayTitle returns the title without the site name a page template glued
// to it and without the trailing tag list news sites append.
func displayTitle(item *dataset.Bookmark) string {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		return title
	}

	// The bookmark's own site name is the most reliable suffix. Cutting at its
	// last occurrence also removes whatever the template put after it
	// ("_哔哩哔哩_bilibili", " - OSCHINA - 开源 × AI · ...").
	for _, name := range siteNameCandidates(item) {
		if cut, ok := cutAtSiteName(title, name); ok {
			title = cut
		}
	}

	title = stripKnownTitleTail(title)
	title = stripTitleTags(title)

	return strings.TrimSpace(title)
}

// siteNameCandidates lists the strings that can appear as the site part of a
// title: the stored site name, its first segment and the domain's second
// level label.
func siteNameCandidates(item *dataset.Bookmark) []string {
	out := []string{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || len([]rune(s)) > 40 {
			return
		}
		for _, x := range out {
			if strings.EqualFold(x, s) {
				return
			}
		}
		out = append(out, s)
	}

	add(item.SiteName)
	if i := strings.IndexAny(item.SiteName, "-|·"); i > 0 {
		add(item.SiteName[:i])
	}
	if host := item.Site; host != "" {
		parts := strings.Split(host, ".")
		if len(parts) >= 2 {
			add(parts[len(parts)-2])
		}
	}

	return out
}

// cutAtSiteName cuts everything from the last occurrence of name, provided it
// starts at a separator and what follows is short enough to be a site suffix
// rather than the rest of the headline.
func cutAtSiteName(title, name string) (string, bool) {
	lowerTitle := strings.ToLower(title)
	lowerName := strings.ToLower(name)
	idx := strings.LastIndex(lowerTitle, lowerName)
	if idx <= 0 {
		return title, false
	}

	before := title[:idx]
	after := title[idx+len(name):]
	prev, _ := utf8.DecodeLastRuneInString(before)
	if !isTitleSeparator(prev) {
		return title, false
	}

	// " - OSCHINA - 开源 × AI · ..." leaves more than the site name behind;
	// that is still a suffix as long as it is short. A headline that merely
	// mentions the site keeps a long tail and is left untouched.
	if len([]rune(after)) > 48 {
		return title, false
	}

	return strings.TrimRightFunc(strings.TrimSpace(before), func(r rune) bool {
		return isTitleSeparator(r)
	}), true
}

// stripKnownTitleTail peels up to three trailing site names, one separator at
// a time: "标题_哔哩哔哩_bilibili" -> "标题", "标题 - 少数派" -> "标题".
func stripKnownTitleTail(title string) string {
	for range 3 {
		idx, size := lastTitleSeparator(title)
		if idx < 0 {
			break
		}
		tail := strings.TrimSpace(title[idx+size:])
		if !isKnownTitleTail(tail) {
			break
		}
		title = strings.TrimRightFunc(strings.TrimSpace(title[:idx]), isTitleSeparator)
	}
	return title
}

// stripTitleTags removes the tag list news sites put after the headline:
// "…的决定|总书记|中央纪委". Only a run of at least two short, punctuation-free
// segments is removed, so "A | B" style titles survive.
func stripTitleTags(title string) string {
	sep := "|"
	if strings.Count(title, "｜") > strings.Count(title, "|") {
		sep = "｜"
	}
	parts := strings.Split(title, sep)
	if len(parts) < 2 {
		return title
	}

	end := len(parts)
	for end > 1 && len(parts)-end < 4 {
		tag := strings.TrimSpace(parts[end-1])
		if tag == "" || len([]rune(tag)) > 8 || strings.ContainsAny(tag, "。！？：；，、,.!?:;()（）") {
			break
		}
		end--
	}

	if len(parts)-end == 0 {
		return title
	}
	// A single trailing tag is only removed when a real headline remains.
	if len(parts)-end == 1 && len([]rune(strings.Join(parts[:end], sep))) < 10 {
		return title
	}
	return strings.TrimSpace(strings.Join(parts[:end], sep))
}

func lastTitleSeparator(s string) (int, int) {
	best, size := -1, 0
	for _, sep := range []string{"|", "｜", "_", " · ", " - ", " – ", " — ", "-", "·"} {
		if i := strings.LastIndex(s, sep); i > best {
			best, size = i, len(sep)
		}
	}
	return best, size
}

func isKnownTitleTail(tail string) bool {
	tail = strings.TrimSpace(tail)
	if tail == "" || len([]rune(tail)) > 20 {
		return false
	}
	if strings.ContainsAny(tail, "。！？：；，、,.!?:;()（）【】") {
		return false
	}
	if titleSiteLabels[strings.ToLower(tail)] {
		return true
	}
	// "The Paper", "OSCHINA", "36Kr": a short ASCII tail is a site name too.
	for _, r := range tail {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return len(strings.Fields(tail)) <= 3
}

func isTitleSeparator(r rune) bool {
	switch r {
	case ' ', '\t', '_', '|', '｜', '-', '–', '—', '·', ':', '：':
		return true
	}
	return false
}

// videoDescriptionMarkers start the boilerplate Bilibili (and friends) append
// to the real description: view counts, uploader, related videos.
var videoDescriptionMarkers = []string{
	"视频播放量", "播放量 ", "弹幕量", "视频作者", "作者简介", "相关视频：", "相关视频:",
}

// displayDescription cleans the summary and drops it when it is nothing but a
// copy of the title or a keyword list.
func displayDescription(item *dataset.Bookmark) string {
	desc := normalizeText(item.Description)
	if desc == "" || (item.OmitDescription != nil && *item.OmitDescription) {
		return ""
	}

	desc = stripVideoDescriptionNoise(desc)
	if desc == "" || len([]rune(desc)) < 8 {
		return ""
	}

	title := displayTitle(item)
	nt := compareText(title)
	nd := compareText(desc)

	// The description repeats the title, sometimes with a comma-separated
	// keyword list glued on ("标题,陈吉宁,党中央,总书记,…").
	if nt != "" && nd == nt {
		return ""
	}
	if nt != "" && strings.HasPrefix(nd, nt) {
		rest := nd[len(nt):]
		if !strings.ContainsAny(rest, "。！？.!?") {
			return ""
		}
	}

	return truncateText(desc, 280)
}

// stripVideoDescriptionNoise cuts the statistics block video sites append.
func stripVideoDescriptionNoise(desc string) string {
	cut := -1
	for _, marker := range videoDescriptionMarkers {
		if i := strings.Index(desc, marker); i > 20 && (cut < 0 || i < cut) {
			cut = i
		}
	}
	if cut > 0 {
		desc = desc[:cut]
	}
	return strings.TrimRightFunc(strings.TrimSpace(desc), func(r rune) bool {
		return strings.ContainsRune(" ,，、;；:：-–—", r)
	})
}

// normalizeText collapses every whitespace run to a single space and trims.
func normalizeText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// compareText lowercases and drops spaces and punctuation so two copies of the
// same sentence compare equal even when one adds a comma or a full stop.
func compareText(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// authorLine keeps the kicker to one line when a page lists a whole editorial
// team.
func authorLine(item *dataset.Bookmark) string {
	authors := item.Authors
	if len(authors) == 0 {
		return ""
	}
	if len(authors) <= 2 {
		return strings.Join(authors, ", ")
	}
	return strings.Join(authors[:2], ", ") + " +" + strconv.Itoa(len(authors)-2)
}

// truncateText cuts at a sentence boundary when one is close enough and adds
// an ellipsis, counting runes rather than bytes.
func truncateText(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	cut := limit
	for i := limit; i > limit-60 && i > 0; i-- {
		switch runes[i-1] {
		case '。', '！', '？', '.', '!', '?', '；', ';':
			cut = i
			i = 0
		}
	}
	return strings.TrimSpace(string(runes[:cut])) + "…"
}
