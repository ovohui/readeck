// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package meta

import (
	"strings"
)

// videoDescriptionMarkers start the statistics block Bilibili appends to the
// real description:
//
//	视频中出现的工具 已整理至…, 视频播放量 62、弹幕量 0、点赞数 1、…, 视频作者 创心宝库,
//	作者简介 …，相关视频：…
//
// On a bookmark card that is 550 characters of noise behind 100 characters of
// actual description. The stored value stops at the first marker.
var videoDescriptionMarkers = []string{
	"视频播放量", "弹幕量", "视频作者", "作者简介", "相关视频：", "相关视频:",
}

// descriptionLimit is the maximum number of runes kept in a description.
// Counting words (the old behaviour) does nothing for Chinese, where a whole
// paragraph is one whitespace-delimited field.
const descriptionLimit = 300

// cleanDescription trims the video statistics block and caps the result at a
// readable length, at a sentence boundary when possible.
func cleanDescription(desc string) string {
	desc = strings.Join(strings.Fields(desc), " ")
	if desc == "" {
		return desc
	}

	cut := -1
	for _, marker := range videoDescriptionMarkers {
		if i := strings.Index(desc, marker); i > 20 && (cut < 0 || i < cut) {
			cut = i
		}
	}
	if cut > 0 {
		desc = desc[:cut]
	}
	desc = strings.TrimRightFunc(strings.TrimSpace(desc), func(r rune) bool {
		return strings.ContainsRune(" ,，、;；:：-–—", r)
	})

	return truncateDescription(desc, descriptionLimit)
}

func truncateDescription(s string, limit int) string {
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
