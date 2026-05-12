// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package oidc

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
)

var (
	errNoProvider     = &oidcError{error: errors.New("provider not found"), status: http.StatusNotFound}
	errNoAuthCodes    = &oidcError{error: errors.New("no auth codes"), status: http.StatusForbidden}
	errState          = &oidcError{error: errors.New("state doesn't match"), status: http.StatusForbidden}
	errNoProvisioning = &oidcError{error: errors.New("provisioning disabled"), status: http.StatusForbidden}
)

type oidcError struct {
	error
	attrs  []slog.Attr
	status int
}

func newError(err error, options ...func(*oidcError)) *oidcError {
	e := &oidcError{}
	if oe, ok := err.(*oidcError); ok {
		*e = *oe
	} else {
		e = &oidcError{status: 500, error: err}
	}

	for _, fn := range options {
		fn(e)
	}

	return e
}

func withStatus(status int) func(e *oidcError) {
	return func(e *oidcError) {
		e.status = status
	}
}

func withAttrs(attrs ...slog.Attr) func(e *oidcError) {
	return func(e *oidcError) {
		e.attrs = append(e.attrs, attrs...)
	}
}

// Log produces a log when called from [server.Err].
func (e *oidcError) Log(log *slog.Logger) {
	level := slog.LevelWarn
	if e.status >= 500 {
		level = slog.LevelError
	}

	attrs := e.attrs
	attrs = append(attrs, slog.Any("err", e.error))

	log.LogAttrs(context.Background(), level, "oidc", attrs...)
}

// StatusCode returns the HTTP status used by [server.Err].
func (e *oidcError) StatusCode() int {
	if e.status == 0 {
		return 400
	}
	return e.status
}
