// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package contentscripts

import (
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/bleach"
	"codeberg.org/readeck/readeck/pkg/extract"
	"codeberg.org/readeck/readeck/pkg/extract/contents"
	. "codeberg.org/readeck/readeck/pkg/extract/testing" //revive:disable:dot-imports
)

// cleanDOM stands in for the bookmark pipeline's CleanDomProcessor.
func cleanDOM(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom || m.Dom == nil {
		return next
	}
	bleach.DefaultPolicy.Clean(m.Dom)
	return next
}

// End to end on a real video note page: the rebuilt article must survive the
// readability pass (which normally drops media content) and the bleach pass
// (which strips data- attributes), so the archiver still finds the placeholder
// that turns the cover into a locally stored video.
func TestXiaohongshuVideoPipeline(t *testing.T) {
	assert := require.New(t)
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	pageURL := "https://www.xiaohongshu.com/explore/6af2c3ed000000002b0198ff"
	httpmock.RegisterResponder("GET", pageURL, NewHTMLResponder(200, "xiaohongshu_video.html"))

	ex, _ := extract.New(pageURL)
	// The page carries og:type "video.other", which is what the meta pass
	// turns into a media document type.
	ex.Drop().DocumentType = "video"
	ex.AddProcessors(
		LoadScripts(),
		ProcessDom("documentLoaded"),
		LoadSiteConfig,
		ExtractBody,
		ProcessDom("documentReady"),
		contents.StripHeadingAnchors,
		contents.ExtractInlineSVGs,
		contents.Readability(),
		ProcessDom("documentDone"),
		cleanDOM,
	)
	ex.Run()
	assert.Empty(ex.Errors())

	html := string(ex.HTML)
	t.Logf("archived html: %s", html)

	assert.Contains(html, "谭松韵红毯候场冻得裹紧羽绒服")
	assert.Contains(html, "作者：moneyXn")
	assert.Contains(html, "data-iframe-params")
	assert.NotContains(html, "x-data-iframe-params")
	assert.False(strings.Contains(html, "<video"), "players are rebuilt by the archiver")
}
