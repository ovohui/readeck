// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: MIT

package archiver

import "testing"

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
