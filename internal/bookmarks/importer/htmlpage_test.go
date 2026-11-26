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

func TestHTMLPageTurnsAttachmentLinksIntoPictures(t *testing.T) {
	page := `<!DOCTYPE html><html><head><title>帖子</title></head><body>
		<div id="postmessage_1">
			<p>文字</p>
			<p><a href="https://example.com/a.gif">下载附件</a></p>
			<p><a href="https://example.com/b.pdf">下载附件</a></p>
		</div>
	</body></html>`

	adapter := LoadAdapter(ImportHTMLPage)
	f, items := loadFile(t, dataFile(page), adapter)
	require.True(t, f.IsValid())
	require.Len(t, items, 1)

	res := items[0].(BookmarkResourceProvider).Resources()
	require.Len(t, res, 1)
	out := string(res[0].Data)
	assert.Contains(t, out, `<img src="https://example.com/a.gif"`)
	assert.NotContains(t, out, `href="https://example.com/a.gif"`)
	// A PDF attachment is not a picture and must stay a link.
	assert.Contains(t, out, `href="https://example.com/b.pdf"`)
}

func TestHTMLPageKeepsInlineImageOverRemoteCopy(t *testing.T) {
	payload := strings.Repeat("A", 200)
	page := `<!DOCTYPE html><html><head><title>帖子</title></head><body>
		<div id="postmessage_1">
			<img src="data:image/gif;base64,` + payload + `" file="https://cdn.example.com/a.gif" width="200">
		</div>
	</body></html>`

	adapter := LoadAdapter(ImportHTMLPage)
	f, items := loadFile(t, dataFile(page), adapter)
	require.True(t, f.IsValid())

	res := items[0].(BookmarkResourceProvider).Resources()
	require.Len(t, res, 1)
	out := string(res[0].Data)
	assert.Contains(t, out, "data:image/gif;base64,"+payload)
	assert.NotContains(t, out, `file="https://cdn.example.com/a.gif"`)
}

func TestHTMLPageDoesNotDuplicateInlineAttachment(t *testing.T) {
	payload := strings.Repeat("A", 200)
	page := `<!DOCTYPE html><html><head><title>帖子</title></head><body>
		<div id="postmessage_1">
			<img src="data:image/gif;base64,` + payload + `" file="https://example.com/a.gif" width="200">
			<p><a href="https://example.com/a.gif">下载附件</a></p>
		</div>
	</body></html>`

	adapter := LoadAdapter(ImportHTMLPage)
	f, items := loadFile(t, dataFile(page), adapter)
	require.True(t, f.IsValid())

	res := items[0].(BookmarkResourceProvider).Resources()
	out := string(res[0].Data)
	assert.Equal(t, 1, strings.Count(out, "<img"))
	assert.NotContains(t, out, "<figure>")
	assert.NotContains(t, out, `file="https://example.com/a.gif"`)
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
