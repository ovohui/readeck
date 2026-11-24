// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package contentscripts

import (
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/extract"
	"codeberg.org/readeck/readeck/pkg/extract/meta"
	. "codeberg.org/readeck/readeck/pkg/extract/testing" //revive:disable:dot-imports
)

// A Bilibili video page renders its player with JavaScript: the archived
// document carries a title and no paragraph at all. The script rebuilds an
// article around the page state - uploader, date, duration, views, description,
// cover - and leaves a poster placeholder that the archiver turns into a real
// <video> with yt-dlp.
func TestBilibiliVideoLayout(t *testing.T) {
	assert := require.New(t)
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	pageURL := "https://www.bilibili.com/video/BV1atse6EEfx"
	httpmock.RegisterResponder("GET", pageURL, NewHTMLResponder(200, "bilibili_video.html"))

	ex, _ := extract.New(pageURL)
	ex.AddProcessors(
		LoadScripts(),
		ProcessDom("documentLoaded"),
		meta.ExtractMeta,
		meta.SetDropProperties,
	)
	ex.Run()
	assert.Empty(ex.Errors())

	html := string(ex.HTML)
	t.Logf("archived html: %s", html)

	assert.Contains(html, `<article`)
	// The meta line is built from the page state, not from the DOM.
	assert.Contains(html, "UP：Nero_闪灵")
	assert.Contains(html, "2026-11-20")
	assert.Contains(html, "21:06")
	assert.Contains(html, "776.9 万播放")
	// The description keeps its line breaks as separate paragraphs.
	assert.Contains(html, "<p>拒绝意难平，CP通通HE</p>")
	// \u002F in the payload must be decoded, and the cover upgraded to https.
	assert.Contains(html, "https://i2.hdslb.com/bfs/archive/cover.jpg")
	// The placeholder carries the page URL for yt-dlp.
	assert.Contains(html, "data-iframe-params")
	assert.Contains(html, "src=https%3A%2F%2Fwww.bilibili.com%2Fvideo%2FBV1atse6EEfx")
	assert.Contains(html, "#AI整活全宇宙")
	assert.Equal("当我用AI续写爱情公寓4关谷悠悠婚礼【AI全民制作人】", ex.Drop().Title)
	assert.Equal([]string{"Nero_闪灵"}, ex.Drop().Authors)
	// og:type is rewritten to video.other; the meta processor turns that into
	// the document type, which is what makes the archiver download the clip.
	assert.Equal("video.other", ex.Drop().Meta.LookupGet("graph.type"))
}

// A column page (/read/cv...) is not a video: the script must leave it alone
// rather than replace its body with an empty article.
func TestBilibiliReadPageIsUntouched(t *testing.T) {
	assert := require.New(t)
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	pageURL := "https://www.bilibili.com/read/cv54052258"
	httpmock.RegisterResponder("GET", pageURL, NewHTMLResponder(200, "bilibili_video.html"))

	ex, _ := extract.New(pageURL)
	ex.AddProcessors(
		LoadScripts(),
		ProcessDom("documentLoaded"),
	)
	ex.Run()
	assert.Empty(ex.Errors())

	html := string(ex.HTML)
	assert.NotContains(html, "bili-meta")
	assert.Contains(html, "video-title")
}
