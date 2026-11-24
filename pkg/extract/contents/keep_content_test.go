// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package contents_test

import (
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/extract"
	"codeberg.org/readeck/readeck/pkg/extract/contents"
)

// A media document (a video note) normally has its whole content thrown away:
// the archiver rebuilds a player from the page URL instead. When a content
// script has already turned the page into a clean article, it sets the
// "readeck:keep-content" meta and the text must survive.
func TestReadabilityKeepsFlaggedMediaContent(t *testing.T) {
	page := func(flag string) string {
		return `<!DOCTYPE html><html><head><title>Note</title>` + flag +
			`</head><body><article>` +
			`<p>作者：某人 · 2026-11-16 16:30 · 福建</p>` +
			`<p>这是一条小红书笔记的正文，长度足够让 readability 保留它，而不是当成空页面丢掉。</p>` +
			`<figure><img src="cover.jpg" data-iframe-params="src=https%3A%2F%2Fwww.xiaohongshu.com%2Fexplore%2Fabc"></figure>` +
			`</article></body></html>`
	}

	tests := []struct {
		name  string
		flag  string
		keeps bool
	}{
		{name: "flagged media page keeps its content", flag: `<meta name="readeck:keep-content" content="1">`, keeps: true},
		{name: "unflagged media page is reset", flag: "", keeps: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()

			httpmock.RegisterResponder("GET", "https://www.xiaohongshu.com/explore/abc",
				httpmock.NewStringResponder(200, page(tt.flag)).
					HeaderAdd(http.Header{"Content-Type": {"text/html"}}))

			ex, err := extract.New("https://www.xiaohongshu.com/explore/abc")
			require.NoError(t, err)
			ex.Drop().DocumentType = "video"
			ex.AddProcessors(contents.Readability())
			ex.Run()
			require.Empty(t, ex.Errors())

			if tt.keeps {
				require.Contains(t, string(ex.HTML), "这是一条小红书笔记的正文")
				require.Contains(t, string(ex.HTML), "data-iframe-params")
			} else {
				require.Empty(t, ex.HTML)
			}
		})
	}
}
