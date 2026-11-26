// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/internal/ai"
)

func TestBuildAIOutlineKeepsImages(t *testing.T) {
	outline, images := buildAIOutline(`<section><p>第一段</p><figure><img src="/bm/x/a.gif" alt="动图" width="400" height="300"></figure><p>第二段</p></section>`)
	require.Len(t, images, 1)
	assert.Equal(t, "/bm/x/a.gif", images[0].URL)
	assert.Contains(t, outline, "第一段")
	assert.Contains(t, outline, "[IMG 0]")
	assert.Contains(t, outline, "第二段")
}

func TestRenderAIBlocksKeepsOmittedPictures(t *testing.T) {
	images := []ai.Image{
		{URL: "/bm/x/a.gif", Alt: "a"},
		{URL: "/bm/x/b.jpg", Alt: "b"},
	}
	body := renderAIBlocks([]ai.Block{
		{Type: "p", Text: "保留的正文"},
		{Type: "img", Index: 0},
	}, images)

	assert.Contains(t, body, "保留的正文")
	assert.Equal(t, 2, strings.Count(body, "<img"))
	assert.Contains(t, body, "/bm/x/b.jpg")
}
