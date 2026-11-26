// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package meta

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Browsers put notification counters into document.title on sites that keep
// a message inbox, and nearly every site appends its own name. Both end up in
// the bookmark card, where three lines of chrome push the headline out of
// view:
//
//	(99+ 封私信 / 80 条消息) 神舟二十四号载人飞船瞄准11月25日8时18分发射 - 知乎
//
// becomes
//
//	神舟二十四号载人飞船瞄准11月25日8时18分发射
var (
	// rxTitleNotice matches a leading bracketed banner.
	rxTitleNotice = regexp.MustCompile(`^[\(\[（【][^\)\]）】]{1,60}[\)\]）】]\s*`)
	// rxTitleSuffix splits "headline - Site" into its two halves. The site
	// part must be a single word, which keeps "A - B 两种方案的对比" intact.
	// A hyphen needs surrounding space; the other separators do not, because
	// "标题_简书" is as common as "标题 - 简书".
	rxTitleSuffix = regexp.MustCompile(`^(.+?)(?:\s+[-–—]\s*|\s*[_|·]\s*)([^-–—|_·\s]{1,20})$`)
)

// titleNoticeWords are the words that mark a leading banner as chrome rather
// than part of the headline. A bracketed year or an abbreviation must survive.
var titleNoticeWords = []string{
	"私信", "消息", "未读", "通知", "条新", "条回复", "新动态",
	"inbox", "unread", "messages", "notifications", "new message",
}

// cleanTitle removes the notification banner a logged-in browser prepends and
// the site name a page template appends.
func cleanTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return title
	}

	if m := rxTitleNotice.FindString(title); m != "" {
		rest := strings.TrimSpace(title[len(m):])
		inner := strings.Trim(m, "()[]（）【】 \t")
		// A banner is chrome when it counts something and either names the
		// inbox or is nothing but the counter itself. "(2026)" at the start of
		// a headline is a year, not a badge, so it stays.
		if rest != "" && (containsAnyFold(inner, titleNoticeWords) || isCounterOnly(inner)) {
			title = rest
		}
	}

	// Peel up to two trailing site names (" - Zhihu", " | Example").
	for i := 0; i < 2; i++ {
		m := rxTitleSuffix.FindStringSubmatch(title)
		if m == nil {
			break
		}
		head, tail := strings.TrimSpace(m[1]), m[2]
		if head == "" || strings.ContainsAny(tail, ".。,，!！?？:：;；") {
			break
		}
		// Never let the suffix eat the headline itself.
		if len([]rune(tail))*2 > len([]rune(title)) {
			break
		}
		title = head
	}

	// Templates glue more than one site name on, and some of those names have
	// spaces or a second brand ("_哔哩哔哩_bilibili",
	// "- OSCHINA - 开源 × AI · 开发者生态社区", "_财新网_财新网"). Cut at the last
	// known site name when it sits behind a separator.
	for i := 0; i < 3; i++ {
		head, ok := cutAtKnownSite(title)
		if !ok {
			break
		}
		title = head
	}

	title = stripTitleTags(title)

	return strings.TrimSpace(title)
}

// cutAtKnownSite cuts everything from the last known site name, provided it is
// preceded by a separator and what follows is short enough to be a suffix.
func cutAtKnownSite(title string) (string, bool) {
	lower := strings.ToLower(title)
	best := -1
	for label := range siteLabels {
		idx := strings.LastIndex(lower, strings.ToLower(label))
		if idx <= 0 || idx <= best {
			continue
		}
		prev, _ := utf8.DecodeLastRuneInString(title[:idx])
		after := title[idx+len(label):]
		if !isTitleSeparator(prev) {
			continue
		}
		if after != "" {
			next, _ := utf8.DecodeRuneInString(after)
			if !isTitleSeparator(next) {
				continue
			}
		}
		if len([]rune(after)) > 48 {
			continue
		}
		best = idx
	}
	if best <= 0 {
		return title, false
	}
	return strings.TrimRightFunc(strings.TrimSpace(title[:best]), isTitleSeparator), true
}

// stripTitleTags removes a trailing tag list: "…的决定|总书记|中央纪委".
func stripTitleTags(title string) string {
	parts := strings.Split(title, "|")
	if len(parts) < 2 {
		return title
	}
	end := len(parts)
	for end > 1 && len(parts)-end < 4 {
		tag := strings.TrimSpace(parts[end-1])
		if tag == "" || len([]rune(tag)) > 8 || strings.ContainsAny(tag, ".。,，!！?？:：;；、()（）") {
			break
		}
		end--
	}
	if len(parts)-end == 0 {
		return title
	}
	// A single trailing tag is only removed when what remains is a real
	// headline: "设计|界面" must not become "设计".
	if len(parts)-end == 1 && len([]rune(strings.Join(parts[:end], "|"))) < 10 {
		return title
	}
	return strings.TrimSpace(strings.Join(parts[:end], "|"))
}

func isTitleSeparator(r rune) bool {
	switch r {
	case ' ', '\t', '_', '|', '｜', '-', '–', '—', '·', ':', '：':
		return true
	}
	return false
}

// isCounterOnly reports whether s is just a badge count: "3", "99+", "1.2k",
// "3 万". Four digits or more is a year or an identifier, never a badge.
func isCounterOnly(s string) bool {
	digits := 0
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '+' || r == ' ' || r == '/' || r == ',' || r == '.' ||
			r == 'k' || r == 'K' || r == '百' || r == '千' || r == '万':
			// allowed around a count
		default:
			return false
		}
	}
	return digits > 0 && digits <= 3
}

// containsAnyFold reports whether s contains any of the given substrings,
// ignoring case.
func containsAnyFold(s string, subs []string) bool {
	lower := strings.ToLower(s)
	for _, sub := range subs {
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}
