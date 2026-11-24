// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package contentscripts

import (
	"testing"

	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/extract"
	"codeberg.org/readeck/readeck/pkg/extract/contents"
)

// The browser extension does not scan the page again: it posts the note it
// built from the page state, with the note URL left on the cover so the
// archiver can download the clip. This is that document, verbatim, run through
// the extraction pipeline the bookmark goes through. The placeholder must
// survive readability and bleach (which strips data- attributes), otherwise
// the extension saves a note with a picture where the video should be.
func TestExtensionNotePipeline(t *testing.T) {
	assert := require.New(t)

	const pageURL = "https://www.xiaohongshu.com/explore/6afabf31000000001803b7b0?xsec_token=ABWOyVMA"
	const doc = `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>决战美鞋之巅2.0（保暖靴子篇）</title></head><body>` +
		`<article class="xhs-note"><h1>决战美鞋之巅2.0（保暖靴子篇）</h1>` +
		`<p class="xhs-meta">作者：证人筠筠 · 2026-11-23 15:51 · 江苏</p>` +
		`<p>还有我的联名靴哦哦哦！</p><p>这期更是精彩吧？</p>` +
		`<p>#鞋控の日常 #通勤穿搭 #鞋子 #靴子 #繁荣主义 #繁荣美学 #买手直播预告</p>` +
		`<figure><img src="https://sns-img-qc.xhscdn.com/cover1.jpg" alt="决战美鞋之巅2.0（保暖靴子篇）" ` +
		`width="1080" height="1920" ` +
		`data-iframe-params="src=https%3A%2F%2Fwww.xiaohongshu.com%2Fexplore%2F6afabf31000000001803b7b0%3Fxsec_token%3DABWOyVMA&amp;w=1080&amp;h=1920" ` +
		`poster="https://sns-img-qc.xhscdn.com/cover1.jpg"></figure></article></body></html>`

	ex, err := extract.New(pageURL)
	assert.NoError(err)
	ex.Drop().Body = []byte(doc)

	ex.AddProcessors(
		LoadScripts(),
		ProcessDom("documentLoaded"),
		LoadSiteConfig,
		ExtractBody,
		ProcessDom("documentReady"),
		contents.Readability(),
		ProcessDom("documentDone"),
		cleanDOM,
	)
	ex.Run()
	assert.Empty(ex.Errors())

	html := string(ex.HTML)
	t.Logf("archived html: %s", html)

	assert.Contains(html, "还有我的联名靴哦哦哦！")
	assert.Contains(html, "作者：证人筠筠")
	assert.Contains(html, "data-iframe-params")
	assert.Contains(html, "6afabf31000000001803b7b0")
	assert.Contains(html, "poster=")
	assert.NotContains(html, "x-data-iframe-params")
}
