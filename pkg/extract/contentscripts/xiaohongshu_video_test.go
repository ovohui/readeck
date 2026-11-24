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

// A video note keeps its <video> element and the CDN URL after the content
// script runs. The SSR payload stores the stream under video.media.stream.
func TestXiaohongshuVideoNote(t *testing.T) {
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
	t.Logf("archived html: %d bytes", len(html))
	assert.Contains(html, "<video")
	assert.Contains(html, ".mp4")
}
