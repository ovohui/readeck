// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: MIT

package archiver

import (
	"strings"
	"testing"

	"github.com/go-shiori/dom"
	"golang.org/x/net/html"
)

// A page that already carries a placeholder for its player must not get a
// second one: injectForcedVideo runs for every video document, and two
// placeholders mean two downloads and two players.
func TestInjectForcedVideoSkipsExistingPlaceholder(t *testing.T) {
	noteURL := "https://www.xiaohongshu.com/explore/6af2c3ed000000002b0198ff?xsec_token=ABC"

	cases := []struct {
		name string
		body string
		want int // forced iframes expected
	}{
		{
			name: "image placeholder",
			body: `<article><p>text</p><figure><img src="cover.jpg" data-iframe-params="src=` +
				html.EscapeString(noteURL) + `&amp;w=1080&amp;h=1920"></figure></article>`,
			want: 0,
		},
		{
			name: "unsupported placeholder host",
			body: `<article><figure><img src="cover.jpg" data-iframe-params="src=https%3A%2F%2Fexample.com%2Fx"></figure></article>`,
			want: 1,
		},
		{
			name: "nothing at all",
			body: `<article><p>text</p></article>`,
			want: 1,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc, err := html.Parse(strings.NewReader("<html><body>" + c.body + "</body></html>"))
			if err != nil {
				t.Fatal(err)
			}

			arc := New(WithYtdlp(YtdlpConfig{Path: "/bin/true"}), WithForcedVideo(noteURL))
			arc.injectForcedVideo(doc)

			got := 0
			for _, node := range dom.GetElementsByTagName(doc, "iframe") {
				if dom.GetAttribute(node, "data-forced-video") == "1" {
					got++
				}
			}
			if got != c.want {
				t.Errorf("forced iframes = %d, want %d", got, c.want)
			}
		})
	}
}

func TestEmbeddedVideoPageURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"https://www.youtube.com/embed/dQw4w9WgXcQ", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", true},
		{"https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ?rel=0", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", true},
		{"https://youtu.be/dQw4w9WgXcQ", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", true},
		{"https://player.bilibili.com/player.html?bvid=BV1xx411c7mD&page=1", "https://www.bilibili.com/video/BV1xx411c7mD", true},
		{"https://www.bilibili.com/video/BV1xx411c7mD", "https://www.bilibili.com/video/BV1xx411c7mD", true},
		{"https://player.vimeo.com/video/123456789", "https://vimeo.com/123456789", true},
		{
			"https://www.xiaohongshu.com/explore/6af2c3ed000000002b0198ff?xsec_token=ABC&xsec_source=pc_feed",
			"https://www.xiaohongshu.com/explore/6af2c3ed000000002b0198ff?xsec_token=ABC&xsec_source=pc_feed",
			true,
		},
		{"https://example.com/player/123", "", false},
		{"javascript:void(0)", "", false},
		{"", "", false},
	}

	for _, c := range cases {
		got, ok := embeddedVideoPageURL(c.in)
		if ok != c.ok {
			t.Errorf("%q: ok = %v, want %v", c.in, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("%q: got %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsSupportedEmbedHost(t *testing.T) {
	yes := []string{"www.youtube.com", "youtube.com", "player.bilibili.com", "b23.tv", "player.vimeo.com",
		"www.xiaohongshu.com", "xhslink.com"}
	no := []string{"example.com", "notyoutube.com", "youtube.com.evil.test"}
	for _, h := range yes {
		if !isSupportedEmbedHost(h) {
			t.Errorf("%q should be supported", h)
		}
	}
	for _, h := range no {
		if isSupportedEmbedHost(h) {
			t.Errorf("%q should not be supported", h)
		}
	}
}
