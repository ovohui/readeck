// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractURL(t *testing.T) {
	assert := assert.New(t)

	tests := []struct{ in, out string }{
		{
			"小红书（25 【新时代三幻神，不接受反驳😮😮😮 - 晚风叙 | 小红书 - 你的生活兴趣社区】 😆 eqvXifjJzdarcBk 😆 https://www.xiaohongshu.com/discovery/item/6b0418fc000000001601ae6b?source=webshare&xhsshare=pc_web&xsec_token=ABtxdxBlFwR6dfH-N6rAy5JC6u1r_wNYhA3cozyXM4STk=&xsec_source=pc_share）",
			"https://www.xiaohongshu.com/discovery/item/6b0418fc000000001601ae6b?source=webshare&xhsshare=pc_web&xsec_token=ABtxdxBlFwR6dfH-N6rAy5JC6u1r_wNYhA3cozyXM4STk=&xsec_source=pc_share",
		},
		{
			"如果我把以色列的农业滴灌技术引进，使得农业产粮全面翻倍，足养20亿人，国家奖励100万少吗？ - 战上海的回答 - 知乎\nhttps://www.zhihu.com/question/2042206394632368178/answer/2093325041043879103",
			"https://www.zhihu.com/question/2042206394632368178/answer/2093325041043879103",
		},
		{"https://example.org/path?a=1", "https://example.org/path?a=1"},
		{"https://example.org/path).", "https://example.org/path"},
		{"https://en.wikipedia.org/wiki/Foo_(bar)", "https://en.wikipedia.org/wiki/Foo_(bar)"},
		{"[标题](https://example.org/a/b)", "https://example.org/a/b"},
		{"  https://example.org/x  ", "https://example.org/x"},
		{"no url here", "no url here"},
	}

	for _, test := range tests {
		assert.Equal(test.out, ExtractURL(test.in), "input %q", test.in)
	}
}
