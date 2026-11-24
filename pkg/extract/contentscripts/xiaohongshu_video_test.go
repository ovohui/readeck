// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package contentscripts

import (
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/extract"
	. "codeberg.org/readeck/readeck/pkg/extract/testing" //revive:disable:dot-imports
)

// A Xiaohongshu note is rebuilt from the state payload: the text becomes
// paragraphs, the author/date/location line comes first, and the pictures are
// appended in the author's order.
func TestXiaohongshuNoteLayout(t *testing.T) {
	assert := require.New(t)
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	pageURL := "https://www.xiaohongshu.com/explore/6af2c3ed000000002b0198ff"
	httpmock.RegisterResponder("GET", pageURL, NewHTMLResponder(200, "xiaohongshu_video.html"))

	ex, _ := extract.New(pageURL)
	ex.AddProcessors(
		LoadScripts(),
		ProcessDom("documentLoaded"),
	)
	ex.Run()
	assert.Empty(ex.Errors())

	html := string(ex.HTML)
	t.Logf("archived html: %s", html)

	assert.Contains(html, "<article")
	assert.Contains(html, "作者：moneyXn · 2026-11-10 15:12 · 山东")
	assert.Contains(html, "山东")
	assert.Contains(html, "谭松韵红毯候场冻得裹紧羽绒服")
	// The hashtags carry a "[话题]" marker in the payload.
	assert.Contains(html, "#谭松韵 ")
	assert.NotContains(html, "[话题]")
	// No player: it would be removed by the readability pass anyway.
	assert.NotContains(html, "<video")
	assert.Equal("谭松韵红毯候场冻得裹紧羽绒服，一上红毯立马切换状态#谭松韵 #谭松韵许兰香 #兰香如故 #w盛典", ex.Drop().Title)
}

// The cover of a video note stands in for the player: the archiver replaces
// this image with a locally stored <video> downloaded from the note URL.
func TestXiaohongshuVideoNoteMarker(t *testing.T) {
	assert := require.New(t)
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	pageURL := "https://www.xiaohongshu.com/explore/6af2c3ed000000002b0198ff"
	httpmock.RegisterResponder("GET", pageURL, NewHTMLResponder(200, "xiaohongshu_video.html"))

	ex, _ := extract.New(pageURL)
	ex.AddProcessors(
		LoadScripts(),
		ProcessDom("documentLoaded"),
	)
	ex.Run()
	assert.Empty(ex.Errors())

	html := string(ex.HTML)
	assert.Contains(html, "data-iframe-params=")
	assert.Contains(html, "src=https%3A%2F%2Fwww.xiaohongshu.com%2Fexplore%2F6af2c3ed000000002b0198ff")
	// The cover doubles as the poster of the archived video.
	assert.Contains(html, `poster="https://sns-webpic-qc.xhscdn.com/`)
	// ... and it is the only media element, never a duplicate.
	assert.Equal(1, countOccurrences(html, "<img"))
}

func countOccurrences(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}
