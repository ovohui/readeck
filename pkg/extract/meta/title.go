// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package meta

import (
	"regexp"
	"strings"
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

	return strings.TrimSpace(title)
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
