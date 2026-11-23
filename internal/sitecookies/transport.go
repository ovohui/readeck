// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package sitecookies

import (
	"log/slog"
	"net/http"
)

// RoundTripper wraps an http.RoundTripper and injects the cookies and headers
// configured for the destination domain. Headers explicitly set by a request
// are never overwritten.
type RoundTripper struct {
	http.RoundTripper
}

// headerSetter matches the interface used by content scripts to configure
// custom HTTP headers for a site.
type headerSetter interface {
	SetHeader(func(http.Header))
}

// NewRoundTripper wraps the given transport with cookie injection.
func NewRoundTripper(rt http.RoundTripper) *RoundTripper {
	return &RoundTripper{RoundTripper: rt}
}

// SetHeader forwards header configuration to the wrapped transport so site
// configuration headers keep working alongside cookie injection.
func (t *RoundTripper) SetHeader(fn func(http.Header)) {
	if hs, ok := t.RoundTripper.(headerSetter); ok {
		hs.SetHeader(fn)
	}
}

// RoundTrip implements http.RoundTripper.
func (t *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	cookie, headers, err := Matching(req.URL)
	if err != nil {
		slog.Warn("site cookies lookup", slog.Any("err", err))
	} else {
		// Clone to avoid mutating the caller's request.
		r := req.Clone(req.Context())
		if cookie != "" && r.Header.Get("Cookie") == "" {
			r.Header.Set("Cookie", cookie)
			slog.Debug("injecting site cookies", slog.String("domain", req.URL.Hostname()))
		}
		for k, v := range headers {
			if r.Header.Get(k) == "" {
				r.Header.Set(k, v)
			}
		}
		return t.RoundTripper.RoundTrip(r)
	}
	return t.RoundTripper.RoundTrip(req)
}
