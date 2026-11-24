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
	if r.URL.Hostname() == "m.zcool.com.cn" {
		body = "<html><body><article>real work</article></body></html>"
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
