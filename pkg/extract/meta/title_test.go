// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package meta

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanTitle(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  string
	}{
		{
			"zhihu inbox banner and site suffix",
			"(99+ 封私信 / 80 条消息) 神舟二十四号载人飞船瞄准11月25日8时18分发射 - 知乎",
			"神舟二十四号载人飞船瞄准11月25日8时18分发射",
		},
		{
			"plain site suffix",
			"如何评价这个方案 - 知乎",
			"如何评价这个方案",
		},
		{
			"pipe suffix",
			"标题 | 少数派",
			"标题",
		},
		{
			"underscore suffix",
			"标题_简书",
			"标题",
		},
		{
			"a leading year is part of the headline",
			"(2026) 年度总结",
			"(2026) 年度总结",
		},
		{
			"a dash inside the headline stays",
			"A - B 两种方案的对比",
			"A - B 两种方案的对比",
		},
		{
			"hyphen without space is not a separator",
			"state-of-the-art 综述",
			"state-of-the-art 综述",
		},
		{
			"site name must not eat the headline",
			"- 知乎",
			"- 知乎",
		},
		{
			"empty title",
			"",
			"",
		},
		{
			"leading and trailing space",
			"  标题 - 知乎  ",
			"标题",
		},
		{
			"bare counter badge",
			"(3) Example News - Example",
			"Example News",
		},
		{
			"counter with a word",
			"(12 Unread) Example News",
			"Example News",
		},
		{
			"four digits is a year, not a badge",
			"(2026) Example News",
			"(2026) Example News",
		},
		{
			"bilibili glues two site names",
			"一个宝藏开源项目，它干的事就是把动效整理成名字。_哔哩哔哩_bilibili",
			"一个宝藏开源项目，它干的事就是把动效整理成名字。",
		},
		{
			"repeated caixin suffix",
			"华为AI专利超8000件 2025年专利许可收入8亿美元_财新网_财新网",
			"华为AI专利超8000件 2025年专利许可收入8亿美元",
		},
		{
			"tagline after a dash",
			"🔥 xbatis 1.10.13 正式发版 - OSCHINA - 开源 × AI · 开发者生态社区",
			"🔥 xbatis 1.10.13 正式发版",
		},
		{
			"news tags before the site name",
			"上海市委常委会举行扩大会议的决定|总书记|中央纪委_网易政务",
			"上海市委常委会举行扩大会议的决定",
		},
		{
			"the paper suffix with an english brand",
			"广西：打击非法猎捕专项行动_中国政库_澎湃新闻-The Paper",
			"广西：打击非法猎捕专项行动",
		},
		{
			"hyphenated english words are not site names",
			"state-of-the-art 综述",
			"state-of-the-art 综述",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.out, cleanTitle(tt.in))
		})
	}
}
