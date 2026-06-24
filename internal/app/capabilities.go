// SPDX-FileCopyrightText: © 2026 Claudio Yanes <me@claudio4.com>
//
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !linux

package app

// dropCapabilities is a no-op in any platform other than linux
// For its behaviour in linux checl capabilties_linux.go
func dropCapabilities() error {
	return nil
}
