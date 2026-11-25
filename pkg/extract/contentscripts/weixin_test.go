// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package contentscripts

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// WeChat keeps the article body in #js_content, shipped with
// `style="visibility: hidden; opacity: 0"` and revealed by its own script once
// the page is on screen. Readability skips hidden nodes, so a server side fetch
// used to keep the title, the account name and nothing else — 38 characters of
// header for a 17 000 character article.
//
// The site config rewrites the hidden marker before the DOM is parsed. The
// rule was written as `"visibility: hidden"`, quotes included, which only
// matches a style attribute holding that single declaration and not the
// `visibility: hidden; opacity: 0;` the page actually sends.
func TestWeixinConfigRevealsHiddenBody(t *testing.T) {
	u, err := url.Parse("https://mp.weixin.qq.com/s/FAeiC7XX6MBxjSFKBNXhaQ")
	require.NoError(t, err)

	cfg, err := NewConfigForURL(SiteConfigFiles, u)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Contains(t, cfg.ReplaceStrings, [2]string{"visibility: hidden", "visibility: visible"})

	// The declaration as the page really writes it.
	body := `<div id="js_content" style="visibility: hidden; opacity: 0; ">正文</div>`
	for _, r := range cfg.ReplaceStrings {
		body = strings.ReplaceAll(body, r[0], r[1])
	}
	assert.NotContains(t, body, "visibility: hidden",
		"the body must be visible again before readability runs")
	assert.Contains(t, body, "正文")
}
