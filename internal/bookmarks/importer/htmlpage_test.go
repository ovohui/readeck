// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package importer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTMLPageSyntheticURL(t *testing.T) {
	page := `<!DOCTYPE html><html lang="zh-CN"><head>
		<title>本地保存的网页 - 示例站</title>
		<meta name="description" content="一段简介">
		<base href="/">
		<link rel="canonical" href="/thread-1-1.html">
	</head><body><p>正文</p></body></html>`

	adapter := LoadAdapter(ImportHTMLPage)
	f, items := loadFile(t, dataFile(page), adapter)
	require.True(t, f.IsValid())
	require.Len(t, items, 1)

	item := items[0]
	assert.Equal(t, "https://import.local/data", item.URL())

	meta, err := item.(BookmarkEnhancer).Meta()
	require.NoError(t, err)
	assert.Equal(t, "本地保存的网页 - 示例站", meta.Title)
	assert.Equal(t, "一段简介", meta.Description)
	assert.Equal(t, "zh", meta.Lang)

	res := item.(BookmarkResourceProvider).Resources()
	require.Len(t, res, 1)
	assert.Equal(t, item.URL(), res[0].URL)
	assert.Contains(t, string(res[0].Data), "正文")
}

func TestHTMLPageTrimsSiteChrome(t *testing.T) {
	page := `<!DOCTYPE html><html><head><title>帖子 - Powered by Discuz!</title></head><body>
		<div id="header"><span id="thread_subject">帖子标题</span> 网站导航</div>
		<table id="postlist"><tr><td class="t_f">帖子正文</td></tr></table>
		<div id="footer">页脚</div>
	</body></html>`

	adapter := LoadAdapter(ImportHTMLPage)
	f, items := loadFile(t, dataFile(page), adapter)
	require.True(t, f.IsValid())
	require.Len(t, items, 1)

	meta, err := items[0].(BookmarkEnhancer).Meta()
	require.NoError(t, err)
	assert.Equal(t, "帖子标题", meta.Title)

	res := items[0].(BookmarkResourceProvider).Resources()
	require.Len(t, res, 1)
	assert.Contains(t, string(res[0].Data), "帖子正文")
	assert.NotContains(t, string(res[0].Data), "网站导航")
	assert.NotContains(t, string(res[0].Data), "页脚")
}

func TestHTMLPageCanonicalAndRelativePictures(t *testing.T) {
	page := `<!DOCTYPE html><html><head>
		<title>标题</title>
		<link rel="canonical" href="https://example.com/posts/1">
	</head><body>
		<img src="/images/a.png"><img src="data:image/gif;base64,R0lGODlhAQABAAAAACw=">
	</body></html>`

	adapter := LoadAdapter(ImportHTMLPage)
	f, items := loadFile(t, dataFile(page), adapter)
	require.True(t, f.IsValid())
	require.Len(t, items, 1)

	item := items[0]
	assert.Equal(t, "https://example.com/posts/1", item.URL())

	res := item.(BookmarkResourceProvider).Resources()
	require.Len(t, res, 1)
	assert.Contains(t, string(res[0].Data), `src="https://example.com/images/a.png"`)
	assert.True(t, strings.Contains(string(res[0].Data), "data:image/gif;base64"))
}
