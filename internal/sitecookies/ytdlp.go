// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package sitecookies

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// NetscapeFile writes every saved cookie as a Netscape cookies file and
// returns its path, so yt-dlp can use the same session the extractor uses.
//
// yt-dlp is a separate process: it never sees the transport wrapper that
// injects cookies into the HTTP client. A site that answers an anonymous
// request with a login wall (Zhihu) also refuses to hand over its clip, so
// passing the saved cookies along is what makes video archiving work there.
//
// The file is rewritten on each call on purpose: cookies expire and get
// rotated, and a stale copy is worse than none.
func NetscapeFile() (string, error) {
	items, err := All()
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "", fmt.Errorf("no site cookie saved")
	}

	var b strings.Builder
	b.WriteString("# Netscape HTTP Cookie File\n")
	b.WriteString("# Written by Readeck from Settings > Site cookies\n\n")

	expiry := time.Now().Add(365 * 24 * time.Hour).Unix()
	written := 0

	for _, sc := range items {
		domain := normalizeDomain(sc.Domain)
		if domain == "" {
			continue
		}
		for _, pair := range strings.Split(sc.Cookies, ";") {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			name, value, ok := strings.Cut(pair, "=")
			if !ok {
				continue
			}
			name = strings.TrimSpace(name)
			value = strings.TrimSpace(value)
			// A tab or a newline would break the column layout, and a cookie
			// with neither a name nor a value is unusable anyway.
			if name == "" || strings.ContainsAny(name+value, "\t\r\n") {
				continue
			}
			// A leading dot makes it a domain cookie, which is what a value
			// saved for "zhihu.com" should be: valid for every subdomain.
			fmt.Fprintf(&b, ".%s\tTRUE\t/\tFALSE\t%d\t%s\t%s\n", domain, expiry, name, value)
			written++
		}
	}

	if written == 0 {
		return "", fmt.Errorf("no usable cookie in the store")
	}

	path := filepath.Join(os.TempDir(), "readeck-ytdlp-cookies.txt")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil { //nolint:gosec // the file is 0600 and holds the user's own cookies
		return "", err
	}

	return path, nil
}
