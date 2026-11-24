// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package utils

import (
	"strings"
	"unicode"
)

// urlTerminators are the characters that end a URL when it is embedded in a
// message. They cover the punctuation used by Chinese apps when a link is
// shared together with its title (for instance "标题 | 站点 😆 token 😆
// https://example.com/a?b=c）。"). ASCII brackets are absent on purpose:
// they can legitimately appear in a path.
const urlTerminators = "（）<>\"'“”‘’【】「」『』《》，。、；：！？…—"

// ExtractURL returns the first URL found in s.
//
// People usually copy a whole chunk of text when they share a link: Xiaohongshu
// prepends "小红书（25 【标题 - 作者 | 小红书】😆 token 😆" and Zhihu puts the
// question title on the line before the answer URL. Pasting that into the
// bookmark form fails the URL validation, so the value is reduced to the link
// itself.
//
// When no scheme is found the input is returned untouched (trimmed), letting
// the regular URL validator report the error.
func ExtractURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	lower := strings.ToLower(s)
	start := -1
	for _, prefix := range []string{"https://", "http://"} {
		if i := strings.Index(lower, prefix); i >= 0 && (start < 0 || i < start) {
			start = i
		}
	}
	if start < 0 {
		return s
	}

	s = s[start:]

	// Cut at the first character that cannot belong to a URL: whitespace,
	// control characters or CJK punctuation.
	if i := strings.IndexFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r) || strings.ContainsRune(urlTerminators, r)
	}); i >= 0 {
		s = s[:i]
	}

	return trimTrailingPunctuation(s)
}

// trimTrailingPunctuation removes the punctuation that belongs to the
// surrounding sentence. Closing brackets are only dropped when they are
// unbalanced, so a Wikipedia style "…/Foo_(bar)" keeps its parenthesis.
func trimTrailingPunctuation(s string) string {
	for len(s) > 0 {
		last := s[len(s)-1]
		switch last {
		case '.', ',', ';', ':', '!', '?', '\'', '"', '>':
			s = s[:len(s)-1]
			continue
		case ')':
			if strings.Count(s, "(") >= strings.Count(s, ")") {
				return s
			}
		case ']':
			if strings.Count(s, "[") >= strings.Count(s, "]") {
				return s
			}
		case '}':
			if strings.Count(s, "{") >= strings.Count(s, "}") {
				return s
			}
		default:
			return s
		}
		s = s[:len(s)-1]
	}
	return s
}
