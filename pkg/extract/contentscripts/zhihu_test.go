// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package contentscripts

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Zhihu answers a request that looks like a desktop browser with a 650 byte
// page holding nothing but the zh-zse-ck challenge, and the cookie that would
// satisfy it is computed by a 522 KB fingerprinting VM. The in-app webview
// user agent is the one client the site still serves without a challenge, so
// the site configs must keep sending it. Dropping it does not break any test
// that reads configuration — it only breaks archiving, silently, with a 403.
func TestZhihuConfigSendsAppUserAgent(t *testing.T) {
	for _, rawURL := range []string{
		"https://www.zhihu.com/question/19550225",
		"https://www.zhihu.com/question/19550225/answer/123",
		"https://zhuanlan.zhihu.com/p/2100121489433535118",
	} {
		u, err := url.Parse(rawURL)
		require.NoError(t, err)

		cfg, err := NewConfigForURL(SiteConfigFiles, u)
		require.NoError(t, err)
		require.NotNil(t, cfg)

		assert.Contains(t, cfg.HTTPHeaders["user-agent"], "ZhihuHybrid",
			"%s must be fetched with the Zhihu in-app user agent", u.Hostname())
		assert.Equal(t, "https://www.zhihu.com/", cfg.HTTPHeaders["referer"])
	}
}

// The column config used to carry a Firefox 39 user agent, hardcoded and
// stale. It was replaced, not merged, so this asserts the single value the
// request really gets.
func TestZhihuColumnConfigIsNotTheOldBrowserUA(t *testing.T) {
	u, err := url.Parse("https://zhuanlan.zhihu.com/p/1")
	require.NoError(t, err)

	cfg, err := NewConfigForURL(SiteConfigFiles, u)
	require.NoError(t, err)

	assert.NotContains(t, cfg.HTTPHeaders["user-agent"], "Firefox")
	assert.Contains(t, cfg.Files(), "zhuanlan.zhihu.com.json")
}
