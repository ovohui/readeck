// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package sitecookies

import "errors"

var (
	errInvalidDomain = errors.New("a valid domain is required")
	errEmptyCookie   = errors.New("cookies or headers are required")
)
