// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/bookmarks/dataset"
)

func displayItem(title, desc, siteName, site, typ string) *dataset.Bookmark {
	return &dataset.Bookmark{
		Bookmark: &bookmarks.Bookmark{
			Title:        title,
			Description:  desc,
			SiteName:     siteName,
			Site:         site,
			DocumentType: typ,
		},
		Title:       title,
		Description: desc,
		SiteName:    siteName,
		Site:        site,
		Type:        typ,
		Resources:   map[string]*dataset.BookmarkFile{},
	}
}

func TestDisplayTitle(t *testing.T) {
	tests := []struct {
		name string
		item *dataset.Bookmark
		want string
	}{
		{
			"bilibili suffix",
			displayItem("一个宝藏开源项目，它干的事就是把动效整理成名字。_哔哩哔哩_bilibili", "", "哔哩哔哩", "www.bilibili.com", "video"),
			"一个宝藏开源项目，它干的事就是把动效整理成名字。",
		},
		{
			"oschina tagline",
			displayItem("🔥 xbatis 1.10.13 正式发版 - OSCHINA - 开源 × AI · 开发者生态社区", "", "OSCHINA", "www.oschina.net", "article"),
			"🔥 xbatis 1.10.13 正式发版",
		},
		{
			"news tag list",
			displayItem("上海市委常委会举行扩大会议的决定|总书记|中央纪委_网易政务", "", "www.163.com", "www.163.com", "article"),
			"上海市委常委会举行扩大会议的决定",
		},
		{
			"caixin double suffix",
			displayItem("华为AI专利超8000件 2025年专利许可收入8亿美元_财新网_财新网", "", "www.caixin.com", "www.caixin.com", "article"),
			"华为AI专利超8000件 2025年专利许可收入8亿美元",
		},
		{
			"plain title is untouched",
			displayItem("海信85U7T Pro测评，真彩黑曜屏的实力", "", "知乎专栏", "zhuanlan.zhihu.com", "article"),
			"海信85U7T Pro测评，真彩黑曜屏的实力",
		},
		{
			"short title is not eaten",
			displayItem("知乎", "", "知乎", "www.zhihu.com", "article"),
			"知乎",
		},
		{
			"english subtitle is not a site name",
			displayItem("A - Hello World", "", "Example", "example.com", "article"),
			"A - Hello World",
		},
		{
			"numeric domain label inside the headline is left alone",
			displayItem("Windows 163 个新功能 - 示例", "", "www.163.com", "news.163.com", "article"),
			"Windows 163 个新功能 - 示例",
		},
		{
			"cms footer is removed",
			displayItem("我的帖子 - 原创区 - 98堂 - Powered by Discuz!", "", "import.local", "import.local", "article"),
			"我的帖子 - 原创区 - 98堂",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, displayTitle(tt.item))
		})
	}
}

func TestDisplayDescription(t *testing.T) {
	bili := "视频中出现的工具 已整理至👇下载地址：https://example.com/x 喜欢的点个三连呗，谢谢啦~, " +
		"视频播放量 62、弹幕量 0、点赞数 1、投硬币枚数 0、收藏人数 8、转发人数 0, " +
		"视频作者 创心宝库, 作者简介 资源在简介或评论区自取～，相关视频：别的视频"
	wantBili := "视频中出现的工具 已整理至👇下载地址：https://example.com/x 喜欢的点个三连呗，谢谢啦~"

	t.Run("video statistics are cut", func(t *testing.T) {
		item := displayItem("标题", bili, "哔哩哔哩", "www.bilibili.com", "video")
		assert.Equal(t, wantBili, displayDescription(item))
	})

	t.Run("description equal to the title is dropped", func(t *testing.T) {
		item := displayItem("【数据看盘】沪深300ETF成交额激增", "【数据看盘】沪深300ETF成交额激增", "财联社", "www.cls.cn", "article")
		assert.Equal(t, "", displayDescription(item))
	})

	t.Run("keyword list after the title is dropped", func(t *testing.T) {
		item := displayItem(
			"上海市委常委会举行扩大会议的决定",
			"上海市委常委会举行扩大会议的决定,陈吉宁,党中央,习近平,总书记,中央纪委,常委会,上海市委",
			"www.163.com", "www.163.com", "article",
		)
		assert.Equal(t, "", displayDescription(item))
	})

	t.Run("long description is truncated", func(t *testing.T) {
		item := displayItem("标题", strings.Repeat("这是一段摘要。", 60), "站点", "example.com", "article")
		out := displayDescription(item)
		assert.LessOrEqual(t, len([]rune(out)), 281)
		assert.True(t, strings.HasSuffix(out, "…"))
	})

	t.Run("omitted description stays omitted", func(t *testing.T) {
		item := displayItem("标题", "一段正常的摘要，不应该显示。", "站点", "example.com", "article")
		omit := true
		item.OmitDescription = &omit
		assert.Equal(t, "", displayDescription(item))
	})
}
