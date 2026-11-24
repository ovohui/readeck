// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package wafshield works around sites whose desktop pages sit behind a
// JavaScript challenge that a plain HTTP client can never pass.
//
// Aliyun WAF (阿里巴巴云盾) answers such a request with a 200 and a tiny
// "render this to get your cookie" page: a <textarea id="renderData"> holding
// an obfuscated argument, and a script that computes the acw_sc__v2 cookie and
// reloads. The obfuscation changes between deployments and the whole thing has
// to be executed, so solving it here is not an option.
//
// What *is* an option: those same sites usually serve a mobile front end that
// is not behind the challenge (ZCOOL does). When the challenge page is
// detected, the request is replayed once against the mobile mirror with a
// mobile User-Agent. Everything else goes straight through untouched.
package wafshield

import (
	"bufio"
	"io"
	"net/http"
	"strings"
)

// mirrorRule describes how to reach a site that will not serve its content to
// a plain HTTP client.
type mirrorRule struct {
	// target is the host that answers a mobile browser with a real page.
	target string
	// marks are site specific strings found in the useless answer instead of
	// the generic WAF markers. A site can refuse for reasons that have nothing
	// to do with Aliyun (36kr runs its own interstitial), but the remedy is the
	// same: the mobile front end behind the same path.
	marks []string
}

// mirrors maps a host that hides its content from a HTTP client to a host that
// serves the same pages to a mobile browser.
var mirrors = map[string]mirrorRule{
	"zcool.com.cn":     {target: "m.zcool.com.cn"},
	"www.zcool.com.cn": {target: "m.zcool.com.cn"},

	// 36kr answers a desktop request with its own "正在进行安全检测" interstitial
	// (no Aliyun marker). m.36kr.com serves the article as plain markup.
	"36kr.com":     {target: "m.36kr.com", marks: []string{"正在进行安全检测"}},
	"www.36kr.com": {target: "m.36kr.com", marks: []string{"正在进行安全检测"}},
}

// mobileUA identifies the retry as a phone browser. The mobile front end of a
// site only renders its JSON payload for a mobile User-Agent.
const mobileUA = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) " +
	"AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1"

// probeSize is how much of the response body is inspected for the challenge
// markers. Aliyun's interstitial is a couple of hundred bytes of markup before
// the payload, so 32 KiB is generous.
const probeSize = 32 << 10

// markers identify an Aliyun WAF JavaScript challenge page.
var markers = []string{
	"aliyun_waf",
	"aliyunwaf_",
	`id="renderData"`,
}

// Transport replays a challenged request against a mobile mirror.
type Transport struct {
	next http.RoundTripper
}

// New wraps next with the challenge retry.
func New(next http.RoundTripper) *Transport {
	return &Transport{next: next}
}

// SetHeader forwards header configuration to the wrapped transport, so site
// config headers keep working through the wrapper chain.
func (t *Transport) SetHeader(fn func(http.Header)) {
	if hs, ok := t.next.(interface {
		SetHeader(func(http.Header))
	}); ok {
		hs.SetHeader(fn)
	}
}

// RoundTrip implements http.RoundTripper.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	rsp, err := t.next.RoundTrip(req)
	if err != nil || rsp == nil {
		return rsp, err
	}
	if req.Method != http.MethodGet || !isHTML(rsp) {
		return rsp, nil
	}

	rule, ok := mirrorFor(req.URL.Hostname())
	if !ok {
		return rsp, nil
	}

	// Peek at the beginning of the body without consuming it: a real page must
	// be handed back byte for byte.
	br := bufio.NewReaderSize(rsp.Body, probeSize)
	head, _ := br.Peek(probeSize)
	if !isChallenge(head, rule.marks) {
		rsp.Body = readCloser{br, rsp.Body}
		return rsp, nil
	}
	mirror := rule.target

	// The challenge page carries nothing worth keeping.
	_ = rsp.Body.Close()

	clone := req.Clone(req.Context())
	clone.URL.Host = mirror
	clone.Host = mirror

	// The header set is rebuilt, not patched. The extractor sends a desktop
	// Chrome fingerprint (Sec-CH-UA, Sec-Fetch-*, desktop User-Agent); keeping
	// any of it next to an iPhone User-Agent is exactly what the challenge
	// looks for, and the replay would be challenged in turn. A mobile Safari
	// sends none of these.
	clone.Header = http.Header{
		"User-Agent":                {mobileUA},
		"Accept":                    {"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"},
		"Accept-Language":           {"zh-CN,zh;q=0.9,en;q=0.8"},
		"Referer":                   {"https://" + mirror + "/"},
		"Upgrade-Insecure-Requests": {"1"},
	}

	return t.next.RoundTrip(clone)
}

func isHTML(rsp *http.Response) bool {
	ct := strings.ToLower(rsp.Header.Get("Content-Type"))
	return ct == "" || strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml")
}

func isChallenge(body []byte, extra []string) bool {
	if len(body) == 0 {
		return false
	}
	s := string(body)
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	for _, m := range extra {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

func mirrorFor(host string) (mirrorRule, bool) {
	host = strings.ToLower(host)
	r, ok := mirrors[host]
	return r, ok
}

type readCloser struct {
	io.Reader
	io.Closer
}
