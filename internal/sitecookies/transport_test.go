// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package sitecookies

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A saved session must survive a request that already carries cookies from the
// client's jar. Skipping the injection in that case was what made a configured
// cookie look like it had never been saved.
func TestMergeCookieHeader(t *testing.T) {
	assert := assert.New(t)

	assert.Equal("z_c0=abc; d_c0=def", mergeCookieHeader("", "z_c0=abc; d_c0=def"))
	assert.Equal("acw_tc=1; z_c0=abc", mergeCookieHeader("acw_tc=1", "z_c0=abc"))

	// The saved value wins over the one the site handed out during this run.
	assert.Equal(
		"z_c0=fresh; d_c0=old",
		mergeCookieHeader("z_c0=stale; d_c0=old", "z_c0=fresh"),
	)

	// Spacing and a trailing semicolon are cleaned up, not duplicated.
	assert.Equal(
		"a=1; b=2",
		mergeCookieHeader("  a=1 ; ", " b=2; "),
	)

	assert.Equal("", mergeCookieHeader("", ""))
	assert.Equal("a=1", mergeCookieHeader("a=1", ""))
}
