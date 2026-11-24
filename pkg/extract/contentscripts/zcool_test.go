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

// The ZCOOL mobile front end ships the whole work in its Next.js payload and
// paints an empty shell. The script rebuilds the work from that payload: meta
// line, description when present, then the gallery in the author's order.
func TestZcoolWorkLayout(t *testing.T) {
	assert := require.New(t)
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// The WAF shield rewrites the desktop host to the mobile one, so this is
	// the URL the extractor really fetches.
	pageURL := "https://m.zcool.com.cn/work/ZNzQ0Mzk0ODg=.html"
	httpmock.RegisterResponder("GET", pageURL, NewHTMLResponder(200, "zcool_work.html"))

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
	assert.Contains(html, "作者：1To3_Design_Lab")
	assert.Contains(html, "原创")
	assert.Contains(html, "平面 / 品牌")
	// createTime is rendered in UTC+8, whatever the server timezone is.
	assert.Contains(html, "2026-11-24")
	// Every gallery picture is kept, in order.
	assert.Equal(3, countOccurrences(html, "<img"))
	assert.Contains(html, "6b050039f0699cb1ag109j192.jpg")
	assert.Contains(html, "#品牌设计")
	assert.Equal("Panda Factory熊猫工厂品牌设计", ex.Drop().Title)
}
