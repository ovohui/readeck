// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package sitecookies

import (
	"log/slog"
	"net/http"
	"strings"
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
		if cookie != "" {
			// The client's jar may already hold a cookie for this host: the
			// first request of an extraction often receives a device id, and
			// a page load can set half a dozen more. Skipping injection in
			// that case silently dropped the saved session and the site
			// answered with its login wall, which looked exactly like "the
			// cookie was never saved". Merge instead: the saved cookies win,
			// the jar keeps whatever else it collected.
			r.Header.Set("Cookie", mergeCookieHeader(r.Header.Get("Cookie"), cookie))
			slog.Debug("injecting site cookies", slog.String("domain", req.URL.Hostname()))
		}
		for k, v := range headers {
			// Referer is intentionally overridable: the archiver sets a
			// generic Referer for every resource, but a site cookie entry can
			// supply the exact Referer a CDN requires. Other headers follow
			// the usual "don't overwrite" rule.
			if r.Header.Get(k) == "" || strings.EqualFold(k, "Referer") {
				r.Header.Set(k, v)
			}
		}
		return t.RoundTripper.RoundTrip(r)
	}
	return t.RoundTripper.RoundTrip(req)
}

// mergeCookieHeader combines the cookies already attached to a request with the
// ones saved for the site. A name present in both keeps the saved value: that
// is the session the user pasted, the other one is whatever the site handed out
// during this extraction.
func mergeCookieHeader(existing, injected string) string {
	if existing == "" {
		return injected
	}
	if injected == "" {
		return existing
	}

	order := []string{}
	value := map[string]string{}

	for _, part := range []string{existing, injected} {
		for _, pair := range strings.Split(part, ";") {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			name, val, ok := strings.Cut(pair, "=")
			if !ok {
				continue
			}
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if _, seen := value[name]; !seen {
				order = append(order, name)
			}
			value[name] = strings.TrimSpace(val)
		}
	}

	out := make([]string, 0, len(order))
	for _, name := range order {
		out = append(out, name+"="+value[name])
	}
	return strings.Join(out, "; ")
}
