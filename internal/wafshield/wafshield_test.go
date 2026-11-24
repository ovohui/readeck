// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package wafshield

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTransport answers the desktop host with an Aliyun challenge page and the
// mobile mirror with the real document.
type fakeTransport struct {
	calls []*http.Request
	body  string
}

func (f *fakeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	f.calls = append(f.calls, r)
	body := f.body
	switch r.URL.Hostname() {
	case "m.zcool.com.cn":
		body = "<html><body><article>real work</article></body></html>"
	case "m.36kr.com":
		body = "<html><body><article>real article</article></body></html>"
	}
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    r,
	}, nil
}

const challengePage = `<textarea id="renderData" style="display:none">{"l1":"var arg1='5c28492a';"}</textarea>` +
	`<meta name="aliyun_waf_aa" content="1adb65ab2eec0824754c04a19aa316e0"><script>setCookie("acw_sc__v2",e)</script>`

func TestChallengeIsReplayedOnTheMobileMirror(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	next := &fakeTransport{body: challengePage}
	rt := New(next)

	req, err := http.NewRequest("GET", "https://www.zcool.com.cn/work/ZNzQ0Mzk0ODg=.html", nil)
	require.NoError(err)

	rsp, err := rt.RoundTrip(req)
	require.NoError(err)
	defer rsp.Body.Close()

	body, err := io.ReadAll(rsp.Body)
	require.NoError(err)

	assert.Equal(2, len(next.calls), "the request is replayed once")
	assert.Equal("www.zcool.com.cn", next.calls[0].URL.Hostname())
	assert.Equal("m.zcool.com.cn", next.calls[1].URL.Hostname())
	assert.Contains(next.calls[1].Header.Get("User-Agent"), "iPhone")
	// The desktop fingerprint must be gone, not blended with a mobile UA.
	assert.Empty(next.calls[1].Header.Get("Sec-Ch-Ua-Platform"))
	assert.Empty(next.calls[1].Header.Get("Sec-CH-UA"))
	assert.Empty(next.calls[1].Header.Get("Sec-Fetch-Mode"))
	assert.Equal("https://m.zcool.com.cn/", next.calls[1].Header.Get("Referer"))
	assert.Contains(string(body), "real work")
}

func TestPlainPageIsHandedBackUntouched(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	page := "<html><body>" + strings.Repeat("<p>text</p>", 50) + "</body></html>"
	next := &fakeTransport{body: page}
	rt := New(next)

	req, err := http.NewRequest("GET", "https://www.zcool.com.cn/work/x.html", nil)
	require.NoError(err)

	rsp, err := rt.RoundTrip(req)
	require.NoError(err)
	defer rsp.Body.Close()

	body, err := io.ReadAll(rsp.Body)
	require.NoError(err)

	assert.Equal(1, len(next.calls))
	assert.Equal(page, string(body))
}

func TestHostsWithoutMirrorAreNotTouched(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	next := &fakeTransport{body: challengePage}
	rt := New(next)

	u, _ := url.Parse("https://example.com/page")
	req := &http.Request{Method: "GET", URL: u, Header: http.Header{}}

	rsp, err := rt.RoundTrip(req)
	require.NoError(err)
	defer rsp.Body.Close()

	assert.Equal(1, len(next.calls))
}

// 36kr does not run an Aliyun challenge: it answers a desktop client with its
// own "正在进行安全检测" interstitial. The mirror table carries the marker, so
// the retry happens for the same reason as on ZCOOL.
const krInterstitial = `<html><head><title></title></head><body>` +
	`<section class="page"><p><span>火山引擎</span></p>` +
	`<p><span>正在进行安全检测...</span></p>` +
	`<p>为保障您的访问安全，系统正在检测当前网络环境，该过程通常需要几秒钟，请耐心等待</p>` +
	`</section></body></html>`

func TestSiteSpecificInterstitialIsReplayed(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	next := &fakeTransport{body: krInterstitial}
	rt := New(next)

	req, err := http.NewRequest("GET", "https://36kr.com/p/4083016736478085", nil)
	require.NoError(err)

	rsp, err := rt.RoundTrip(req)
	require.NoError(err)
	defer rsp.Body.Close()

	body, err := io.ReadAll(rsp.Body)
	require.NoError(err)

	assert.Equal(2, len(next.calls))
	assert.Equal("m.36kr.com", next.calls[1].URL.Hostname())
	assert.Equal("/p/4083016736478085", next.calls[1].URL.Path)
	assert.Contains(string(body), "real article")
}

// A page that merely mentions the words must not be replayed: the mirror is
// only tried when the marker really is the interstitial.
func TestKrRealPageIsKept(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	page := "<html><body><article>" + strings.Repeat("<p>36kr story</p>", 40) +
		"</article></body></html>"
	next := &fakeTransport{body: page}
	rt := New(next)

	req, err := http.NewRequest("GET", "https://36kr.com/p/1", nil)
	require.NoError(err)

	rsp, err := rt.RoundTrip(req)
	require.NoError(err)
	defer rsp.Body.Close()

	body, err := io.ReadAll(rsp.Body)
	require.NoError(err)

	assert.Equal(1, len(next.calls))
	assert.Equal(page, string(body))
}
