// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package components

import (
	"context"
	"time"

	"codeberg.org/readeck/readeck/pkg/strftime"
)

// These are the date formats used on Readeck's frontend.
// In English, they translate like this:
// localeDate         : 28 Aug 2026
// localeDateTime     : 28 Aug 2026, 15:37
// localeDateTimeFull : 28 Aug 2026, 15:37:23 UTC
//
// Translations should match their respective formats and can't start with an article.
var (
	localeDate         = strftime.Pgettext("datetime", "%e %b %Y")
	localeDateTime     = strftime.Pgettext("datetime", "%e %b %Y, %H:%M")
	localeDateTimeFull = strftime.Pgettext("datetime", "%e %b %Y, %H:%M:%S %Z")
)

// Strftime calls [strftime.Formatter.Strftime] with the context's locale.
func Strftime(ctx context.Context, t time.Time, f string) string {
	return strftime.New(L(ctx)).Strftime(f, t)
}

// formatTime calls [strftime.Formatter.Format] with the context's locale.
func formatTime(ctx context.Context, t time.Time, tr strftime.Translatable) string {
	return strftime.New(L(ctx)).Format(tr, t)
}

// Date returns the date only, formatted and localized time.
func Date(ctx context.Context, t time.Time) string {
	return formatTime(ctx, t, localeDate)
}

// DateTime returns the date and time, formatted and localized time.
func DateTime(ctx context.Context, t time.Time) string {
	return formatTime(ctx, t, localeDateTime)
}

// DateTimeFull returns the full date and time, formatted and localized time.
func DateTimeFull(ctx context.Context, t time.Time) string {
	return formatTime(ctx, t, localeDateTimeFull)
}
